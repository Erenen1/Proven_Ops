package executor

import (
	"context"
	"path/filepath"
	"testing"

	"opspilot/agent/internal/tools"
)

func TestRunnerStepExecutionIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_runner_ledger.db")
	ledger, err := NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	defer ledger.Close()

	registry := tools.NewRegistry()
	guard := NewCommandGuard()
	runner := NewRunnerWithLedger(registry, guard, ledger)
	ctx := context.Background()

	execID := "task-1/step-1/attempt-1"

	// First execution of read-only tool
	res1, err := runner.ExecuteWithID(ctx, execID, "get_os_info", "{}", 10)
	if err != nil || !res1.Success {
		t.Fatalf("first execution failed: %v", err)
	}
	if res1.Data["cached_execution"] == true {
		t.Errorf("first execution should not be cached")
	}

	// Second execution with same execution ID
	res2, err := runner.ExecuteWithID(ctx, execID, "get_os_info", "{}", 10)
	if err != nil || !res2.Success {
		t.Fatalf("second execution failed: %v", err)
	}
	if res2.Data["idempotent"] != true || res2.Data["cached_execution"] != true {
		t.Errorf("second execution must be returned from idempotency cache: %+v", res2.Data)
	}
}

func TestPersistentLedgerProcessRestart(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ledger.db")

	ledger1, err := NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	registry := tools.NewRegistry()
	guard := NewCommandGuard()
	runner1 := NewRunnerWithLedger(registry, guard, ledger1)
	ctx := context.Background()

	execID := "task-restart/step-1/attempt-0"

	// 1. First execution succeeds
	res1, err := runner1.ExecuteWithID(ctx, execID, "get_os_info", "{}", 10)
	if err != nil || !res1.Success {
		t.Fatalf("runner1 execution failed: %v", err)
	}
	_ = ledger1.Close()

	// 2. Simulate agent process restart with new Runner instance opening the same DB
	ledger2, err := NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to re-open ledger after restart: %v", err)
	}
	defer ledger2.Close()
	runner2 := NewRunnerWithLedger(registry, guard, ledger2)

	res2, err := runner2.ExecuteWithID(ctx, execID, "get_os_info", "{}", 10)
	if err != nil || !res2.Success {
		t.Fatalf("runner2 replay execution failed: %v", err)
	}
	if res2.Data["idempotent"] != true || res2.Data["persistent_ledger"] != true {
		t.Fatalf("expected persistent ledger cached result, got: %+v", res2.Data)
	}
}

func TestPersistentLedgerUncertainExecution(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ledger_crash.db")

	ledger1, err := NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	execID := "task-crash/step-mutating/attempt-0"

	// Simulate step started and agent process crash while RUNNING
	_ = ledger1.RecordStart(execID, "task-crash", "step-mutating", "install_package")
	_ = ledger1.Close()

	// Agent restarts: NewLedger converts RUNNING to UNKNOWN
	ledger2, err := NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to re-open ledger: %v", err)
	}
	defer ledger2.Close()

	registry := tools.NewRegistry()
	guard := NewCommandGuard()
	runner2 := NewRunnerWithLedger(registry, guard, ledger2)
	ctx := context.Background()

	// Re-attempting the mutating operation must return UNCERTAIN_EXECUTION instead of executing
	res, err := runner2.ExecuteWithID(ctx, execID, "install_package", `{"name":"nginx"}`, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Fatalf("expected UNCERTAIN_EXECUTION failure, but got success")
	}
	if res.Data["uncertain_execution"] != true {
		t.Fatalf("expected uncertain_execution data flag, got: %+v", res.Data)
	}
}
