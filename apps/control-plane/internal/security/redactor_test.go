package security

import (
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	// Private key
	rawKey := "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEI...secret...data\n-----END EC PRIVATE KEY-----"
	redacted := RedactString("Server key is: " + rawKey)
	if strings.Contains(redacted, "secret") {
		t.Errorf("private key was not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED_PRIVATE_KEY]") {
		t.Errorf("expected [REDACTED_PRIVATE_KEY], got: %s", redacted)
	}

	// JWT
	jwt := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.doNotLeak"
	redactedJWT := RedactString(jwt)
	if strings.Contains(redactedJWT, "doNotLeak") {
		t.Errorf("JWT signature leaked: %s", redactedJWT)
	}

	// URI credentials
	dbURL := "postgres://opspilot:SuperSecret123@postgres-host:5432/opspilot"
	redactedURI := RedactString(dbURL)
	if strings.Contains(redactedURI, "SuperSecret123") {
		t.Errorf("DB password leaked: %s", redactedURI)
	}
	if !strings.Contains(redactedURI, "postgres://opspilot:[REDACTED]@postgres-host:5432/opspilot") {
		t.Errorf("expected redacted URI, got: %s", redactedURI)
	}
}

func TestRedactMap(t *testing.T) {
	input := map[string]any{
		"username": "admin",
		"password": "super_secret_password",
		"token":    "xyz12345678",
		"nested": map[string]any{
			"api_key": "secret-key-value",
			"host":    "127.0.0.1",
		},
	}

	cleaned := RedactMap(input)

	if cleaned["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED], got: %v", cleaned["password"])
	}
	if cleaned["token"] != "[REDACTED]" {
		t.Errorf("expected token to be [REDACTED], got: %v", cleaned["token"])
	}

	nested, ok := cleaned["nested"].(map[string]any)
	if !ok || nested["api_key"] != "[REDACTED]" {
		t.Errorf("expected nested api_key to be [REDACTED], got: %v", nested["api_key"])
	}
	if nested["host"] != "127.0.0.1" {
		t.Errorf("expected host to remain intact")
	}
}
