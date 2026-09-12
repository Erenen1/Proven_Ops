package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"opspilot/agent/internal/client"
	"opspilot/agent/internal/config"
	"opspilot/agent/internal/discovery"
	"opspilot/agent/internal/executor"
	"opspilot/agent/internal/tools"
)

func main() {
	log.Println("[OpsPilot Agent] Starting Server Agent daemon...")

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
		if err := agentClient.Start(ctx); err != nil {
			log.Printf("[OpsPilot Agent] Client error: %v", err)
		}
	}()

	<-quit
	log.Println("[OpsPilot Agent] Shutting down agent daemon...")
	cancel()
	log.Println("[OpsPilot Agent] Daemon stopped cleanly.")
}
