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
}
