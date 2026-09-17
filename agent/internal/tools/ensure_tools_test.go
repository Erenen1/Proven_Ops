package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFilesystemPath(t *testing.T) {
	// Forbidden protected roots
	if _, err := ValidateFilesystemPath("/etc/shadow"); err == nil {
		t.Errorf("expected error for /etc/shadow, got nil")
	}
	if _, err := ValidateFilesystemPath("/boot/vmlinuz"); err == nil {
		t.Errorf("expected error for /boot/vmlinuz, got nil")
	}

	// Relative paths forbidden
	if _, err := ValidateFilesystemPath("relative/file.txt"); err == nil {
		t.Errorf("expected error for relative path, got nil")
	}

	// Path traversal with ..
	if _, err := ValidateFilesystemPath("/etc/nginx/../../etc/shadow"); err == nil {
		t.Errorf("expected error for path traversal with .., got nil")
	}
}

func TestEnsureFileIdempotencyAndAtomicSwap(t *testing.T) {
	tool := NewEnsureFileTool()
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "opspilot_ensure_file_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "app.conf")
	initialContent := "server { listen 80; }"

	// 1. First execution: should create file (StateChanged)
	res1, err := tool.Execute(ctx, map[string]any{
		"path":    targetFile,
		"content": initialContent,
		"task_id": "test-task-1",
	})
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}
	if !res1.Success || res1.ExitCode != 0 {
		t.Fatalf("expected success, got exit code %d: %s", res1.ExitCode, res1.Stderr)
	}
	if status, _ := res1.Data["status"].(string); status != StateChanged {
		t.Errorf("expected status %s, got %s", StateChanged, status)
	}

	// Verify content on disk
	data, err := os.ReadFile(targetFile)
	if err != nil || string(data) != initialContent {
		t.Fatalf("file content mismatch: expected %q, got %q (err: %v)", initialContent, string(data), err)
	}

	// 2. Second execution with identical content: should be SKIPPED_ALREADY_DESIRED
	res2, err := tool.Execute(ctx, map[string]any{
		"path":    targetFile,
		"content": initialContent,
		"task_id": "test-task-1",
	})
	if err != nil {
		t.Fatalf("second execution failed: %v", err)
	}
	if !res2.Success {
		t.Fatalf("expected second execution to succeed")
	}
	if status, _ := res2.Data["status"].(string); status != StateAlreadyDesired {
		t.Errorf("expected idempotent status %s, got %s", StateAlreadyDesired, status)
	}
	if idemp, _ := res2.Data["idempotent"].(bool); !idemp {
		t.Errorf("expected idempotent=true")
	}

	// 3. Third execution with updated content: should update and create backup
	updatedContent := "server { listen 443 ssl; }"
	res3, err := tool.Execute(ctx, map[string]any{
		"path":    targetFile,
		"content": updatedContent,
		"task_id": "test-task-1",
	})
	if err != nil {
		t.Fatalf("third execution failed: %v", err)
	}
	if !res3.Success {
		t.Fatalf("expected third execution to succeed")
	}
	if status, _ := res3.Data["status"].(string); status != StateChanged {
		t.Errorf("expected status %s on content update, got %s", StateChanged, status)
	}

	// Verify updated content on disk
	dataUpdated, err := os.ReadFile(targetFile)
	if err != nil || string(dataUpdated) != updatedContent {
		t.Fatalf("file updated content mismatch: expected %q, got %q", updatedContent, string(dataUpdated))
	}
}

func TestEnsureDirectoryIdempotency(t *testing.T) {
	tool := NewEnsureDirectoryTool()
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "opspilot_ensure_dir_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetDir := filepath.Join(tmpDir, "nested", "config")

	// 1. Create directory
	res1, err := tool.Execute(ctx, map[string]any{
		"path": targetDir,
	})
	if err != nil || !res1.Success {
		t.Fatalf("first ensure_directory execution failed: %v, res: %+v", err, res1)
	}
	if status, _ := res1.Data["status"].(string); status != StateChanged {
		t.Errorf("expected %s, got %s", StateChanged, status)
	}

	// 2. Second execution: already exists
	res2, err := tool.Execute(ctx, map[string]any{
		"path": targetDir,
	})
	if err != nil || !res2.Success {
		t.Fatalf("second ensure_directory execution failed: %v, res: %+v", err, res2)
	}
	if status, _ := res2.Data["status"].(string); status != StateAlreadyDesired {
		t.Errorf("expected %s, got %s", StateAlreadyDesired, status)
	}
}
