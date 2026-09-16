package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	StateAlreadyDesired = "SKIPPED_ALREADY_DESIRED"
	StateNeedsChange     = "NEEDS_CHANGE"
	StateChanged         = "CHANGED"
)

// Allowed mutation prefixes for filesystem security guardrail
var defaultAllowedPaths = []string{
	"/etc",
	"/opt",
	"/var/lib/opspilot",
	"/var/log",
	"/var/www",
	"/tmp",
}

// ValidateFilesystemPath guards against path traversal, symlink attacks, and disallowed roots
func ValidateFilesystemPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("empty path provided")
	}

	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("relative paths are forbidden: %s", path)
	}

	// Check path traversal pattern
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path traversal detected: %s", path)
	}

	// Forbid dangerous system targets
	forbiddenRoots := []string{"/proc", "/sys", "/dev", "/boot", "/root/.ssh", "/etc/shadow", "/etc/sudoers"}
	for _, f := range forbiddenRoots {
		if clean == f || strings.HasPrefix(clean, f+"/") {
			return "", fmt.Errorf("modification of protected system path is forbidden: %s", clean)
		}
	}

	// If file exists or symlink exists, verify target
	if fi, err := os.Lstat(clean); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(clean)
			if err != nil {
				return "", fmt.Errorf("failed resolving symlink for %s: %w", clean, err)
			}
			for _, f := range forbiddenRoots {
				if resolved == f || strings.HasPrefix(resolved, f+"/") {
					return "", fmt.Errorf("symlink points to protected system path %s: forbidden", resolved)
				}
			}
		}
	}

	return clean, nil
}

// ============================================================================
// 1. EnsurePackageTool
// ============================================================================
type EnsurePackageTool struct{}

func NewEnsurePackageTool() *EnsurePackageTool {
	return &EnsurePackageTool{}
}

func (t *EnsurePackageTool) Name() string {
	return "ensure_package"
}

func (t *EnsurePackageTool) Description() string {
	return "Idempotent desired-state package management (present/absent)"
}

func (t *EnsurePackageTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	pkgName, _ := args["name"].(string)
	if pkgName == "" {
		pkgName, _ = args["package"].(string)
	}
	if pkgName == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'name' or 'package' argument", Success: false}, nil
	}

	state, _ := args["state"].(string)
	if state == "" {
		state = "present"
	}
	state = strings.ToLower(state)

	// Precheck: check if package is installed
	checkCmd := exec.CommandContext(ctx, "dpkg", "-s", pkgName)
	var checkOut bytes.Buffer
	checkCmd.Stdout = &checkOut
	isInstalled := (checkCmd.Run() == nil && strings.Contains(checkOut.String(), "Status: install ok installed"))

	if state == "present" {
		if isInstalled {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("Package '%s' is already in desired state (installed).", pkgName),
				Success:  true,
				Data: map[string]any{
					"package":    pkgName,
					"state":      "present",
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}

		// State needs change: install package
		cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", "-q", pkgName)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		if err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stdout:   outBuf.String(),
				Stderr:   fmt.Sprintf("failed to install package %s: %s %v", pkgName, errBuf.String(), err),
				Success:  false,
			}, nil
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Package '%s' successfully installed.", pkgName),
			Success:  true,
			Data: map[string]any{
				"package": pkgName,
				"state":   "present",
				"status":  StateChanged,
			},
		}, nil
	}

	if state == "absent" {
		if !isInstalled {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("Package '%s' is already in desired state (absent).", pkgName),
				Success:  true,
				Data: map[string]any{
					"package":    pkgName,
					"state":      "absent",
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}

		// State needs change: remove package
		cmd := exec.CommandContext(ctx, "apt-get", "remove", "-y", "-q", pkgName)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		if err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stdout:   outBuf.String(),
				Stderr:   fmt.Sprintf("failed to remove package %s: %s %v", pkgName, errBuf.String(), err),
				Success:  false,
			}, nil
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Package '%s' successfully removed.", pkgName),
			Success:  true,
			Data: map[string]any{
				"package": pkgName,
				"state":   "absent",
				"status":  StateChanged,
			},
		}, nil
	}

	return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("invalid state '%s', expected 'present' or 'absent'", state), Success: false}, nil
}

// ============================================================================
// 2. EnsureServiceTool
// ============================================================================
type EnsureServiceTool struct{}

func NewEnsureServiceTool() *EnsureServiceTool {
	return &EnsureServiceTool{}
}

func (t *EnsureServiceTool) Name() string {
	return "ensure_service"
}

func (t *EnsureServiceTool) Description() string {
	return "Idempotent desired-state systemd service management (running/stopped/enabled)"
}

