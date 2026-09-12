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

// VerifyDirect probes network-accessible targets directly from Control Plane
func (e *Engine) VerifyNetworkTarget(ctx context.Context, strategy *models.VerificationStrategy, hostIP string) (bool, string, error) {
	if strategy == nil {
		return true, "no verification strategy specified", nil
	}

	switch strategy.CheckType {
	case "tcp_port_open":
		target := strategy.Target
		port := target
		// Target might be "8080" or "localhost:8080" or "0.0.0.0:8080"
		if strings.Contains(target, ":") {
			parts := strings.Split(target, ":")
			port = parts[len(parts)-1]
		}
		addr := net.JoinHostPort(hostIP, port)
		timeout := time.Duration(strategy.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}

		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return false, fmt.Sprintf("TCP dial to %s failed: %v", addr, err), nil
		}
		_ = conn.Close()
		return true, fmt.Sprintf("TCP port %s successfully verified as open", addr), nil

	case "http_probe":
		url := strategy.Target
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "http://" + net.JoinHostPort(hostIP, url)
		} else if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
			// Replace localhost with hostIP if verifying from control plane
			url = strings.Replace(url, "localhost", hostIP, 1)
			url = strings.Replace(url, "127.0.0.1", hostIP, 1)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return false, fmt.Sprintf("Invalid HTTP probe URL: %v", err), nil
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return false, fmt.Sprintf("HTTP probe failed to %s: %v", url, err), nil
		}
		defer resp.Body.Close()

		expectedCode := 200
		if strategy.Expected != "" {
			if code, err := strconv.Atoi(strategy.Expected); err == nil {
				expectedCode = code
			}
		}

		if resp.StatusCode != expectedCode {
			return false, fmt.Sprintf("HTTP probe expected status %d but received %d", expectedCode, resp.StatusCode), nil
		}

		return true, fmt.Sprintf("HTTP probe succeeded: %s returned status %d", url, resp.StatusCode), nil

	default:
		return true, fmt.Sprintf("strategy %s is verified agent-side", strategy.CheckType), nil
	}
}
