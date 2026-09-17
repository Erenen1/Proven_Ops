package verification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"opspilot/control-plane/internal/models"
)

func TestVerifyContractHTTPProbe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	engine := NewEngine()
	ctx := context.Background()

	strat := &models.VerificationStrategy{
		CheckType:  "http_probe",
		Target:     ts.URL,
		Expected:   "200",
		TimeoutSec: 2,
	}

	ev, err := engine.VerifyContract(ctx, "task-1", "step-1", "agent-1", strat, "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev.Passed {
		t.Fatalf("expected probe to pass, actual: %s", ev.Actual)
	}
	if ev.CheckType != "http_probe" {
		t.Errorf("expected http_probe, got %s", ev.CheckType)
	}
	if ev.DurationMS < 0 {
		t.Errorf("expected positive duration")
	}

	// Failing status code
	stratFail := &models.VerificationStrategy{
		CheckType:  "http_probe",
		Target:     ts.URL,
		Expected:   "404",
		TimeoutSec: 2,
	}
	evFail, _ := engine.VerifyContract(ctx, "task-1", "step-1", "agent-1", stratFail, "127.0.0.1")
	if evFail.Passed {
		t.Errorf("expected verification to fail on status mismatch")
	}
}

func TestVerifyContractTCPPort(t *testing.T) {
	// Start a local TCP listener
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	parts := strings.Split(ts.URL, ":")
	portStr := parts[len(parts)-1]
	port, _ := strconv.Atoi(portStr)

	engine := NewEngine()
	ctx := context.Background()

	strat := &models.VerificationStrategy{
		CheckType:  "tcp_port_open",
		Target:     strconv.Itoa(port),
		TimeoutSec: 2,
	}

	ev, err := engine.VerifyContract(ctx, "task-1", "step-1", "agent-1", strat, "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev.Passed {
		t.Errorf("expected TCP port open check to pass: %s", ev.Actual)
	}
}
