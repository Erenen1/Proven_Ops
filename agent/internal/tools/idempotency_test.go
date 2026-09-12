package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileToolIdempotency(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.conf")

	tool := NewFileTool("write_config_file")
	ctx := context.Background()

	content := "server { listen 8080; }"

	// First write
	res1, err := tool.Execute(ctx, map[string]any{
		"path":    filePath,
		"content": content,
		"task_id": "test-task",
	})
	if err != nil || !res1.Success {
		t.Fatalf("first write failed: %v, res: %+v", err, res1)
	}
	if res1.Data["idempotent"] == true {
		t.Errorf("first write should not be marked idempotent")
	}

	// Second write with identical content
	res2, err := tool.Execute(ctx, map[string]any{
		"path":    filePath,
		"content": content,
		"task_id": "test-task",
	})
	if err != nil || !res2.Success {
		t.Fatalf("second write failed: %v, res: %+v", err, res2)
	}
	if res2.Data["idempotent"] != true {
		t.Errorf("second write should be marked idempotent no-op")
	}
	if res2.Data["status"] != "ALREADY_SATISFIED" {
		t.Errorf("expected status ALREADY_SATISFIED, got %v", res2.Data["status"])
	}
}

func TestFileToolRollback(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.conf")
	backupPath := filepath.Join(dir, "test.conf.bak")

	origContent := "original content"
	_ = os.WriteFile(backupPath, []byte(origContent), 0644)
	_ = os.WriteFile(filePath, []byte("broken content"), 0644)

	tool := NewFileTool("rollback_config")
	ctx := context.Background()

	res, err := tool.Execute(ctx, map[string]any{
		"path":        filePath,
		"backup_path": backupPath,
	})
	if err != nil || !res.Success {
		t.Fatalf("rollback failed: %v, res: %+v", err, res)
	}

	data, _ := os.ReadFile(filePath)
	if string(data) != origContent {
		t.Errorf("expected restored content %q, got %q", origContent, string(data))
	}
}
