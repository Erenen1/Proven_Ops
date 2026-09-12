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
