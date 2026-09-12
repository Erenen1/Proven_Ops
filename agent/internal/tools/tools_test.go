package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileToolAndBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "opspilot_file_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.conf")

	fileTool := NewFileTool("write_config_file")
	readTool := NewFileTool("read_file")

	// 1. First write
	initialContent := "worker_processes 1;\nevents { worker_connections 1024; }"
	res, err := fileTool.Execute(context.Background(), map[string]any{
		"path":    testFile,
		"content": initialContent,
	})
	if err != nil || !res.Success {
		t.Fatalf("write failed: %v, res: %+v", err, res)
	}

	// 2. Read back
	readRes, err := readTool.Execute(context.Background(), map[string]any{
		"path": testFile,
	})
	if err != nil || !readRes.Success {
		t.Fatalf("read failed: %v", err)
	}
	if readRes.Stdout != initialContent {
		t.Errorf("read content mismatch. Expected %s, got %s", initialContent, readRes.Stdout)
	}

	// 3. Second write should create a .bak file
	updatedContent := "worker_processes 4;\nevents { worker_connections 2048; }"
	res2, err := fileTool.Execute(context.Background(), map[string]any{
		"path":    testFile,
		"content": updatedContent,
	})
	if err != nil || !res2.Success {
		t.Fatalf("second write failed: %v", err)
	}

	backupPath, ok := res2.Data["backup_path"].(string)
	if !ok || backupPath == "" {
		t.Fatalf("expected backup_path in res2 Data, got: %+v", res2.Data)
	}

	// Verify backup content matches initial content
	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup file %s: %v", backupPath, err)
	}
	if string(backupBytes) != initialContent {
		t.Errorf("backup content mismatch: got %s, expected %s", string(backupBytes), initialContent)
	}
}

func TestRegistryTools(t *testing.T) {
	reg := NewRegistry()
	toolsList := reg.List()
	if len(toolsList) < 10 {
		t.Errorf("expected at least 10 registered tools, got %d", len(toolsList))
	}

	for _, name := range []string{"get_os_info", "check_package", "restart_service", "read_file", "check_port"} {
		tool, err := reg.Get(name)
		if err != nil || tool == nil {
			t.Errorf("expected tool '%s' to be present", name)
		}
	}

	// Test get_os_info execution
	osTool, _ := reg.Get("get_os_info")
	res, err := osTool.Execute(context.Background(), nil)
	if err != nil || !res.Success {
		t.Errorf("get_os_info failed: %v", err)
	}
	if !strings.Contains(res.Stdout, "Hostname:") {
		t.Errorf("expected Hostname in get_os_info output, got: %s", res.Stdout)
	}
}
