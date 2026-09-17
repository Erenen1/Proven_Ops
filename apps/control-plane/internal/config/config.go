package config

import (
	"fmt"
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

func getEnv(keys ...string) string {
	for _, k := range keys {
		if val := os.Getenv(k); val != "" {
			return val
		}
	}
	return ""
}

func LoadConfig() *Config {
	httpPort := 8080
	if val, err := strconv.Atoi(getEnv("PROVENOPS_HTTP_PORT", "OPSPILOT_HTTP_PORT", "CONTROL_PLANE_HTTP_PORT")); err == nil && val > 0 {
		httpPort = val
	}

	grpcPort := 9090
	if val, err := strconv.Atoi(getEnv("PROVENOPS_GRPC_PORT", "OPSPILOT_GRPC_PORT", "CONTROL_PLANE_GRPC_PORT")); err == nil && val > 0 {
		grpcPort = val
	}

	dbURL := getEnv("PROVENOPS_DATABASE_URL", "OPSPILOT_DATABASE_URL", "DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://opspilot:opspilot_dev_secret@localhost:5432/opspilot?sslmode=disable"
	}

	jwtSecret := getEnv("PROVENOPS_JWT_SECRET", "OPSPILOT_JWT_SECRET", "JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super_secret_jwt_signing_key_change_in_production_min_32_bytes"
	}

	bootstrapToken := getEnv("PROVENOPS_BOOTSTRAP_TOKEN", "OPSPILOT_BOOTSTRAP_TOKEN", "BOOTSTRAP_TOKEN")
	if bootstrapToken == "" {
		bootstrapToken = "opspilot-default-bootstrap-token-2026"
	}

	aiServiceURL := getEnv("PROVENOPS_AI_SERVICE_URL", "OPSPILOT_AI_SERVICE_URL", "AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}

	env := getEnv("PROVENOPS_ENVIRONMENT", "OPSPILOT_ENVIRONMENT", "ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	tlsVal := getEnv("PROVENOPS_TLS_ENABLED", "OPSPILOT_TLS_ENABLED", "TLS_ENABLED")
	tlsEnabled := tlsVal == "true" || tlsVal == "1"

	return &Config{
		DatabaseURL:         dbURL,
		HTTPPort:            httpPort,
		GRPCPort:            grpcPort,
		JWTSecret:           jwtSecret,
		BootstrapToken:      bootstrapToken,
		AIServiceURL:        aiServiceURL,
		DefaultHeartbeatSec: 5,
		Environment:         env,
		TLSEnabled:          tlsEnabled,
		TLSCACert:           getEnv("PROVENOPS_TLS_CA_CERT", "OPSPILOT_TLS_CA_CERT", "TLS_CA_CERT"),
		TLSServerCert:       getEnv("PROVENOPS_TLS_SERVER_CERT", "OPSPILOT_TLS_SERVER_CERT", "TLS_SERVER_CERT"),
		TLSServerKey:        getEnv("PROVENOPS_TLS_SERVER_KEY", "OPSPILOT_TLS_SERVER_KEY", "TLS_SERVER_KEY"),
	}
}

// Validate ensures production environments do not run with insecure defaults
func (c *Config) Validate() error {
	if c.Environment == "production" {
		if c.JWTSecret == "super_secret_jwt_signing_key_change_in_production_min_32_bytes" {
			return fmt.Errorf("insecure default JWT_SECRET cannot be used in production")
		}
		if c.BootstrapToken == "opspilot-default-bootstrap-token-2026" {
			return fmt.Errorf("insecure default BOOTSTRAP_TOKEN cannot be used in production")
		}
		if c.DatabaseURL == "" {
			return fmt.Errorf("DATABASE_URL is required in production")
		}
	}
	return nil
}
