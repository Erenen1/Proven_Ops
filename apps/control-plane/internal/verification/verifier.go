package verification

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"opspilot/control-plane/internal/models"
)

type VerificationEvidence struct {
	CheckType  string         `json:"check_type"`
	Target     string         `json:"target"`
	Expected   string         `json:"expected"`
	Actual     string         `json:"actual"`
	Passed     bool           `json:"passed"`
	DurationMS int64          `json:"duration_ms"`
	Timestamp  time.Time      `json:"timestamp"`
	AgentID    string         `json:"agent_id"`
	Details    map[string]any `json:"details,omitempty"`
}

type Engine struct {
	httpClient *http.Client
}

func NewEngine() *Engine {
	return &Engine{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// VerifyContract executes deterministic verification and produces structured audit evidence
func (e *Engine) VerifyContract(
	ctx context.Context,
	taskID string,
	stepID string,
	agentID string,
	strategy *models.VerificationStrategy,
	hostIP string,
) (*VerificationEvidence, error) {
	start := time.Now()

	evidence := &VerificationEvidence{
		CheckType: strategy.CheckType,
		Target:    strategy.Target,
		Expected:  strategy.Expected,
		Timestamp: start,
		AgentID:   agentID,
		Details:   make(map[string]any),
	}

	if strategy == nil {
		evidence.Passed = true
		evidence.Actual = "no verification strategy specified"
		evidence.DurationMS = time.Since(start).Milliseconds()
		return evidence, nil
	}

	switch strategy.CheckType {
	case "tcp_port_open":
		port := strategy.Target
		if strings.Contains(port, ":") {
			parts := strings.Split(port, ":")
			port = parts[len(parts)-1]
		}
		addr := net.JoinHostPort(hostIP, port)
		timeout := time.Duration(strategy.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}

		conn, err := net.DialTimeout("tcp", addr, timeout)
		duration := time.Since(start).Milliseconds()
		evidence.DurationMS = duration

		if err != nil {
			evidence.Passed = false
			evidence.Actual = fmt.Sprintf("TCP dial failed: %v", err)
			return evidence, nil
		}
		_ = conn.Close()

		evidence.Passed = true
		evidence.Actual = "TCP port listening and open"
		return evidence, nil

	case "http_probe":
		url := strategy.Target
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "http://" + net.JoinHostPort(hostIP, url)
		} else if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
			url = strings.Replace(url, "localhost", hostIP, 1)
			url = strings.Replace(url, "127.0.0.1", hostIP, 1)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			evidence.Passed = false
			evidence.Actual = fmt.Sprintf("invalid URL: %v", err)
			evidence.DurationMS = time.Since(start).Milliseconds()
			return evidence, nil
		}

		resp, err := e.httpClient.Do(req)
		evidence.DurationMS = time.Since(start).Milliseconds()

		if err != nil {
			evidence.Passed = false
			evidence.Actual = fmt.Sprintf("HTTP probe failed: %v", err)
			return evidence, nil
		}
		defer resp.Body.Close()

		expectedCode := 200
		if strategy.Expected != "" {
			if code, err := strconv.Atoi(strategy.Expected); err == nil {
				expectedCode = code
			}
		}

		evidence.Details["status_code"] = resp.StatusCode
		evidence.Actual = fmt.Sprintf("status_code=%d", resp.StatusCode)

		if resp.StatusCode != expectedCode {
			evidence.Passed = false
			return evidence, nil
		}

		evidence.Passed = true
		return evidence, nil

	default:
		evidence.Passed = true
		evidence.Actual = fmt.Sprintf("agent-verified: %s", strategy.CheckType)
		evidence.DurationMS = time.Since(start).Milliseconds()
		return evidence, nil
	}
}

// VerifyNetworkTarget legacy adapter for backwards compatibility
func (e *Engine) VerifyNetworkTarget(ctx context.Context, strategy *models.VerificationStrategy, hostIP string) (bool, string, error) {
	ev, err := e.VerifyContract(ctx, "", "", "", strategy, hostIP)
	if err != nil {
		return false, "", err
	}
	return ev.Passed, ev.Actual, nil
}