func (t *EnsureServiceTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	svcName, _ := args["name"].(string)
	if svcName == "" {
		svcName, _ = args["service"].(string)
	}
	if svcName == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'name' or 'service' argument", Success: false}, nil
	}

	state, _ := args["state"].(string)
	if state == "" {
		state = "running"
	}
	state = strings.ToLower(state)

	// Precheck: check if active
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", svcName)
	var outBuf bytes.Buffer
	statusCmd.Stdout = &outBuf
	isActive := (statusCmd.Run() == nil && strings.TrimSpace(outBuf.String()) == "active")

	if state == "running" && isActive {
		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Service '%s' is already in desired state (active/running).", svcName),
			Success:  true,
			Data: map[string]any{
				"service":    svcName,
				"state":      "running",
				"status":     StateAlreadyDesired,
				"idempotent": true,
			},
		}, nil
	}

	if state == "stopped" && !isActive {
		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Service '%s' is already in desired state (stopped/inactive).", svcName),
			Success:  true,
			Data: map[string]any{
				"service":    svcName,
				"state":      "stopped",
				"status":     StateAlreadyDesired,
				"idempotent": true,
			},
		}, nil
	}

	// Apply mutation
	action := "start"
	if state == "stopped" {
		action = "stop"
	} else if state == "restarted" {
		action = "restart"
	}

	cmd := exec.CommandContext(ctx, "systemctl", action, svcName)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("systemctl %s %s failed: %s %v", action, svcName, errBuf.String(), err),
			Success:  false,
		}, nil
	}

	// Verify if enabled is requested
	if enabled, ok := args["enabled"].(bool); ok && enabled {
		_ = exec.CommandContext(ctx, "systemctl", "enable", svcName).Run()
	}

	return &ExecutionResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("Service '%s' transitioned to state '%s'.", svcName, state),
		Success:  true,
		Data: map[string]any{
			"service": svcName,
			"state":   state,
			"status":  StateChanged,
		},
	}, nil
}

// ============================================================================
// 3. EnsureFileTool (Atomic replacement + Precheck + Backup)
// ============================================================================
type EnsureFileTool struct{}

func NewEnsureFileTool() *EnsureFileTool {
	return &EnsureFileTool{}
}

func (t *EnsureFileTool) Name() string {
	return "ensure_file"
}

func (t *EnsureFileTool) Description() string {
	return "Idempotent desired-state file creation with atomic replacement and backup snapshot"
}

func (t *EnsureFileTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		rawPath, _ = args["target"].(string)
	}
	if rawPath == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'path' argument", Success: false}, nil
	}

	cleanPath, err := ValidateFilesystemPath(rawPath)
	if err != nil {
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("path validation rejected: %v", err), Success: false}, nil
	}

	content, _ := args["content"].(string)
	if content == "" {
		content, _ = args["data"].(string)
	}

	desiredHashBytes := sha256.Sum256([]byte(content))
	desiredHash := hex.EncodeToString(desiredHashBytes[:])

	// Precheck: check if file exists and hash matches
	if existingBytes, err := os.ReadFile(cleanPath); err == nil {
		existingHashBytes := sha256.Sum256(existingBytes)
		existingHash := hex.EncodeToString(existingHashBytes[:])
		if existingHash == desiredHash {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("File '%s' content is already up-to-date (SHA256: %s).", cleanPath, desiredHash[:12]),
				Success:  true,
				Data: map[string]any{
					"path":       cleanPath,
					"sha256":     desiredHash,
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}
	}

	// Prepare backup snapshot
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		taskID = "global"
	}
	backupDir := filepath.Join("/var/lib/opspilot/backups", taskID)
	_ = os.MkdirAll(backupDir, 0700)
	backupPath := ""

	if _, err := os.Stat(cleanPath); err == nil {
		backupPath = filepath.Join(backupDir, fmt.Sprintf("%s.bak.%d", filepath.Base(cleanPath), time.Now().UnixNano()))
		if srcF, err := os.Open(cleanPath); err == nil {
			if dstF, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600); err == nil {
				_, _ = io.Copy(dstF, srcF)
				_ = dstF.Sync()
				dstF.Close()
			}
			srcF.Close()
		}
	}

	// Atomic Write: write to temporary file, sync, then atomic rename
	parentDir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("failed to create directory %s: %v", parentDir, err), Success: false}, nil
	}

	tmpFile, err := os.CreateTemp(parentDir, ".opspilot_tmp_*")
	if err != nil {
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("failed to create temp file: %v", err), Success: false}, nil
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpName)
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("failed writing to temp file: %v", err), Success: false}, nil
	}

	// Explicit fsync
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpName)
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("failed syncing temp file: %v", err), Success: false}, nil
	}
	tmpFile.Close()

	// Mode
	modeStr, _ := args["mode"].(string)
	mode := os.FileMode(0644)
	if modeStr != "" {
		if parsed, err := strconv.ParseUint(modeStr, 8, 32); err == nil {
			mode = os.FileMode(parsed)
		}
	}
	_ = os.Chmod(tmpName, mode)

	// Atomic Rename
	if err := os.Rename(tmpName, cleanPath); err != nil {
		_ = os.Remove(tmpName)
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("atomic file rename failed: %v", err), Success: false}, nil
	}

	return &ExecutionResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("File '%s' successfully written (SHA256: %s, atomic swap).", cleanPath, desiredHash[:12]),
		Success:  true,
		Data: map[string]any{
			"path":        cleanPath,
			"sha256":      desiredHash,
			"backup_path": backupPath,
			"status":      StateChanged,
		},
	}, nil
}

