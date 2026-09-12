package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type DockerTool struct {
	action string
}

func NewDockerTool(action string) *DockerTool {
	return &DockerTool{action: action}
}

func (t *DockerTool) Name() string {
	return t.action
}

func (t *DockerTool) Description() string {
	return fmt.Sprintf("Docker container tool: %s", t.action)
}

func (t *DockerTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	var cmd *exec.Cmd

	switch t.action {
	case "docker_info":
		cmd = exec.CommandContext(ctx, "docker", "info", "--format", "{{json .}}")
	case "docker_ps":
		cmd = exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Ports}}")
	case "docker_logs":
		container, _ := args["container"].(string)
		if container == "" {
			return &ExecutionResult{ExitCode: 1, Stderr: "missing 'container' argument", Success: false}, nil
		}
		cmd = exec.CommandContext(ctx, "docker", "logs", "--tail", "50", container)
	case "docker_inspect":
		container, _ := args["container"].(string)
		if container == "" {
			return &ExecutionResult{ExitCode: 1, Stderr: "missing 'container' argument", Success: false}, nil
		}
		cmd = exec.CommandContext(ctx, "docker", "inspect", container)
	default:
		return nil, fmt.Errorf("unknown docker action: %s", t.action)
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

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Success:  exitCode == 0,
	}, nil
}
