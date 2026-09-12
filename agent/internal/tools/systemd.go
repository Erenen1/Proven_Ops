package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type SystemdTool struct {
	action string
}

func NewSystemdTool(action string) *SystemdTool {
	return &SystemdTool{action: action}
}

func (t *SystemdTool) Name() string {
	return t.action
}

func (t *SystemdTool) Description() string {
	return fmt.Sprintf("Systemd service operation: %s", t.action)
}

func (t *SystemdTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	serviceName, _ := args["name"].(string)
	if serviceName == "" {
		serviceName, _ = args["service"].(string)
	}
	if serviceName == "" {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   "missing required 'name' or 'service' argument",
			Success:  false,
		}, nil
	}

	// Sanitize service name
	if strings.ContainsAny(serviceName, ";|&$` \t\n") {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   "invalid characters in service name",
			Success:  false,
		}, nil
	}

	var cmd *exec.Cmd
	switch t.action {
	case "get_service_status":
		cmd = exec.CommandContext(ctx, "systemctl", "status", serviceName, "--no-pager")
	case "start_service":
		cmd = exec.CommandContext(ctx, "sudo", "systemctl", "start", serviceName)
	case "stop_service":
		cmd = exec.CommandContext(ctx, "sudo", "systemctl", "stop", serviceName)
	case "restart_service":
		cmd = exec.CommandContext(ctx, "sudo", "systemctl", "restart", serviceName)
	case "enable_service":
		cmd = exec.CommandContext(ctx, "sudo", "systemctl", "enable", serviceName)
	case "get_service_logs":
		cmd = exec.CommandContext(ctx, "journalctl", "-u", serviceName, "-n", "50", "--no-pager")
	default:
		return nil, fmt.Errorf("unknown systemd tool action: %s", t.action)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// For status, exit code 0 or 3 (inactive) is informative
	success := (exitCode == 0)
	if t.action == "get_service_status" || t.action == "get_service_logs" {
		success = true
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Success:  success,
		Data: map[string]any{
			"service": serviceName,
			"action":  t.action,
		},
	}, nil
}
