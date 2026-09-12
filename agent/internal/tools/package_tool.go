package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type PackageTool struct {
	action string
}

func NewPackageTool(action string) *PackageTool {
	return &PackageTool{action: action}
}

func (t *PackageTool) Name() string {
	return t.action
}

func (t *PackageTool) Description() string {
	return fmt.Sprintf("Apt package management tool: %s", t.action)
}

func (t *PackageTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	pkgName, _ := args["name"].(string)
	if pkgName == "" {
		pkgName, _ = args["package"].(string)
	}
	if pkgName == "" {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   "missing required 'name' or 'package' argument",
			Success:  false,
		}, nil
	}

	// Validate package name (alphanumeric, -, +, .)
	for _, ch := range pkgName {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' || ch == '.') {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("illegal package name: %s", pkgName),
				Success:  false,
			}, nil
		}
	}

	var cmd *exec.Cmd
	switch t.action {
	case "check_package":
		cmd = exec.CommandContext(ctx, "dpkg", "-s", pkgName)
	case "install_package":
		// DEBIAN_FRONTEND=noninteractive sudo apt-get install -y <pkg>
		cmd = exec.CommandContext(ctx, "sudo", "apt-get", "install", "-y", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", pkgName)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")
	default:
		return nil, fmt.Errorf("unknown package action: %s", t.action)
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

	isInstalled := false
	if t.action == "check_package" && exitCode == 0 && strings.Contains(stdout.String(), "Status: install ok installed") {
		isInstalled = true
	} else if t.action == "install_package" && exitCode == 0 {
		isInstalled = true
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Success:  exitCode == 0,
		Data: map[string]any{
			"package":   pkgName,
			"installed": isInstalled,
		},
	}, nil
}