// ============================================================================
// 4. EnsureDirectoryTool
// ============================================================================
type EnsureDirectoryTool struct{}

func NewEnsureDirectoryTool() *EnsureDirectoryTool {
	return &EnsureDirectoryTool{}
}

func (t *EnsureDirectoryTool) Name() string {
	return "ensure_directory"
}

func (t *EnsureDirectoryTool) Description() string {
	return "Idempotent desired-state directory management (present/absent)"
}

func (t *EnsureDirectoryTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'path' argument", Success: false}, nil
	}

	cleanPath, err := ValidateFilesystemPath(rawPath)
	if err != nil {
		return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("path validation rejected: %v", err), Success: false}, nil
	}

	state, _ := args["state"].(string)
	if state == "" {
		state = "present"
	}
	state = strings.ToLower(state)

	fi, err := os.Stat(cleanPath)
	exists := (err == nil && fi.IsDir())

	if state == "present" {
		if exists {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("Directory '%s' already exists in desired state.", cleanPath),
				Success:  true,
				Data: map[string]any{
					"path":       cleanPath,
					"state":      "present",
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}

		mode := os.FileMode(0755)
		if modeStr, ok := args["mode"].(string); ok && modeStr != "" {
			if parsed, err := strconv.ParseUint(modeStr, 8, 32); err == nil {
				mode = os.FileMode(parsed)
			}
		}

		if err := os.MkdirAll(cleanPath, mode); err != nil {
			return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("failed to create directory %s: %v", cleanPath, err), Success: false}, nil
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Directory '%s' successfully created.", cleanPath),
			Success:  true,
			Data: map[string]any{
				"path":   cleanPath,
				"state":  "present",
				"status": StateChanged,
			},
		}, nil
	}

	if state == "absent" {
		if !exists {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("Directory '%s' is already absent in desired state.", cleanPath),
				Success:  true,
				Data: map[string]any{
					"path":       cleanPath,
					"state":      "absent",
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}

		if err := os.RemoveAll(cleanPath); err != nil {
			return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("failed removing directory %s: %v", cleanPath, err), Success: false}, nil
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Directory '%s' successfully removed.", cleanPath),
			Success:  true,
			Data: map[string]any{
				"path":   cleanPath,
				"state":  "absent",
				"status": StateChanged,
			},
		}, nil
	}

	return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("invalid state '%s'", state), Success: false}, nil
}

// ============================================================================
// 5. EnsurePortStateTool
// ============================================================================
type EnsurePortStateTool struct{}

func NewEnsurePortStateTool() *EnsurePortStateTool {
	return &EnsurePortStateTool{}
}

func (t *EnsurePortStateTool) Name() string {
	return "ensure_port_state"
}

func (t *EnsurePortStateTool) Description() string {
	return "Idempotent port state verification (open/closed)"
}

func (t *EnsurePortStateTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	portVal, ok := args["port"]
	if !ok {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'port' argument", Success: false}, nil
	}
	port := fmt.Sprintf("%v", portVal)

	expectedState, _ := args["state"].(string)
	if expectedState == "" {
		expectedState = "open"
	}
	expectedState = strings.ToLower(expectedState)

	timeout := 2 * time.Second
	if sec, ok := args["timeout_seconds"].(float64); ok && sec > 0 {
		timeout = time.Duration(sec) * time.Second
	}

	addr := net.JoinHostPort("127.0.0.1", port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	isOpen := (err == nil)
	if conn != nil {
		_ = conn.Close()
	}

	if expectedState == "open" {
		if isOpen {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("Port %s is OPEN and responsive.", port),
				Success:  true,
				Data: map[string]any{
					"port":       port,
					"state":      "open",
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("Port %s is CLOSED (connection failed: %v)", port, err),
			Success:  false,
			Data:     map[string]any{"port": port, "state": "closed"},
		}, nil
	}

	if expectedState == "closed" {
		if !isOpen {
			return &ExecutionResult{
				ExitCode: 0,
				Stdout:   fmt.Sprintf("Port %s is CLOSED as desired.", port),
				Success:  true,
				Data: map[string]any{
					"port":       port,
					"state":      "closed",
					"status":     StateAlreadyDesired,
					"idempotent": true,
				},
			}, nil
		}
		return &ExecutionResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("Port %s is OPEN, expected CLOSED", port),
			Success:  false,
			Data:     map[string]any{"port": port, "state": "open"},
		}, nil
	}

	return &ExecutionResult{ExitCode: 1, Stderr: fmt.Sprintf("invalid state '%s'", expectedState), Success: false}, nil
}
