package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL          string
	HTTPPort             int
	GRPCPort             int
	JWTSecret            string
	BootstrapToken       string
	AIServiceURL         string
	DefaultHeartbeatSec  int
	Environment          string
	TLSEnabled           bool
	TLSCACert            string
	TLSServerCert        string
	TLSServerKey         string
}

func LoadConfig() *Config {
	httpPort := 8080
	if val, err := strconv.Atoi(os.Getenv("CONTROL_PLANE_HTTP_PORT")); err == nil {
		httpPort = val
	}

	grpcPort := 9090
	if val, err := strconv.Atoi(os.Getenv("CONTROL_PLANE_GRPC_PORT")); err == nil {
		grpcPort = val
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://opspilot:opspilot_dev_secret@localhost:5432/opspilot?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super_secret_jwt_signing_key_change_in_production_min_32_bytes"
	}

	bootstrapToken := os.Getenv("BOOTSTRAP_TOKEN")
	if bootstrapToken == "" {
		bootstrapToken = "opspilot-default-bootstrap-token-2026"
	}

	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}

	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	tlsEnabled := os.Getenv("TLS_ENABLED") == "true" || os.Getenv("TLS_ENABLED") == "1"

	return &Config{
		DatabaseURL:         dbURL,
		HTTPPort:            httpPort,
		GRPCPort:            grpcPort,
		JWTSecret:           jwtSecret,
		BootstrapToken:      bootstrapToken,
		AIServiceURL:        aiServiceURL,
		DefaultHeartbeatSec: 5,
		Environment:          env,
		TLSEnabled:           tlsEnabled,
		TLSCACert:            os.Getenv("TLS_CA_CERT"),
		TLSServerCert:        os.Getenv("TLS_SERVER_CERT"),
		TLSServerKey:         os.Getenv("TLS_SERVER_KEY"),
	}
}
