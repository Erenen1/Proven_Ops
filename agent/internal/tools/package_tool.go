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
		pkgName, _ = args["package_name"].(string)
	}
	if pkgName == "" {
		pkgName, _ = args["pkg"].(string)
	}
	if pkgName == "" {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   "missing required package argument (name/package/package_name)",
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

	var stdout, stderr bytes.Buffer

	switch t.action {
	case "check_package":
		cmd := exec.CommandContext(ctx, "dpkg", "-s", pkgName)
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
		isInstalled := (exitCode == 0 && strings.Contains(stdout.String(), "Status: install ok installed"))
		return &ExecutionResult{
			ExitCode: exitCode,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Success:  isInstalled,
			Data: map[string]any{
				"package":   pkgName,
				"installed": isInstalled,
			},
		}, nil

	case "install_package":
		cmd := exec.CommandContext(ctx, "sudo", "apt-get", "install", "-y", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", pkgName)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()

		if err != nil {
			// If failed (e.g. stale package index 404), update cache and retry once
			updateCmd := exec.CommandContext(ctx, "sudo", "apt-get", "update")
			_ = updateCmd.Run()

			stdout.Reset()
			stderr.Reset()
			retryCmd := exec.CommandContext(ctx, "sudo", "apt-get", "install", "-y", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", pkgName)
			retryCmd.Env = append(retryCmd.Environ(), "DEBIAN_FRONTEND=noninteractive")
			retryCmd.Stdout = &stdout
			retryCmd.Stderr = &stderr
			err = retryCmd.Run()
		}

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
			Data: map[string]any{
				"package":   pkgName,
				"installed": exitCode == 0,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown package action: %s", t.action)
	}
}
