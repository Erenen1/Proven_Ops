package statemachine

import (
	"testing"

	"opspilot/control-plane/internal/models"
)

func TestStateTransitions(t *testing.T) {
	sm := NewMachine()
	task := &models.Task{
		ID:     "task-123",
		Status: models.TaskStatusCreated,
	}

	// Valid transition: CREATED -> DISCOVERING
	tr, err := sm.Transition(task, models.TaskStatusDiscovering, "Discovery starting", "user-1")
	if err != nil {
		t.Fatalf("expected valid transition, got error: %v", err)
	}
	if task.Status != models.TaskStatusDiscovering {
		t.Errorf("expected status %s, got %s", models.TaskStatusDiscovering, task.Status)
	}
	if tr.FromStatus != models.TaskStatusCreated || tr.ToStatus != models.TaskStatusDiscovering {
		t.Errorf("transition history mismatch: %+v", tr)
	}

	// Valid transition: DISCOVERING -> PLANNING
	_, err = sm.Transition(task, models.TaskStatusPlanning, "Planning", "user-1")
	if err != nil {
		t.Fatalf("expected valid transition, got error: %v", err)
	}

	// Valid transition: PLANNING -> WAITING_APPROVAL
	_, err = sm.Transition(task, models.TaskStatusWaitingApproval, "Approval needed", "user-1")
	if err != nil {
		t.Fatalf("expected valid transition, got error: %v", err)
	}

	// Invalid transition: WAITING_APPROVAL -> COMPLETED (cannot bypass execution/verification)
	_, err = sm.Transition(task, models.TaskStatusCompleted, "Skip execution", "user-1")
	if err == nil {
		t.Fatalf("expected error for illegal transition, got nil")
	}

	// Valid transition: WAITING_APPROVAL -> EXECUTING
	_, err = sm.Transition(task, models.TaskStatusExecuting, "Approved", "user-1")
	if err != nil {
		t.Fatalf("expected valid transition, got error: %v", err)
	}

	// Valid transition: EXECUTING -> VERIFYING
	_, err = sm.Transition(task, models.TaskStatusVerifying, "Verifying steps", "system")
	if err != nil {
		t.Fatalf("expected valid transition, got error: %v", err)
	}

	// Valid transition: VERIFYING -> ROLLING_BACK (Verification failed, trigger rollback)
	_, err = sm.Transition(task, models.TaskStatusRollingBack, "Verification failed, rolling back", "system")
	if err != nil {
		t.Fatalf("expected valid transition to ROLLING_BACK, got error: %v", err)
	}

	// Valid transition: ROLLING_BACK -> ROLLED_BACK
	_, err = sm.Transition(task, models.TaskStatusRolledBack, "Rollback verified", "system")
	if err != nil {
		t.Fatalf("expected valid transition to ROLLED_BACK, got error: %v", err)
	}

	// Invalid transition: ROLLED_BACK -> COMPLETED (cannot resurrect rolled back task)
	_, err = sm.Transition(task, models.TaskStatusCompleted, "Cannot complete rolled back task", "system")
	if err == nil {
		t.Fatalf("expected error for illegal transition from ROLLED_BACK to COMPLETED, got nil")
	}
}

func TestReplanningAndAgentDisconnectTransitions(t *testing.T) {
	sm := NewMachine()
	task := &models.Task{
		ID:     "task-replan",
		Status: models.TaskStatusExecuting,
	}

	// EXECUTING -> OBSERVING -> REPLANNING -> WAITING_APPROVAL -> EXECUTING
	_, err := sm.Transition(task, models.TaskStatusObserving, "Failure observed", "system")
	if err != nil {
		t.Fatalf("expected valid transition to OBSERVING: %v", err)
	}

	_, err = sm.Transition(task, models.TaskStatusReplanning, "Replanning triggered", "system")
	if err != nil {
		t.Fatalf("expected valid transition to REPLANNING: %v", err)
	}

	_, err = sm.Transition(task, models.TaskStatusWaitingApproval, "Replan requires approval", "system")
	if err != nil {
		t.Fatalf("expected valid transition to WAITING_APPROVAL: %v", err)
	}

	_, err = sm.Transition(task, models.TaskStatusExecuting, "Replan approved", "user-1")
	if err != nil {
		t.Fatalf("expected valid transition to EXECUTING: %v", err)
	}

	// Agent disconnect: EXECUTING -> WAITING_FOR_AGENT -> OBSERVING -> EXECUTING
	_, err = sm.Transition(task, models.TaskStatusWaitingForAgent, "Agent disconnected", "system")
	if err != nil {
		t.Fatalf("expected valid transition to WAITING_FOR_AGENT: %v", err)
	}

	_, err = sm.Transition(task, models.TaskStatusObserving, "Agent reconnected, observing state", "system")
	if err != nil {
		t.Fatalf("expected valid transition to OBSERVING after reconnect: %v", err)
	}

	_, err = sm.Transition(task, models.TaskStatusExecuting, "Resuming execution", "system")
	if err != nil {
		t.Fatalf("expected valid transition to EXECUTING: %v", err)
	}

	// Rollback failure transition
	_, err = sm.Transition(task, models.TaskStatusRollingBack, "Rollback initiated", "system")
	if err != nil {
		t.Fatalf("expected transition to ROLLING_BACK: %v", err)
	}

	_, err = sm.Transition(task, models.TaskStatusRollbackFailed, "Rollback script failed", "system")
	if err != nil {
		t.Fatalf("expected transition to ROLLBACK_FAILED: %v", err)
	}
}

func TestProduction16StateLifecycle(t *testing.T) {
	sm := NewMachine()
	task := &models.Task{
		ID:                    "task-prod-16",
		Status:                models.TaskStatusPending,
		OptimisticLockVersion: 1,
	}

	// 1. PENDING -> PRECHECKING
	_, err := sm.Transition(task, models.TaskStatusPrechecking, "Prechecking desired state", "system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.OptimisticLockVersion != 2 {
		t.Errorf("expected version 2, got %d", task.OptimisticLockVersion)
	}

	// 2. PRECHECKING -> SKIPPED (Idempotent already satisfied)
	taskCopy := *task
	_, err = sm.Transition(&taskCopy, models.TaskStatusSkipped, "Already satisfied", "system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(&taskCopy, models.TaskStatusSucceeded, "Completed without mutations", "system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3. Normal path: PRECHECKING -> WAITING_APPROVAL -> QUEUED -> DISPATCHED -> RUNNING
	_, err = sm.Transition(task, models.TaskStatusWaitingApproval, "Needs approval", "system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusQueued, "Approved", "operator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusDispatched, "Assigned to worker", "orchestrator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusRunning, "Agent execution started", "agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4. RUNNING -> RETRYING -> RUNNING -> VERIFYING -> COMPENSATING -> COMPENSATED
	_, err = sm.Transition(task, models.TaskStatusRetrying, "Transient failure retry", "system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusRunning, "Retry execution", "orchestrator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusVerifying, "Verifying postconditions", "verifier")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusCompensating, "Verification failed, rolling back", "saga")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = sm.Transition(task, models.TaskStatusCompensated, "Rollback fully succeeded", "saga")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 5. Invalid resurrection from COMPENSATED
	_, err = sm.Transition(task, models.TaskStatusRunning, "Invalid rerun", "system")
	if err == nil {
		t.Fatalf("expected error resurrecting terminal compensated task, got nil")
	}
}
