package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"opspilot/control-plane/internal/api"
	"opspilot/control-plane/internal/auth"
	"opspilot/control-plane/internal/config"
	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/grpcserver"
	"opspilot/control-plane/internal/orchestrator"
	"opspilot/control-plane/internal/pki"
	"opspilot/control-plane/internal/policy"
	"opspilot/control-plane/internal/statemachine"
	"opspilot/control-plane/internal/verification"
	opspilotv1 "opspilot/proto/v1"
)

func main() {
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[ProvenOps Control Plane] FATAL Configuration Error: %v", err)
	}
	log.Printf("[ProvenOps Control Plane] Initializing on HTTP :%d, gRPC :%d (Environment: %s)...", cfg.HTTPPort, cfg.GRPCPort, cfg.Environment)

	// Initialize Storage: Fail-fast in production, fallback in dev
	var store database.Store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.ConnectPool(ctx, cfg.DatabaseURL)
	if err != nil || pool.Ping(ctx) != nil {
		if cfg.Environment == "production" {
			log.Fatalf("[ProvenOps Control Plane] FATAL: PostgreSQL unavailable in production mode: %v", err)
		}
		log.Printf("[ProvenOps Control Plane] WARNING: Running with in-memory fallback store (dev only): %v", err)
		store = database.NewMemoryStore()
	} else {
		log.Println("[ProvenOps Control Plane] Connected to PostgreSQL successfully. Using persistent PostgresStore.")
		store = database.NewPostgresStore(pool)
	}

	// Initialize Domain Services
	authService := auth.NewService(cfg.JWTSecret, 24)
	machine := statemachine.NewMachine()
	policyEngine := policy.NewEngine()
	verifier := verification.NewEngine()
	hub := events.NewHub()

	// Initialize gRPC Agent Server
	grpcServerImpl := grpcserver.NewServer(store, hub, cfg.BootstrapToken)
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to bind gRPC port: %v", err)
	}

	var ca *pki.CertificateAuthority
	var grpcOpts []grpc.ServerOption
	if cfg.TLSEnabled {
		log.Println("[ProvenOps Control Plane] mTLS enabled: Loading or bootstrapping CA and server certificates...")
		if cfg.TLSCACert == "" {
			if _, err := os.Stat("/etc/provenops/certs"); err == nil {
				cfg.TLSCACert = "/etc/provenops/certs/ca.crt"
			} else {
				cfg.TLSCACert = "/etc/opspilot/certs/ca.crt"
			}
		}
		if cfg.TLSServerCert == "" {
			if _, err := os.Stat("/etc/provenops/certs"); err == nil {
				cfg.TLSServerCert = "/etc/provenops/certs/server.crt"
			} else {
				cfg.TLSServerCert = "/etc/opspilot/certs/server.crt"
			}
		}
		if cfg.TLSServerKey == "" {
			if _, err := os.Stat("/etc/provenops/certs"); err == nil {
				cfg.TLSServerKey = "/etc/provenops/certs/server.key"
			} else {
				cfg.TLSServerKey = "/etc/opspilot/certs/server.key"
			}
		}
		caKeyPath := filepath.Join(filepath.Dir(cfg.TLSCACert), "ca.key")

		caPEM, errCA := os.ReadFile(cfg.TLSCACert)
		serverCertPEM, errCert := os.ReadFile(cfg.TLSServerCert)
		serverKeyPEM, errKey := os.ReadFile(cfg.TLSServerKey)

		if errCA != nil || errCert != nil || errKey != nil {
			log.Println("[ProvenOps Control Plane] Cert files missing on disk. Generating internal Root CA and Server certificates...")
			genCA, err := pki.GenerateCA("ProvenOps Internal Root CA")
			if err != nil {
				log.Fatalf("Failed to generate CA: %v", err)
			}
			serverKP, err := pki.GenerateServerCert(genCA, []string{"localhost", "control-plane", "127.0.0.1", "provenops-control-plane", "opspilot-control-plane"})
			if err != nil {
				log.Fatalf("Failed to generate server certificate: %v", err)
			}
			_ = pki.SavePEM(cfg.TLSCACert, genCA.CertPEM, 0644)
			_ = pki.SavePEM(caKeyPath, genCA.KeyPEM, 0600)
			_ = pki.SavePEM(cfg.TLSServerCert, serverKP.CertPEM, 0644)
			_ = pki.SavePEM(cfg.TLSServerKey, serverKP.KeyPEM, 0600)
			caPEM = genCA.CertPEM
			serverCertPEM = serverKP.CertPEM
			serverKeyPEM = serverKP.KeyPEM
			ca = genCA
		} else {
			parsedCA, _ := pki.LoadCA(cfg.TLSCACert, caKeyPath)
			ca = parsedCA
		}

		grpcServerImpl.SetCA(ca)

		revocationChecker := &pki.StoreRevocationChecker{
			IsRevokedFunc: func(serial string) (bool, error) {
				return store.IsCertificateRevoked(context.Background(), serial)
			},
		}

		tlsConfig, err := pki.ServerTLSConfig(caPEM, serverCertPEM, serverKeyPEM, revocationChecker)
		if err != nil {
			log.Fatalf("Failed to configure mTLS: %v", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		log.Println("[ProvenOps Control Plane] mTLS active. Client certificate verification required.")
	} else {
		log.Println("[ProvenOps Control Plane] NOTICE: Running with plaintext gRPC transport.")
	}

	// Seed pre-provisioned single-use bootstrap tokens into store
	seedTokens := []string{
		cfg.BootstrapToken,
		"token-node-01", "token-node-02", "token-node-03", "token-node-04", "token-node-05",
	}
	for _, tok := range seedTokens {
		if tok != "" {
			_ = store.SaveBootstrapToken(context.Background(), tok, "Pre-provisioned enrollment token", time.Now().Add(365*24*time.Hour))
		}
	}

	grpcSrv := grpc.NewServer(grpcOpts...)
	opspilotv1.RegisterAgentServiceServer(grpcSrv, grpcServerImpl)

	go func() {
		log.Printf("[ProvenOps Control Plane] gRPC server listening on :%d", cfg.GRPCPort)
		if err := grpcSrv.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// Initialize Task Orchestrator
	taskOrchestrator := orchestrator.NewOrchestrator(
		store,
		machine,
		policyEngine,
		verifier,
		hub,
		grpcServerImpl,
		cfg.AIServiceURL,
	)

	// Initialize HTTP REST / SSE Router with PKI CA and bootstrap token
	router := api.NewRouterWithPKI(store, authService, taskOrchestrator, hub, machine, ca, cfg.BootstrapToken)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("[ProvenOps Control Plane] HTTP/REST/SSE server listening on :%d", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Recover in-flight tasks after crash/restart
	go func() {
		time.Sleep(3 * time.Second)
		if err := taskOrchestrator.RecoverInFlightTasks(context.Background()); err != nil {
			log.Printf("[ProvenOps Control Plane] In-flight recovery notice: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[ProvenOps Control Plane] Shutting down gracefully...")
	grpcSrv.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)

	log.Println("[ProvenOps Control Plane] Stopped cleanly.")
}
