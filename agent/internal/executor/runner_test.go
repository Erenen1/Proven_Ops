package executor

import (
	"context"
	"testing"

	"opspilot/agent/internal/tools"
)

func TestRunnerStepExecutionIdempotency(t *testing.T) {
	registry := tools.NewRegistry()
	guard := NewCommandGuard()
	runner := NewRunner(registry, guard)
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
