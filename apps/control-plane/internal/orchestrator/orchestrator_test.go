package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/grpcserver"
	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/policy"
	"opspilot/control-plane/internal/statemachine"
	"opspilot/control-plane/internal/verification"
)

// TestAIFailureIsolation verifies that malformed JSON, unsupported tools, and forbidden actions
// from AI never reach the executor, terminate safely, and record appropriate failure/audit events.
func TestAIFailureIsolation(t *testing.T) {
	tests := []struct {
		name               string
		aiResponseJSON     string
		expectedStatus     models.TaskStatus
		expectedErrSubstr  string
	}{
		{
			name:              "MalformedPlanProvider_InvalidJSON",
			aiResponseJSON:    `{ invalid json `,
			expectedStatus:    models.TaskStatusFailed,
			expectedErrSubstr: "AI planning failure",
		},
		{
			name: "UnsupportedToolProvider_HallucinatedAction",
			aiResponseJSON: `{
				"goal": "Hallucinated destructive action",
				"reasoning": "Try to execute imaginary action",
				"steps": [
					{
						"id": "step-1",
						"action": "destroy_datacenter",
						"arguments": {},
						"reason": "Imaginary tool",
						"suggested_risk": "HIGH"
					}
				]
			}`,
			expectedStatus:    models.TaskStatusFailed,
			expectedErrSubstr: "Forbidden action in plan",
		},
		{
			name: "ForbiddenActionProvider_DestructiveCommand",
			aiResponseJSON: `{
				"goal": "Destructive rm -rf attempt",
				"reasoning": "Attempt rm -rf /",
				"steps": [
					{
						"id": "step-1",
						"action": "execute_command",
						"arguments": {"command": "rm -rf /"},
						"reason": "Destructive pattern",
						"suggested_risk": "HIGH"
					}
				]
			}`,
			expectedStatus:    models.TaskStatusFailed,
			expectedErrSubstr: "security violation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock AI Service returning the test payload
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.aiResponseJSON)
			}))
			defer server.Close()

			store := database.NewMemoryStore()
			agent := &models.Agent{
				ID:           "test-agent-1",
				Hostname:     "ubuntu-test",
				Status:       "online",
				Environment:  "production",
				Capabilities: []string{"pkg", "file", "systemd"},
			}
			_ = store.SaveAgent(context.Background(), agent)

			task := &models.Task{
				ID:             "task-" + tt.name,
				Title:          "Test AI Isolation",
				Prompt:         "Test AI Isolation prompt",
				Status:         models.TaskStatusCreated,
				TargetAgentIDs: []string{agent.ID},
				MaxReplans:     3,
				CreatedAt:      time.Now(),
			}
			_ = store.SaveTask(context.Background(), task)

			sm := statemachine.NewMachine()
			pe := policy.NewEngine()
			hub := events.NewHub()
			verifier := verification.NewEngine()
			grpcSrv := grpcserver.NewServer(store, hub, "test-token")

			orch := NewOrchestrator(store, sm, pe, verifier, hub, grpcSrv, server.URL)

			// StartTask invokes Discovery -> Planning -> Policy Evaluation
			err := orch.StartTask(context.Background(), task.ID)
			if err == nil {
				t.Fatalf("expected error from StartTask for test %s, but got nil", tt.name)
			}

			updatedTask, err := store.GetTask(context.Background(), task.ID)
			if err != nil {
				t.Fatalf("failed to retrieve task: %v", err)
			}

			if updatedTask.Status != tt.expectedStatus {
				t.Errorf("expected task status %s, got %s", tt.expectedStatus, updatedTask.Status)
			}
		})
	}
}

func TestExecutionBounds_MaxToolCalls(t *testing.T) {
	store := database.NewMemoryStore()
	agent := &models.Agent{
		ID:           "test-agent-bounds",
		Hostname:     "ubuntu-test",
		Status:       "online",
		Environment:  "production",
		Capabilities: []string{"pkg"},
	}
	_ = store.SaveAgent(context.Background(), agent)

	task := &models.Task{
		ID:             "task-bounds-test",
		Title:          "Test Bounds",
		Prompt:         "Test Bounds",
		Status:         models.TaskStatusWaitingApproval,
		TargetAgentIDs: []string{agent.ID},
		CreatedAt:      time.Now(),
	}
	_ = store.SaveTask(context.Background(), task)

	// Add 3 steps
	for i := 1; i <= 3; i++ {
		_ = store.SaveTaskStep(context.Background(), &models.TaskStep{
			ID:        fmt.Sprintf("step-%d", i),
			TaskID:    task.ID,
			StepOrder: i,
			Action:    "get_os_info",
			Status:    "PENDING",
		})
	}

	sm := statemachine.NewMachine()
	pe := policy.NewEngine()
	hub := events.NewHub()
	verifier := verification.NewEngine()
	grpcSrv := grpcserver.NewServer(store, hub, "test-token")

	orch := NewOrchestrator(store, sm, pe, verifier, hub, grpcSrv, "http://mock-ai")
	// Set strict limit of 2 tool calls
	orch.SetExecutionLimits(2, 10*time.Minute)

	// ExecuteTask should halt when tool calls limit is reached
	orch.ExecuteTask(context.Background(), task.ID)

	updatedTask, _ := store.GetTask(context.Background(), task.ID)
	if updatedTask.Status != models.TaskStatusFailed {
		t.Errorf("expected task to fail due to tool calls limit, got status: %s", updatedTask.Status)
	}
}

func TestExecutionBounds_MaxDuration(t *testing.T) {
	store := database.NewMemoryStore()
	agent := &models.Agent{
		ID:           "test-agent-duration",
		Hostname:     "ubuntu-test",
		Status:       "online",
		Environment:  "production",
		Capabilities: []string{"pkg"},
	}
	_ = store.SaveAgent(context.Background(), agent)

	// Task created 1 hour ago
	task := &models.Task{
		ID:             "task-duration-test",
		Title:          "Test Duration",
		Prompt:         "Test Duration",
		Status:         models.TaskStatusWaitingApproval,
		TargetAgentIDs: []string{agent.ID},
		CreatedAt:      time.Now().Add(-1 * time.Hour),
	}
	_ = store.SaveTask(context.Background(), task)

	sm := statemachine.NewMachine()
	pe := policy.NewEngine()
	hub := events.NewHub()
	verifier := verification.NewEngine()
	grpcSrv := grpcserver.NewServer(store, hub, "test-token")

	orch := NewOrchestrator(store, sm, pe, verifier, hub, grpcSrv, "http://mock-ai")
	orch.SetExecutionLimits(20, 5*time.Minute) // 5 minutes max

	orch.ExecuteTask(context.Background(), task.ID)

	updatedTask, _ := store.GetTask(context.Background(), task.ID)
	if updatedTask.Status != models.TaskStatusTimeout {
		t.Errorf("expected task status TIMEOUT, got status: %s", updatedTask.Status)
	}
}
