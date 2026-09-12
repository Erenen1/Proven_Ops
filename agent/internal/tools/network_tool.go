package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type NetworkTool struct {
	action string
}

func NewNetworkTool(action string) *NetworkTool {
	return &NetworkTool{action: action}
}

func (t *NetworkTool) Name() string {
	return t.action
}

func (t *NetworkTool) Description() string {
	return fmt.Sprintf("Network diagnostic tool: %s", t.action)
}

func (t *NetworkTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	switch t.action {
	case "check_port":
		return t.checkPort(ctx, args)
	case "http_probe":
		return t.httpProbe(ctx, args)
	case "get_open_ports":
		return t.getOpenPorts(ctx, args)
	case "dns_lookup":
		return t.dnsLookup(ctx, args)
	default:
		return nil, fmt.Errorf("unknown network action: %s", t.action)
	}
}

func (t *NetworkTool) checkPort(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	portVal := args["port"]
	if portVal == nil {
		portVal = args["port_number"]
	}
	if portVal == nil {
		portVal = args["port_num"]
	}
	if portVal == nil {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'port' or 'port_number' argument", Success: false}, nil
	}

	port := fmt.Sprintf("%v", portVal)
	host := "127.0.0.1"
	if h, ok := args["host"].(string); ok && h != "" {
		host = h
	}

	addr := net.JoinHostPort(host, port)
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("port %s is not open or unreachable: %v", addr, err),
			Success:  false,
			Data:     map[string]any{"port": port, "open": false},
		}, nil
	}
	_ = conn.Close()

	return &ExecutionResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("Port %s is open and accepting TCP connections", addr),
		Success:  true,
		Data:     map[string]any{"port": port, "open": true},
	}, nil
}

func (t *NetworkTool) httpProbe(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'url' argument", Success: false}, nil
	}

	expectedStatus := 200
	if exp, ok := args["expected_status"]; ok {
		switch v := exp.(type) {
		case float64:
			expectedStatus = int(v)
		case int:
			expectedStatus = v
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				expectedStatus = parsed
			}
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("invalid URL: %v", err), Success: false}, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("HTTP probe to %s failed: %v", url, err),
			Success:  false,
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	success := (resp.StatusCode == expectedStatus)

	stdout := fmt.Sprintf("HTTP %d OK from %s\n%s", resp.StatusCode, url, string(bodyBytes))
	var stderr string
	if !success {
		stderr = fmt.Sprintf("expected HTTP %d but received %d", expectedStatus, resp.StatusCode)
	}

	return &ExecutionResult{
		ExitCode: 0,
		Stdout:   stdout,
		Stderr:   stderr,
		Success:  success,
		Data: map[string]any{
			"url":         url,
			"status_code": resp.StatusCode,
			"match":       success,
		},
	}, nil
}

func (t *NetworkTool) getOpenPorts(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	cmd := exec.CommandContext(ctx, "ss", "-tlpn")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to netstat
		cmd = exec.CommandContext(ctx, "netstat", "-tlpn")
		out, _ = cmd.CombinedOutput()
	}

	return &ExecutionResult{
		ExitCode: 0,
		Stdout:   string(out),
		Success:  true,
	}, nil
}

func (t *NetworkTool) dnsLookup(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	host, _ := args["host"].(string)
	if host == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'host' argument", Success: false}, nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("DNS lookup failed: %v", err),
			Success:  false,
		}, nil
	}

	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}

	return &ExecutionResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("Resolved %s -> %s", host, strings.Join(ipStrings, ", ")),
		Success:  true,
		Data:     map[string]any{"host": host, "ips": ipStrings},
	}, nil
}
