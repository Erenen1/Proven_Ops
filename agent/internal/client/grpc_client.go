package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"opspilot/agent/internal/config"
	"opspilot/agent/internal/discovery"
	"opspilot/agent/internal/executor"
	opspilotv1 "opspilot/proto/v1"
)

type AgentClient struct {
	cfg       *config.Config
	collector *discovery.Collector
	runner    *executor.Runner
	conn      *grpc.ClientConn
	client    opspilotv1.AgentServiceClient
	agentID   string
}

func NewAgentClient(cfg *config.Config, collector *discovery.Collector, runner *executor.Runner) *AgentClient {
	return &AgentClient{
		cfg:       cfg,
		collector: collector,
		runner:    runner,
	}
}

func (c *AgentClient) Start(ctx context.Context) error {
	log.Printf("[ProvenOps Agent] Connecting to Control Plane at %s...", c.cfg.ControlPlaneAddr)

	var dialCreds credentials.TransportCredentials
	if c.cfg.TLSEnabled {
		if _, err := os.Stat(c.cfg.TLSClientCert); os.IsNotExist(err) {
			log.Println("[ProvenOps Agent] Client certificate not found. Initiating automated PKI bootstrap enrollment...")
			if err := c.enrollCertificates(ctx); err != nil {
				return fmt.Errorf("automated PKI bootstrap enrollment failed: %w", err)
			}
		}

		log.Println("[ProvenOps Agent] mTLS enabled: Loading certificates...")
		clientCert, err := tls.LoadX509KeyPair(c.cfg.TLSClientCert, c.cfg.TLSClientKey)
		if err != nil {
			return fmt.Errorf("failed to load client certificate/key: %w", err)
		}
		caPEM, err := os.ReadFile(c.cfg.TLSCACert)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caPEM) {
			return fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      certPool,
			ServerName:   c.cfg.TLSServerName,
			MinVersion:   tls.VersionTLS13,
		}
		dialCreds = credentials.NewTLS(tlsConfig)
		log.Printf("[ProvenOps Agent] mTLS configured with server name %s", c.cfg.TLSServerName)
	} else {
		dialCreds = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(
		c.cfg.ControlPlaneAddr,
		grpc.WithTransportCredentials(dialCreds),
	)
	if err != nil {
		return fmt.Errorf("failed to dial control plane: %w", err)
	}
	c.conn = conn
	c.client = opspilotv1.NewAgentServiceClient(conn)

	// 1. Discovery & Registration
	hostInfo := c.collector.DiscoverHost()
	log.Printf("[ProvenOps Agent] Discovered host: %s (%s %s %s), capabilities: %v",
		hostInfo.Hostname, hostInfo.Distribution, hostInfo.Version, hostInfo.Architecture, hostInfo.Capabilities)

	regReq := &opspilotv1.RegisterRequest{
		Hostname:       hostInfo.Hostname,
		Os:             hostInfo.OS,
		Distribution:   hostInfo.Distribution,
		Version:        hostInfo.Version,
		Architecture:   hostInfo.Architecture,
		Capabilities:   hostInfo.Capabilities,
		BootstrapToken: c.cfg.BootstrapToken,
		IpAddress:      hostInfo.IPAddress,
		Environment:    c.cfg.Environment,
	}

	regResp, err := c.client.Register(ctx, regReq)
	if err != nil {
		return fmt.Errorf("agent registration failed: %w", err)
	}
	if !regResp.Approved {
		return fmt.Errorf("agent registration rejected: %s", regResp.Message)
	}

	c.agentID = regResp.AgentId
	log.Printf("[ProvenOps Agent] Successfully registered with ID: %s", c.agentID)

	// 2. Start periodic heartbeat
	go c.runHeartbeatLoop(ctx)

	// 3. Connect streaming channel
	return c.runStreamLoop(ctx)
}

func (c *AgentClient) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.cfg.HeartbeatIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m := c.collector.CollectMetrics(0)
			req := &opspilotv1.HeartbeatRequest{
				AgentId:           c.agentID,
				CpuUsagePercent:   m.CPUUsagePercent,
				MemoryUsageBytes:  m.MemoryUsageBytes,
				MemoryTotalBytes:  m.MemoryTotalBytes,
				DiskUsagePercent:  m.DiskUsagePercent,
				LoadAvg_1M:        m.LoadAvg1m,
				ActiveTasks:       m.ActiveTasks,
				TimestampUnix:     m.TimestampUnix,
			}
			_, _ = c.client.SendHeartbeat(ctx, req)
		}
	}
}

