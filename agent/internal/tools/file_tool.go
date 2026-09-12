package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FileTool struct {
	action string
}

func NewFileTool(action string) *FileTool {
	return &FileTool{action: action}
}

func (t *FileTool) Name() string {
	return t.action
}

func (t *FileTool) Description() string {
	return fmt.Sprintf("File management tool: %s", t.action)
}

func (t *FileTool) Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path, _ = args["file_path"].(string)
	}
	if path == "" {
		path, _ = args["target"].(string)
	}
	if path == "" {
		return &ExecutionResult{ExitCode: 1, Stderr: "missing 'path' or 'file_path' argument", Success: false}, nil
	}

	cleanPath := filepath.Clean(path)

	switch t.action {
	case "read_file":
		f, err := os.Open(cleanPath)
		if err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("cannot open file: %v", err),
				Success:  false,
			}, nil
		}
		defer f.Close()

		// Read up to 256KB to avoid massive log file dumps into memory
		content, err := io.ReadAll(io.LimitReader(f, 256*1024))
		if err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("cannot read file: %v", err),
				Success:  false,
			}, nil
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   string(content),
			Success:  true,
			Data:     map[string]any{"path": cleanPath, "bytes_read": len(content)},
		}, nil

	case "write_config_file":
		content, _ := args["content"].(string)
		if content == "" {
			content, _ = args["file_content"].(string)
		}
		if content == "" {
			content, _ = args["data"].(string)
		}

		// 1. Idempotency Pre-check: if file exists and has identical content, skip write
		if existing, err := os.ReadFile(cleanPath); err == nil {
			if string(existing) == content {
				return &ExecutionResult{
					ExitCode: 0,
					Stdout:   fmt.Sprintf("Configuration at %s is already up to date (idempotent no-op).", cleanPath),
					Success:  true,
					Data: map[string]any{
						"path":       cleanPath,
						"idempotent": true,
						"status":     "ALREADY_SATISFIED",
					},
				}, nil
			}
		}

		backupPath := ""
		taskID, _ := args["task_id"].(string)
		if taskID == "" {
			taskID = "default"
		}

		// 2. Prepare persistent backup directory: /var/lib/opspilot/backups/{task_id}
		backupDir := filepath.Join("/var/lib/opspilot/backups", taskID)
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			// Fallback to /tmp if /var/lib not writable
			backupDir = filepath.Join("/tmp/opspilot/backups", taskID)
			_ = os.MkdirAll(backupDir, 0755)
		}

		// If original file exists, copy to backup
		hadPrevious := false
		if _, err := os.Stat(cleanPath); err == nil {
			hadPrevious = true
			baseName := filepath.Base(cleanPath)
			backupPath = filepath.Join(backupDir, fmt.Sprintf("%s.bak.%d", baseName, time.Now().Unix()))
			src, err := os.Open(cleanPath)
			if err == nil {
				dst, err := os.Create(backupPath)
				if err == nil {
					_, _ = io.Copy(dst, src)
					dst.Close()
				}
				src.Close()
			}
		}

		// Ensure parent directory exists
		_ = os.MkdirAll(filepath.Dir(cleanPath), 0755)

		// 3. Write new content
		if err := os.WriteFile(cleanPath, []byte(content), 0644); err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("failed to write file: %v", err),
				Success:  false,
			}, nil
		}

		// 4. Validate Configuration (e.g. nginx -t)
		validateCmd, _ := args["validate_command"].(string)
		if validateCmd == "" && strings.Contains(cleanPath, "nginx") {
			validateCmd = "nginx -t"
		}

		if validateCmd != "" {
			vCmd := exec.CommandContext(ctx, "bash", "-c", validateCmd)
			var vStdout, vStderr bytes.Buffer
			vCmd.Stdout = &vStdout
			vCmd.Stderr = &vStderr
			if err := vCmd.Run(); err != nil {
				// Syntax / Configuration validation FAILED!
				// Execute IMMEDIATE transaction rollback to restore system safety
				if hadPrevious && backupPath != "" {
					bSrc, bErr := os.ReadFile(backupPath)
					if bErr == nil {
						_ = os.WriteFile(cleanPath, bSrc, 0644)
					}
				} else {
					_ = os.Remove(cleanPath)
				}

				return &ExecutionResult{
					ExitCode: 1,
					Stdout:   vStdout.String(),
					Stderr:   fmt.Sprintf("Configuration validation failed (%s): %s; automatically rolled back to original", validateCmd, strings.TrimSpace(vStderr.String())),
					Success:  false,
					Data: map[string]any{
						"path":             cleanPath,
						"backup_path":      backupPath,
						"validation_error": vStderr.String(),
						"auto_rolled_back": true,
					},
				}, nil
			}
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Configuration written to %s and validated (backup: %s)", cleanPath, backupPath),
			Success:  true,
			Data: map[string]any{
				"path":        cleanPath,
				"backup_path": backupPath,
			},
		}, nil

	case "rollback_config":
		backupPath, _ := args["backup_path"].(string)
		if backupPath == "" {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   "missing 'backup_path' argument for rollback_config",
				Success:  false,
			}, nil
		}

		if _, err := os.Stat(backupPath); err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("backup file not found: %s", backupPath),
				Success:  false,
			}, nil
		}

		backupData, err := os.ReadFile(backupPath)
		if err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("failed to read backup file: %v", err),
				Success:  false,
			}, nil
		}

		if err := os.WriteFile(cleanPath, backupData, 0644); err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("failed to restore backup to %s: %v", cleanPath, err),
				Success:  false,
			}, nil
		}

		// Re-validate if nginx config
		if strings.Contains(cleanPath, "nginx") {
			vCmd := exec.CommandContext(ctx, "bash", "-c", "nginx -t")
			if err := vCmd.Run(); err != nil {
				return &ExecutionResult{
					ExitCode: 1,
					Stderr:   fmt.Sprintf("Restored configuration at %s failed validation: %v", cleanPath, err),
					Success:  false,
				}, nil
			}
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Successfully restored configuration at %s from %s", cleanPath, backupPath),
			Success:  true,
			Data: map[string]any{
				"path":        cleanPath,
				"backup_path": backupPath,
				"restored":    true,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown file action: %s", t.action)
	}
}
