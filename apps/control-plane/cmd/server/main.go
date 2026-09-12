package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
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
	log.Printf("[OpsPilot Control Plane] Initializing on HTTP :%d, gRPC :%d (Environment: %s)...", cfg.HTTPPort, cfg.GRPCPort, cfg.Environment)

	// Initialize Storage: Fail-fast in production, fallback in dev
	var store database.Store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.ConnectPool(ctx, cfg.DatabaseURL)
	if err != nil || pool.Ping(ctx) != nil {
		if cfg.Environment == "production" {
			log.Fatalf("[OpsPilot Control Plane] FATAL: PostgreSQL unavailable in production mode: %v", err)
		}
		log.Printf("[OpsPilot Control Plane] WARNING: Running with in-memory fallback store (dev only): %v", err)
		store = database.NewMemoryStore()
	} else {
		log.Println("[OpsPilot Control Plane] Connected to PostgreSQL successfully. Using persistent PostgresStore.")
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

	var grpcOpts []grpc.ServerOption
	if cfg.TLSEnabled {
		log.Println("[OpsPilot Control Plane] mTLS enabled: Loading CA and server certificates...")
		caPEM, err := os.ReadFile(cfg.TLSCACert)
		if err != nil {
			log.Fatalf("Failed to read TLS CA cert: %v", err)
		}
		serverCertPEM, err := os.ReadFile(cfg.TLSServerCert)
		if err != nil {
			log.Fatalf("Failed to read TLS server cert: %v", err)
		}
		serverKeyPEM, err := os.ReadFile(cfg.TLSServerKey)
		if err != nil {
			log.Fatalf("Failed to read TLS server key: %v", err)
		}

		tlsConfig, err := pki.ServerTLSConfig(caPEM, serverCertPEM, serverKeyPEM)
		if err != nil {
			log.Fatalf("Failed to configure mTLS: %v", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		log.Println("[OpsPilot Control Plane] mTLS active. Client certificate verification required.")
	} else {
		log.Println("[OpsPilot Control Plane] NOTICE: Running with plaintext gRPC transport.")
	}

	grpcSrv := grpc.NewServer(grpcOpts...)
	opspilotv1.RegisterAgentServiceServer(grpcSrv, grpcServerImpl)

	go func() {
		log.Printf("[OpsPilot Control Plane] gRPC server listening on :%d", cfg.GRPCPort)
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

	// Initialize HTTP REST / SSE Router
	router := api.NewRouter(store, authService, taskOrchestrator, hub, machine)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("[OpsPilot Control Plane] HTTP/REST/SSE server listening on :%d", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[OpsPilot Control Plane] Shutting down gracefully...")
	grpcSrv.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)

	log.Println("[OpsPilot Control Plane] Stopped cleanly.")
}
