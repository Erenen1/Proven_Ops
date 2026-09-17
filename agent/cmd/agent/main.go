package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"opspilot/agent/internal/client"
	"opspilot/agent/internal/config"
	"opspilot/agent/internal/discovery"
	"opspilot/agent/internal/executor"
	"opspilot/agent/internal/tools"
)

func main() {
	log.Println("[ProvenOps Agent] Starting Server Agent daemon...")

	cfg := config.Load()
	collector := discovery.NewCollector()
	registry := tools.NewRegistry()
	guard := executor.NewCommandGuard()
	runner := executor.NewRunner(registry, guard)

	agentClient := client.NewAgentClient(cfg, collector, runner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := agentClient.Start(ctx); err != nil {
					log.Printf("[ProvenOps Agent] Client error: %v. Retrying in 2 seconds...", err)
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Second):
					}
				}
			}
		}
	}()

	<-quit
	log.Println("[ProvenOps Agent] Shutting down agent daemon...")
	cancel()
	log.Println("[ProvenOps Agent] Daemon stopped cleanly.")
}
