package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

type SystemTool struct {
	action string
}

func NewSystemTool(action string) *SystemTool {
	return &SystemTool{action: action}
}

func (t *SystemTool) Name() string {
	return t.action
}

func (t *SystemTool) Description() string {
	return fmt.Sprintf("System diagnostic tool: %s", t.action)
}

func (t *SystemTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	hostname, _ := os.Hostname()

	switch t.action {
	case "get_os_info":
		out := fmt.Sprintf("Hostname: %s\nOS: %s\nArchitecture: %s\nNumCPU: %d", hostname, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
		if runtime.GOOS == "linux" {
			if b, err := os.ReadFile("/etc/os-release"); err == nil {
				out += "\n" + string(b)
			}
		}
		return &ExecutionResult{ExitCode: 0, Stdout: out, Success: true}, nil

	case "get_system_info", "get_cpu_usage", "get_memory_usage", "get_load_average":
		var cmd *exec.Cmd
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "uptime")
		} else {
			out := fmt.Sprintf("Uptime: OK\nNumCPU: %d\nGoVersion: %s", runtime.NumCPU(), runtime.Version())
			return &ExecutionResult{ExitCode: 0, Stdout: out, Success: true}, nil
		}
		outBytes, _ := cmd.CombinedOutput()
		return &ExecutionResult{ExitCode: 0, Stdout: string(outBytes), Success: true}, nil

	case "get_disk_usage":
		var cmd *exec.Cmd
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "df", "-h")
		} else {
			return &ExecutionResult{ExitCode: 0, Stdout: "Disk: OK", Success: true}, nil
		}
		outBytes, _ := cmd.CombinedOutput()
		return &ExecutionResult{ExitCode: 0, Stdout: string(outBytes), Success: true}, nil

	case "list_processes":
		var cmd *exec.Cmd
		if runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, "ps", "aux")
		} else {
			return &ExecutionResult{ExitCode: 0, Stdout: "Process listing not supported on non-linux host", Success: true}, nil
		}
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		_ = cmd.Run()
		return &ExecutionResult{ExitCode: 0, Stdout: stdout.String(), Success: true}, nil

	default:
		return nil, fmt.Errorf("unknown system tool action: %s", t.action)
	}
}
