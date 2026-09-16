package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"opspilot/control-plane/internal/auth"
	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/orchestrator"
	"opspilot/control-plane/internal/pki"
	"opspilot/control-plane/internal/policy"
	"opspilot/control-plane/internal/statemachine"
	"opspilot/control-plane/internal/verification"
)

func setupTestRouter() (http.Handler, *pki.CertificateAuthority) {
	store := database.NewMemoryStore()
	authService := auth.NewService("test-secret", 24)
	machine := statemachine.NewMachine()
	policyEngine := policy.NewEngine()
	verifier := verification.NewEngine()
	hub := events.NewHub()
	orch := orchestrator.NewOrchestrator(store, machine, policyEngine, verifier, hub, nil, "http://localhost:8000")
	ca, _ := pki.GenerateCA("OpsPilot Enterprise Root CA")

	router := NewRouterWithPKI(store, authService, orch, hub, machine, ca, "valid-bootstrap-token")
	return router, ca
}

func TestPKIEnroll_Success(t *testing.T) {
	router, ca := setupTestRouter()

	body, _ := json.Marshal(map[string]string{
		"bootstrap_token": "valid-bootstrap-token",
		"hostname":        "srv-alpha.prod",
		"agent_id":        "agent-srv-alpha",
	})

	req := httptest.NewRequest("POST", "/api/v1/pki/enroll", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Approved      bool   `json:"approved"`
		AgentID       string `json:"agent_id"`
		ClientCertPEM string `json:"client_cert_pem"`
		ClientKeyPEM  string `json:"client_key_pem"`
		CACertPEM     string `json:"ca_cert_pem"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !res.Approved || res.AgentID != "agent-srv-alpha" {
		t.Errorf("unexpected approval or agent_id: %+v", res)
	}

	// Validate returned cert against CA
	if _, err := pki.ValidateCertificate([]byte(res.ClientCertPEM), ca.CertPEM); err != nil {
		t.Errorf("returned client cert failed validation against CA: %v", err)
	}
}

func TestPKIEnroll_InvalidToken_Unauthorized(t *testing.T) {
	router, _ := setupTestRouter()

	body, _ := json.Marshal(map[string]string{
		"bootstrap_token": "wrong-token",
		"hostname":        "srv-alpha.prod",
	})

	req := httptest.NewRequest("POST", "/api/v1/pki/enroll", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

func TestPKIRenew_Success(t *testing.T) {
	router, ca := setupTestRouter()

	// 1. Initial enroll
	enrollBody, _ := json.Marshal(map[string]string{
		"bootstrap_token": "valid-bootstrap-token",
		"hostname":        "srv-beta.prod",
		"agent_id":        "agent-srv-beta",
	})
	enrollReq := httptest.NewRequest("POST", "/api/v1/pki/enroll", bytes.NewBuffer(enrollBody))
	enrollReq.Header.Set("Content-Type", "application/json")
	enrollRec := httptest.NewRecorder()
	router.ServeHTTP(enrollRec, enrollReq)

	var enrollRes struct {
		ClientCertPEM string `json:"client_cert_pem"`
	}
	_ = json.Unmarshal(enrollRec.Body.Bytes(), &enrollRes)

	// 2. Renew cert
	renewBody, _ := json.Marshal(map[string]string{
		"client_cert_pem": enrollRes.ClientCertPEM,
	})
	renewReq := httptest.NewRequest("POST", "/api/v1/pki/renew", bytes.NewBuffer(renewBody))
	renewReq.Header.Set("Content-Type", "application/json")
	renewRec := httptest.NewRecorder()
	router.ServeHTTP(renewRec, renewReq)

	if renewRec.Code != http.StatusOK {
		t.Fatalf("expected renew status 200, got %d: %s", renewRec.Code, renewRec.Body.String())
	}

	var renewRes struct {
		Approved      bool   `json:"approved"`
		AgentID       string `json:"agent_id"`
		ClientCertPEM string `json:"client_cert_pem"`
	}
	if err := json.Unmarshal(renewRec.Body.Bytes(), &renewRes); err != nil {
		t.Fatalf("failed to decode renew response: %v", err)
	}

	if !renewRes.Approved || renewRes.AgentID != "agent-srv-beta" {
		t.Errorf("unexpected renew result: %+v", renewRes)
	}

	// Verify renewed cert validates against CA
	if _, err := pki.ValidateCertificate([]byte(renewRes.ClientCertPEM), ca.CertPEM); err != nil {
		t.Errorf("renewed cert failed validation: %v", err)
	}
}

func TestPKIGetCA_Success(t *testing.T) {
	router, ca := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/pki/ca", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res struct {
		CACertPEM string `json:"ca_cert_pem"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.CACertPEM != string(ca.CertPEM) {
		t.Errorf("expected CA PEM %s, got %s", string(ca.CertPEM), res.CACertPEM)
	}
}
