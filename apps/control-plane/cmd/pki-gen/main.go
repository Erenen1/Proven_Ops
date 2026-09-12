package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"opspilot/control-plane/internal/pki"
)

func main() {
	outDir := flag.String("out", "infra/certs", "Output directory for certificates")
	serverHost := flag.String("host", "localhost", "Server hostname / IP for SAN")
	agentID := flag.String("agent-id", "agent-ubuntu-01", "Agent identifier for client cert")
	flag.Parse()

	log.Printf("[PKI] Generating OpsPilot PKI suite into %s...", *outDir)

	// 1. Root CA
	ca, err := pki.GenerateCA("OpsPilot Production Root CA")
	if err != nil {
		log.Fatalf("Failed to generate CA: %v", err)
	}
	_ = pki.SavePEM(filepath.Join(*outDir, "ca.crt"), ca.CertPEM, 0644)
	_ = pki.SavePEM(filepath.Join(*outDir, "ca.key"), ca.KeyPEM, 0600)
	log.Println("[PKI] Generated Root CA: ca.crt, ca.key")

	// 2. Server Certificate
	serverKP, err := pki.GenerateServerCert(ca, []string{*serverHost, "127.0.0.1", "control-plane"})
	if err != nil {
		log.Fatalf("Failed to generate server certificate: %v", err)
	}
	_ = pki.SavePEM(filepath.Join(*outDir, "server.crt"), serverKP.CertPEM, 0644)
	_ = pki.SavePEM(filepath.Join(*outDir, "server.key"), serverKP.KeyPEM, 0600)
	log.Println("[PKI] Generated Server Certificate: server.crt, server.key")

	// 3. Agent Client Certificate
	agentKP, err := pki.GenerateClientCert(ca, *agentID, "ubuntu-host")
	if err != nil {
		log.Fatalf("Failed to generate agent certificate: %v", err)
	}
	_ = pki.SavePEM(filepath.Join(*outDir, "agent.crt"), agentKP.CertPEM, 0644)
	_ = pki.SavePEM(filepath.Join(*outDir, "agent.key"), agentKP.KeyPEM, 0600)
	log.Printf("[PKI] Generated Agent Certificate: agent.crt, agent.key (CN: %s)", *agentID)

	fmt.Println("\nPKI Generation Complete. Files:")
	fmt.Printf(" - CA:      %s\n", filepath.Join(*outDir, "ca.crt"))
	fmt.Printf(" - Server:  %s\n", filepath.Join(*outDir, "server.crt"))
	fmt.Printf(" - Agent:   %s\n", filepath.Join(*outDir, "agent.crt"))
}
