package saga

import (
	"context"
	"fmt"
	"time"

	"opspilot/control-plane/internal/models"
)

const (
	RollbackSucceeded            = "ROLLBACK_SUCCEEDED"
	RollbackPartial              = "ROLLBACK_PARTIAL"
	RollbackFailed               = "ROLLBACK_FAILED"
	ManualInterventionRequired   = "MANUAL_INTERVENTION_REQUIRED"
)

// ToolReversibilityMap defines whether an action can be safely and automatically compensated
var ToolReversibilityMap = map[string]string{
	"write_config_file":  models.ReversibilityFull,
	"ensure_file":        models.ReversibilityFull,
	"start_service":      models.ReversibilityFull,
	"stop_service":       models.ReversibilityFull,
	"restart_service":    models.ReversibilityPartial,
	"ensure_service":     models.ReversibilityFull,
	"install_package":    models.ReversibilityPartial,
	"ensure_package":     models.ReversibilityPartial,
	"ensure_directory":   models.ReversibilityPartial,
	"execute_command":    models.ReversibilityNone,
	"rollback_config":    models.ReversibilityNone,
}

// GetReversibility returns the reversibility metadata (FULL, PARTIAL, NONE)
func GetReversibility(action string) string {
	if rev, ok := ToolReversibilityMap[action]; ok {
		return rev
	}
	// Probes and getters have no mutation to reverse
	if isReadOnly(action) {
		return models.ReversibilityNone
	}
	return models.ReversibilityNone
}

func isReadOnly(action string) bool {
	return action == "read_file" || action == "check_port" || action == "http_probe" ||
		action == "get_service_status" || action == "check_package" || action == "ensure_port_state" ||
		len(action) >= 4 && action[:4] == "get_"
}

// DeriveCompensatingStep builds an explicit inverse action for a given executed step
func DeriveCompensatingStep(step *models.TaskStep) *models.AIStepPlan {
	switch step.Action {
	case "write_config_file", "ensure_file":
		path, _ := step.Arguments["path"].(string)
		backupPath := ""
		if step.PrecheckResult != nil {
			backupPath, _ = step.PrecheckResult["backup_path"].(string)
		}
		return &models.AIStepPlan{
			ID:     fmt.Sprintf("compensate-%s", step.ID),
			Action: "rollback_config",
			Arguments: map[string]any{
				"path":        path,
				"backup_path": backupPath,
			},
			Reason:        fmt.Sprintf("Compensating write to %s by restoring pre-flight snapshot", path),
			SuggestedRisk: models.RiskLow,
			VerificationStrategy: &models.VerificationStrategy{
				CheckType: "file_exists",
				Target:    path,
			},
		}

	case "start_service":
		svc, _ := step.Arguments["name"].(string)
		return &models.AIStepPlan{
			ID:     fmt.Sprintf("compensate-%s", step.ID),
			Action: "stop_service",
			Arguments: map[string]any{
				"name": svc,
			},
			Reason:        fmt.Sprintf("Compensating service start by stopping %s", svc),
			SuggestedRisk: models.RiskMedium,
			VerificationStrategy: &models.VerificationStrategy{
				CheckType: "systemd_active",
				Target:    svc,
				Expected:  "inactive",
			},
		}

	case "stop_service":
		svc, _ := step.Arguments["name"].(string)
		return &models.AIStepPlan{
			ID:     fmt.Sprintf("compensate-%s", step.ID),
			Action: "start_service",
			Arguments: map[string]any{
				"name": svc,
			},
			Reason:        fmt.Sprintf("Compensating service stop by restarting %s", svc),
			SuggestedRisk: models.RiskMedium,
			VerificationStrategy: &models.VerificationStrategy{
				CheckType: "systemd_active",
				Target:    svc,
				Expected:  "active",
			},
		}

	case "ensure_service":
		svc, _ := step.Arguments["name"].(string)
		state, _ := step.Arguments["state"].(string)
		inverseState := "stopped"
		if state == "stopped" {
			inverseState = "running"
		}
		return &models.AIStepPlan{
			ID:     fmt.Sprintf("compensate-%s", step.ID),
			Action: "ensure_service",
			Arguments: map[string]any{
				"name":  svc,
				"state": inverseState,
			},
			Reason:        fmt.Sprintf("Compensating ensure_service %s to inverse state %s", svc, inverseState),
			SuggestedRisk: models.RiskMedium,
		}

	case "install_package", "ensure_package":
		pkg, _ := step.Arguments["name"].(string)
		if pkg == "" {
			pkg, _ = step.Arguments["package"].(string)
		}
		return &models.AIStepPlan{
			ID:     fmt.Sprintf("compensate-%s", step.ID),
			Action: "ensure_package",
			Arguments: map[string]any{
				"name":  pkg,
				"state": "absent",
			},
			Reason:        fmt.Sprintf("Compensating package install by removing %s", pkg),
			SuggestedRisk: models.RiskMedium,
		}
	}

	return nil
}

