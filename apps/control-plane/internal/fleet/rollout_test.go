package fleet

import (
	"context"
	"fmt"
	"testing"

	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/models"
)

func TestComputeBatches(t *testing.T) {
	eng := NewEngine(nil, nil)
	agents := []string{"agent-1", "agent-2", "agent-3", "agent-4", "agent-5"}

	// 1. All at once
	b1 := eng.ComputeBatches(agents, &models.RolloutConfig{Strategy: models.RolloutAllAtOnce})
	if len(b1) != 1 || len(b1[0]) != 5 {
		t.Errorf("expected 1 batch with 5 agents, got %v", b1)
	}

	// 2. Canary with 1 node, then remainder
	b2 := eng.ComputeBatches(agents, &models.RolloutConfig{
		Strategy:    models.RolloutCanary,
		CanaryNodes: 1,
	})
	if len(b2) != 2 || len(b2[0]) != 1 || len(b2[1]) != 4 {
		t.Errorf("expected 2 batches ([1], [4]), got %v", b2)
	}

	// 3. Canary with 2 nodes, then batches of 2
	b3 := eng.ComputeBatches(agents, &models.RolloutConfig{
		Strategy:    models.RolloutCanary,
		CanaryNodes: 2,
		BatchSize:   2,
	})
	if len(b3) != 3 || len(b3[0]) != 2 || len(b3[1]) != 2 || len(b3[2]) != 1 {
		t.Errorf("expected 3 batches ([2], [2], [1]), got %v", b3)
	}

	// 4. Rolling batches of 2
	b4 := eng.ComputeBatches(agents, &models.RolloutConfig{
		Strategy:  models.RolloutRolling,
		BatchSize: 2,
	})
	if len(b4) != 3 || len(b4[0]) != 2 || len(b4[1]) != 2 || len(b4[2]) != 1 {
		t.Errorf("expected 3 batches ([2], [2], [1]), got %v", b4)
	}
}

func TestExecuteRolloutSuccess(t *testing.T) {
	store := database.NewMemoryStore()
	hub := events.NewHub()
	eng := NewEngine(store, hub)

	task := &models.Task{
		ID:             "task-fleet-1",
		Title:          "Multi-node deploy",
		Status:         models.TaskStatusCreated,
		TargetAgentIDs: []string{"a1", "a2", "a3"},
		RolloutConfig: &models.RolloutConfig{
			Strategy:    models.RolloutCanary,
			CanaryNodes: 1,
		},
	}
	_ = store.SaveTask(context.Background(), task)

	executor := func(ctx context.Context, t *models.Task, agentID string) error {
		return nil
	}

	err := eng.ExecuteRollout(context.Background(), task, executor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Status != models.TaskStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", task.Status)
	}
	if task.RolloutProgress.CompletedHosts != 3 {
		t.Errorf("expected 3 completed hosts, got %d", task.RolloutProgress.CompletedHosts)
	}
	if task.RolloutProgress.FailedHosts != 0 {
		t.Errorf("expected 0 failed hosts, got %d", task.RolloutProgress.FailedHosts)
	}
}

func TestExecuteRolloutCanaryHaltOnFailure(t *testing.T) {
	store := database.NewMemoryStore()
	hub := events.NewHub()
	eng := NewEngine(store, hub)

	task := &models.Task{
		ID:             "task-fleet-halt",
		Title:          "Canary deployment with failure",
		Status:         models.TaskStatusCreated,
		TargetAgentIDs: []string{"canary-1", "node-2", "node-3"},
		RolloutConfig: &models.RolloutConfig{
			Strategy:    models.RolloutCanary,
			CanaryNodes: 1,
			MaxFailures: 1,
		},
	}
	_ = store.SaveTask(context.Background(), task)

	// Canary fails
	executor := func(ctx context.Context, t *models.Task, agentID string) error {
		if agentID == "canary-1" {
			return fmt.Errorf("canary healthcheck failed")
		}
		return nil
	}

	err := eng.ExecuteRollout(context.Background(), task, executor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Status != models.TaskStatusFailed {
		t.Errorf("expected status FAILED due to halt, got %s", task.Status)
	}
	if !task.RolloutProgress.Halted {
		t.Errorf("expected rollout to be halted")
	}
	if task.RolloutProgress.CompletedHosts != 0 {
		t.Errorf("expected 0 completed hosts, got %d", task.RolloutProgress.CompletedHosts)
	}
	// The other 2 nodes should remain in created/unexecuted state
	if task.RolloutProgress.HostStates["node-2"].Status != models.TaskStatusCreated {
		t.Errorf("expected node-2 to be unexecuted (CREATED), got %s", task.RolloutProgress.HostStates["node-2"].Status)
	}
}
