package config

import (
	"os"
	"strconv"
)

type Config struct {
	ControlPlaneAddr     string
	BootstrapToken       string
	HeartbeatIntervalSec int
	Environment          string
	AgentID              string
	Hostname             string
	WorkingDir           string
	TLSEnabled           bool
	TLSCACert            string
	TLSClientCert        string
	TLSClientKey         string
	TLSServerName        string
}

func Load() *Config {
	addr := os.Getenv("AGENT_CONTROL_PLANE_ADDR")
	if addr == "" {
		addr = "localhost:9090"
	}

	token := os.Getenv("AGENT_BOOTSTRAP_TOKEN")
	if token == "" {
		token = "opspilot-default-bootstrap-token-2026"
	}

	hbInterval := 5
	if val, err := strconv.Atoi(os.Getenv("AGENT_HEARTBEAT_INTERVAL_SEC")); err == nil && val > 0 {
		hbInterval = val
	}

	env := os.Getenv("AGENT_ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	hostname, _ := os.Hostname()
	if h := os.Getenv("AGENT_HOSTNAME"); h != "" {
		hostname = h
	}

	workingDir := os.Getenv("AGENT_WORKING_DIR")
	if workingDir == "" {
		workingDir = "/tmp/opspilot"
	}

	tlsEnabled := os.Getenv("TLS_ENABLED") == "true" || os.Getenv("TLS_ENABLED") == "1"
	serverName := os.Getenv("TLS_SERVER_NAME")
	if serverName == "" {
		serverName = "localhost"
	}

	return &Config{
		ControlPlaneAddr:     addr,
		BootstrapToken:       token,
		HeartbeatIntervalSec: hbInterval,
		Environment:          env,
		Hostname:             hostname,
		WorkingDir:           workingDir,
		TLSEnabled:           tlsEnabled,
		TLSCACert:            os.Getenv("TLS_CA_CERT"),
		TLSClientCert:        os.Getenv("TLS_CLIENT_CERT"),
		TLSClientKey:         os.Getenv("TLS_CLIENT_KEY"),
		TLSServerName:        serverName,
	}
}