// CompensationExecutor is an interface for dispatching compensating steps to agents
type CompensationExecutor interface {
	ExecuteCompensatingStep(ctx context.Context, agentID string, step *models.AIStepPlan) (*models.TaskStep, error)
	VerifyCompensatingStep(ctx context.Context, agentID string, strategy *models.VerificationStrategy) (bool, string, error)
}

// RollbackReport captures the detailed outcome of a Saga LIFO rollback
type RollbackReport struct {
	OverallStatus       string               `json:"overall_status"`
	CompensatedSteps    []string             `json:"compensated_steps"`
	FailedCompensations []string             `json:"failed_compensations"`
	IrreversibleSteps   []string             `json:"irreversible_steps"`
	Details             []string             `json:"details"`
	CompletedAt         time.Time            `json:"completed_at"`
}

// ExecuteSagaRollback performs LIFO compensation for all previously succeeded mutating steps
func ExecuteSagaRollback(
	ctx context.Context,
	executor CompensationExecutor,
	agentID string,
	succeededSteps []*models.TaskStep,
) *RollbackReport {
	report := &RollbackReport{
		OverallStatus:       RollbackSucceeded,
		CompensatedSteps:    make([]string, 0),
		FailedCompensations: make([]string, 0),
		IrreversibleSteps:   make([]string, 0),
		Details:             make([]string, 0),
		CompletedAt:         time.Now(),
	}

	// LIFO: Process in reverse order
	for i := len(succeededSteps) - 1; i >= 0; i-- {
		step := succeededSteps[i]

		// Skip read-only steps
		if isReadOnly(step.Action) || step.Status == "SKIPPED" || step.Status == "SKIPPED_ALREADY_DESIRED" {
			continue
		}

		rev := GetReversibility(step.Action)
		if rev == models.ReversibilityNone {
			report.IrreversibleSteps = append(report.IrreversibleSteps, step.ID)
			report.Details = append(report.Details, fmt.Sprintf("Step %s (%s) is IRREVERSIBLE", step.ID, step.Action))
			if step.RiskLevel == models.RiskHigh {
				report.OverallStatus = ManualInterventionRequired
			} else if report.OverallStatus == RollbackSucceeded {
				report.OverallStatus = RollbackPartial
			}
			continue
		}

		compPlan := step.CompensationAction
		if compPlan == nil {
			compPlan = DeriveCompensatingStep(step)
		}

		if compPlan == nil {
			report.IrreversibleSteps = append(report.IrreversibleSteps, step.ID)
			if report.OverallStatus == RollbackSucceeded {
				report.OverallStatus = RollbackPartial
			}
			continue
		}

		// Execute compensating action
		compResult, err := executor.ExecuteCompensatingStep(ctx, agentID, compPlan)
		if err != nil || (compResult != nil && compResult.Status == "FAILED") {
			report.FailedCompensations = append(report.FailedCompensations, step.ID)
			report.OverallStatus = RollbackFailed
			report.Details = append(report.Details, fmt.Sprintf("Compensation failed for step %s: %v", step.ID, err))
			continue
		}

		// Verify compensation if strategy exists
		if compPlan.VerificationStrategy != nil {
			passed, reason, _ := executor.VerifyCompensatingStep(ctx, agentID, compPlan.VerificationStrategy)
			if !passed {
				report.FailedCompensations = append(report.FailedCompensations, step.ID)
				report.OverallStatus = RollbackFailed
				report.Details = append(report.Details, fmt.Sprintf("Compensation verification failed for step %s: %s", step.ID, reason))
				continue
			}
		}

		report.CompensatedSteps = append(report.CompensatedSteps, step.ID)
		report.Details = append(report.Details, fmt.Sprintf("Successfully compensated step %s (%s)", step.ID, step.Action))
	}

	return report
}
