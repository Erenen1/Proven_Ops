package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		backupPath := ""

		// If file exists, create automatic backup
		if _, err := os.Stat(cleanPath); err == nil {
			backupPath = fmt.Sprintf("%s.bak.%d", cleanPath, time.Now().Unix())
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

		// Write new content
		if err := os.WriteFile(cleanPath, []byte(content), 0644); err != nil {
			return &ExecutionResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("failed to write file: %v", err),
				Success:  false,
			}, nil
		}

		return &ExecutionResult{
			ExitCode: 0,
			Stdout:   fmt.Sprintf("Configuration written to %s (backup: %s)", cleanPath, backupPath),
			Success:  true,
			Data: map[string]any{
				"path":        cleanPath,
				"backup_path": backupPath,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown file action: %s", t.action)
	}
}
