package saga

import (
	"context"
	"testing"

	"opspilot/control-plane/internal/models"
)

type mockCompensationExecutor struct {
	executedPlans []*models.AIStepPlan
	verifyPassed  bool
}

func (m *mockCompensationExecutor) ExecuteCompensatingStep(ctx context.Context, agentID string, step *models.AIStepPlan) (*models.TaskStep, error) {
	m.executedPlans = append(m.executedPlans, step)
	return &models.TaskStep{
		ID:     step.ID,
		Action: step.Action,
		Status: "SUCCESS",
	}, nil
}

func (m *mockCompensationExecutor) VerifyCompensatingStep(ctx context.Context, agentID string, strategy *models.VerificationStrategy) (bool, string, error) {
	if m.verifyPassed {
		return true, "verified active", nil
	}
	return false, "target check failed", nil
}

func TestSagaLIFOCompensation(t *testing.T) {
	ctx := context.Background()
	mockExec := &mockCompensationExecutor{verifyPassed: true}

	step1 := &models.TaskStep{
		ID:        "step-1",
		Action:    "ensure_package",
		Arguments: map[string]any{"name": "nginx", "state": "present"},
		Status:    "SUCCESS",
	}

	step2 := &models.TaskStep{
		ID:             "step-2",
		Action:         "write_config_file",
		Arguments:      map[string]any{"path": "/etc/nginx/nginx.conf"},
		PrecheckResult: map[string]any{"backup_path": "/var/lib/opspilot/backups/task-1/nginx.conf.bak"},
		Status:         "SUCCESS",
	}

	step3 := &models.TaskStep{
		ID:        "step-3",
		Action:    "start_service",
		Arguments: map[string]any{"name": "nginx"},
		Status:    "SUCCESS",
	}

	succeeded := []*models.TaskStep{step1, step2, step3}

	report := ExecuteSagaRollback(ctx, mockExec, "agent-1", succeeded)

	if report.OverallStatus != RollbackSucceeded {
		t.Fatalf("expected RollbackSucceeded, got %s", report.OverallStatus)
	}

	if len(mockExec.executedPlans) != 3 {
		t.Fatalf("expected 3 compensating plans executed, got %d", len(mockExec.executedPlans))
	}

	// In LIFO order:
	// 1st compensated must be step-3 (stop_service)
	if mockExec.executedPlans[0].Action != "stop_service" {
		t.Errorf("expected 1st compensation to be stop_service, got %s", mockExec.executedPlans[0].Action)
	}

	// 2nd compensated must be step-2 (rollback_config)
	if mockExec.executedPlans[1].Action != "rollback_config" {
		t.Errorf("expected 2nd compensation to be rollback_config, got %s", mockExec.executedPlans[1].Action)
	}

	// 3rd compensated must be step-1 (ensure_package state: absent)
	if mockExec.executedPlans[2].Action != "ensure_package" {
		t.Errorf("expected 3rd compensation to be ensure_package, got %s", mockExec.executedPlans[2].Action)
	}
}

func TestSagaIrreversibleActionHandling(t *testing.T) {
	ctx := context.Background()
	mockExec := &mockCompensationExecutor{verifyPassed: true}

	step1 := &models.TaskStep{
		ID:        "step-high-risk",
		Action:    "execute_command",
		Arguments: map[string]any{"command": "fdisk -l"},
		RiskLevel: models.RiskHigh,
		Status:    "SUCCESS",
	}

	report := ExecuteSagaRollback(ctx, mockExec, "agent-1", []*models.TaskStep{step1})

	if report.OverallStatus != ManualInterventionRequired {
		t.Fatalf("expected ManualInterventionRequired for high-risk irreversible action, got %s", report.OverallStatus)
	}
	if len(report.IrreversibleSteps) != 1 {
		t.Errorf("expected 1 irreversible step recorded")
	}
}
