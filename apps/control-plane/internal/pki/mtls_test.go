package pki_test

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"opspilot/control-plane/internal/pki"
	opspilotv1 "opspilot/proto/v1"
)

type dummyAgentServiceServer struct {
	opspilotv1.UnimplementedAgentServiceServer
}

func (d *dummyAgentServiceServer) Register(ctx context.Context, req *opspilotv1.RegisterRequest) (*opspilotv1.RegisterResponse, error) {
	return &opspilotv1.RegisterResponse{
		AgentId:  "agent-test-mtls",
		Approved: true,
		Message:  "mTLS Handshake and RPC Successful",
	}, nil
}

func TestMutualTLS_FullLifecycle(t *testing.T) {
	// 1. Generate Root CA
	ca, err := pki.GenerateCA("OpsPilot Test Root CA")
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// 2. Generate Server Certificate
	serverKP, err := pki.GenerateServerCert(ca, []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatalf("Failed to generate server certificate: %v", err)
	}

	// 3. Generate Valid Agent Client Certificate
	clientKP, err := pki.GenerateClientCert(ca, "agent-node-01", "ubuntu-node")
	if err != nil {
		t.Fatalf("Failed to generate client certificate: %v", err)
	}

	// 4. Generate Rogue / Untrusted CA and Client Certificate (for rejection test)
	rogueCA, err := pki.GenerateCA("Rogue Untrusted CA")
	if err != nil {
		t.Fatalf("Failed to generate rogue CA: %v", err)
	}
	rogueClientKP, err := pki.GenerateClientCert(rogueCA, "rogue-agent-01", "attacker")
	if err != nil {
		t.Fatalf("Failed to generate rogue client certificate: %v", err)
	}

	// 5. Start gRPC Server enforcing mTLS
	serverTLS, err := pki.ServerTLSConfig(ca.CertPEM, serverKP.CertPEM, serverKP.KeyPEM)
	if err != nil {
		t.Fatalf("Failed to build server TLS config: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind test port: %v", err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLS)))
	opspilotv1.RegisterAgentServiceServer(grpcServer, &dummyAgentServiceServer{})

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	serverAddr := listener.Addr().String()

	// -------------------------------------------------------------
	// SUBTEST 1: Valid client certificate succeeds
	// -------------------------------------------------------------
	t.Run("ValidClientCertificate_Success", func(t *testing.T) {
		validClientTLS, err := pki.ClientTLSConfig(ca.CertPEM, clientKP.CertPEM, clientKP.KeyPEM, "localhost")
		if err != nil {
			t.Fatalf("Failed to configure valid client TLS: %v", err)
		}

		conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(validClientTLS)))
		if err != nil {
			t.Fatalf("Failed to dial server: %v", err)
		}
		defer conn.Close()

		client := opspilotv1.NewAgentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := client.Register(ctx, &opspilotv1.RegisterRequest{
			Hostname:       "ubuntu-node",
			BootstrapToken: "test-token",
		})
		if err != nil {
			t.Fatalf("mTLS RPC unexpectedly failed with valid certificate: %v", err)
		}

		if !resp.Approved {
			t.Errorf("Expected approved registration, got false")
		}
		t.Logf("PASS: Valid mTLS handshake succeeded: %s", resp.Message)
	})

	// -------------------------------------------------------------
	// SUBTEST 2: Plaintext / Insecure client is rejected
	// -------------------------------------------------------------
	t.Run("PlaintextClient_Rejected", func(t *testing.T) {
		// Attempting TLS without providing client certificate should fail
		emptyTLS := &tls.Config{
			InsecureSkipVerify: true, // will still fail because server requires client cert
		}
		conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(emptyTLS)))
		if err != nil {
			t.Fatalf("Dial failed: %v", err)
		}
		defer conn.Close()

		client := opspilotv1.NewAgentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = client.Register(ctx, &opspilotv1.RegisterRequest{Hostname: "no-cert-agent"})
		if err == nil {
			t.Fatalf("Expected failure when connecting without client certificate, but succeeded!")
		}
		t.Logf("PASS: Client without certificate successfully rejected: %v", err)
	})

	// -------------------------------------------------------------
	// SUBTEST 3: Untrusted / Rogue CA client is rejected
	// -------------------------------------------------------------
	t.Run("RogueCertificate_Rejected", func(t *testing.T) {
		// Client has a certificate, but it is signed by a rogue untrusted CA
		rogueClientTLS, err := pki.ClientTLSConfig(ca.CertPEM, rogueClientKP.CertPEM, rogueClientKP.KeyPEM, "localhost")
		if err != nil {
			t.Fatalf("Failed to build rogue TLS config: %v", err)
		}

		conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(rogueClientTLS)))
		if err != nil {
			t.Fatalf("Dial failed: %v", err)
		}
		defer conn.Close()

		client := opspilotv1.NewAgentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = client.Register(ctx, &opspilotv1.RegisterRequest{Hostname: "rogue-agent"})
		if err == nil {
			t.Fatalf("Expected failure when connecting with untrusted rogue certificate, but succeeded!")
		}
		t.Logf("PASS: Rogue client certificate successfully rejected by mTLS server: %v", err)
	})
}