func (c *AgentClient) runStreamLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		stream, err := c.client.ConnectStream(ctx)
		if err != nil {
			log.Printf("[ProvenOps Agent] Stream connection failed: %v, retrying in 3s...", err)
			time.Sleep(3 * time.Second)
			continue
		}

		// Send handshake
		err = stream.Send(&opspilotv1.AgentStreamMessage{
			MessageId: uuid.New().String(),
			AgentId:   c.agentID,
			Payload: &opspilotv1.AgentStreamMessage_Handshake{
				Handshake: &opspilotv1.HandshakeMessage{
					AgentId:  c.agentID,
					Hostname: c.cfg.Hostname,
					Version:  "1.0.0",
				},
			},
		})
		if err != nil {
			log.Printf("[ProvenOps Agent] Handshake failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Println("[ProvenOps Agent] Control stream established and ready for task dispatch.")

		for {
			in, err := stream.Recv()
			if err != nil {
				log.Printf("[ProvenOps Agent] Stream disconnected: %v", err)
				break
			}

			switch cmd := in.Payload.(type) {
			case *opspilotv1.ControlStreamMessage_ExecuteStep:
				go c.handleExecuteStep(ctx, stream, cmd.ExecuteStep)
			case *opspilotv1.ControlStreamMessage_Ping:
				_ = stream.Send(&opspilotv1.AgentStreamMessage{
					MessageId: uuid.New().String(),
					AgentId:   c.agentID,
					Payload: &opspilotv1.AgentStreamMessage_Pong{
						Pong: &opspilotv1.PongMessage{TimestampUnix: time.Now().Unix()},
					},
				})
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func (c *AgentClient) handleExecuteStep(
	ctx context.Context,
	stream opspilotv1.AgentService_ConnectStreamClient,
	cmd *opspilotv1.ExecuteStepCommand,
) {
	log.Printf("[ProvenOps Agent] Executing step %s: action=%s", cmd.StepId, cmd.Action)

	executionID := fmt.Sprintf("%s/%s", cmd.TaskId, cmd.StepId)

	onChunk := func(streamType string, chunk string) {
		if chunk == "" {
			return
		}
		_ = stream.Send(&opspilotv1.AgentStreamMessage{
			MessageId: uuid.New().String(),
			AgentId:   c.agentID,
			Payload: &opspilotv1.AgentStreamMessage_OutputChunk{
				OutputChunk: &opspilotv1.StepOutputChunk{
					TaskId:        cmd.TaskId,
					StepId:        cmd.StepId,
					StreamType:    streamType,
					Data:          chunk,
					TimestampUnix: time.Now().Unix(),
				},
			},
		})
	}

	res, err := c.runner.ExecuteWithStream(ctx, executionID, cmd.Action, cmd.ArgumentsJson, int(cmd.TimeoutSeconds), onChunk)
	if err != nil {
		log.Printf("[ProvenOps Agent] Execution error for step %s: %v", cmd.StepId, err)
		res = &executor.StepExecutionResult{
			Action:     cmd.Action,
			ExitCode:   1,
			Stderr:     err.Error(),
			DurationMS: 0,
			Success:    false,
		}
	}

	dataJSON, _ := json.Marshal(res.Data)

	// Send final step result
	_ = stream.Send(&opspilotv1.AgentStreamMessage{
		MessageId: uuid.New().String(),
		AgentId:   c.agentID,
		Payload: &opspilotv1.AgentStreamMessage_StepResult{
			StepResult: &opspilotv1.StepResult{
				TaskId:           cmd.TaskId,
				StepId:           cmd.StepId,
				Action:           cmd.Action,
				ExitCode:         int32(res.ExitCode),
				Stdout:           res.Stdout,
				Stderr:           res.Stderr,
				DurationMs:       res.DurationMS,
				Success:          res.Success,
				VerificationJson: string(dataJSON),
			},
		},
	})
	log.Printf("[ProvenOps Agent] Finished step %s: exit_code=%d, success=%v", cmd.StepId, res.ExitCode, res.Success)
}

func (c *AgentClient) enrollCertificates(ctx context.Context) error {
	host := c.cfg.ControlPlaneAddr
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	enrollURL := fmt.Sprintf("http://%s:8080/api/v1/pki/enroll", host)

	payload := map[string]string{
		"bootstrap_token": c.cfg.BootstrapToken,
		"hostname":        c.cfg.Hostname,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", enrollURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("enrollment HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("enrollment rejected by control plane: HTTP %d", resp.StatusCode)
	}

	var res struct {
		Approved      bool   `json:"approved"`
		AgentID       string `json:"agent_id"`
		ClientCertPEM string `json:"client_cert_pem"`
		ClientKeyPEM  string `json:"client_key_pem"`
		CACertPEM     string `json:"ca_cert_pem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return fmt.Errorf("failed to decode enrollment response: %w", err)
	}
	if !res.Approved {
		return fmt.Errorf("enrollment was not approved")
	}

	if err := os.MkdirAll(filepath.Dir(c.cfg.TLSClientCert), 0750); err != nil {
		return err
	}
	if err := os.WriteFile(c.cfg.TLSClientCert, []byte(res.ClientCertPEM), 0644); err != nil {
		return fmt.Errorf("failed to write client cert: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.cfg.TLSClientKey), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(c.cfg.TLSClientKey, []byte(res.ClientKeyPEM), 0600); err != nil {
		return fmt.Errorf("failed to write client key: %w", err)
	}
	if c.cfg.TLSCACert != "" {
		_ = os.MkdirAll(filepath.Dir(c.cfg.TLSCACert), 0750)
		_ = os.WriteFile(c.cfg.TLSCACert, []byte(res.CACertPEM), 0644)
	}

	log.Printf("[ProvenOps Agent] Automated PKI enrollment complete! Cert saved to %s", c.cfg.TLSClientCert)
	return nil
}
