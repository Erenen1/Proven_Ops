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

func getEnv(keys ...string) string {
	for _, k := range keys {
		if val := os.Getenv(k); val != "" {
			return val
		}
	}
	return ""
}

func Load() *Config {
	addr := getEnv("PROVENOPS_CONTROL_PLANE_ADDR", "OPSPILOT_CONTROL_PLANE_ADDR", "AGENT_CONTROL_PLANE_ADDR")
	if addr == "" {
		addr = "localhost:9090"
	}

	token := getEnv("PROVENOPS_BOOTSTRAP_TOKEN", "OPSPILOT_BOOTSTRAP_TOKEN", "AGENT_BOOTSTRAP_TOKEN")
	if token == "" {
		token = "opspilot-default-bootstrap-token-2026"
	}

	hbInterval := 5
	if val, err := strconv.Atoi(getEnv("PROVENOPS_HEARTBEAT_INTERVAL_SEC", "OPSPILOT_HEARTBEAT_INTERVAL_SEC", "AGENT_HEARTBEAT_INTERVAL_SEC")); err == nil && val > 0 {
		hbInterval = val
	}

	env := getEnv("PROVENOPS_ENVIRONMENT", "OPSPILOT_ENVIRONMENT", "AGENT_ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	hostname, _ := os.Hostname()
	if h := getEnv("PROVENOPS_HOSTNAME", "OPSPILOT_HOSTNAME", "AGENT_HOSTNAME"); h != "" {
		hostname = h
	}

	workingDir := getEnv("PROVENOPS_WORKING_DIR", "OPSPILOT_WORKING_DIR", "AGENT_WORKING_DIR")
	if workingDir == "" {
		workingDir = "/tmp/provenops"
	}

	tlsVal := getEnv("PROVENOPS_TLS_ENABLED", "OPSPILOT_TLS_ENABLED", "TLS_ENABLED")
	tlsEnabled := tlsVal == "true" || tlsVal == "1"

	serverName := getEnv("PROVENOPS_TLS_SERVER_NAME", "OPSPILOT_TLS_SERVER_NAME", "TLS_SERVER_NAME")
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
		TLSCACert:            getEnv("PROVENOPS_TLS_CA_CERT", "OPSPILOT_TLS_CA_CERT", "TLS_CA_CERT"),
		TLSClientCert:        getEnv("PROVENOPS_TLS_CLIENT_CERT", "OPSPILOT_TLS_CLIENT_CERT", "TLS_CLIENT_CERT"),
		TLSClientKey:         getEnv("PROVENOPS_TLS_CLIENT_KEY", "OPSPILOT_TLS_CLIENT_KEY", "TLS_CLIENT_KEY"),
		TLSServerName:        serverName,
	}
}
