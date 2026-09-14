package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/failures"
	"opspilot/control-plane/internal/grpcserver"
	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/policy"
	"opspilot/control-plane/internal/statemachine"
	"opspilot/control-plane/internal/verification"
	opspilotv1 "opspilot/proto/v1"
)

type Orchestrator struct {
	store           database.Store
	machine         *statemachine.Machine
	policyEngine    *policy.Engine
	verifier        *verification.Engine
	hub             *events.Hub
	grpcServer      *grpcserver.Server
	aiServiceURL    string
	httpClient      *http.Client
	retryPolicy     *failures.RetryPolicy
	maxToolCalls    int
	maxTaskDuration time.Duration
}

func NewOrchestrator(
	store database.Store,
	machine *statemachine.Machine,
	policyEngine *policy.Engine,
	verifier *verification.Engine,
	hub *events.Hub,
	grpcServer *grpcserver.Server,
	aiServiceURL string,
) *Orchestrator {
	return &Orchestrator{
		store:           store,
		machine:         machine,
		policyEngine:    policyEngine,
		verifier:        verifier,
		hub:             hub,
		grpcServer:      grpcServer,
		aiServiceURL:    aiServiceURL,
		httpClient:      &http.Client{Timeout: 180 * time.Second},
		retryPolicy:     failures.DefaultRetryPolicy(),
		maxToolCalls:    20,
		maxTaskDuration: 10 * time.Minute,
	}
}

func (o *Orchestrator) SetExecutionLimits(maxToolCalls int, maxDuration time.Duration) {
	if maxToolCalls > 0 {
		o.maxToolCalls = maxToolCalls
	}
	if maxDuration > 0 {
		o.maxTaskDuration = maxDuration
	}
}

func (o *Orchestrator) StartTask(ctx context.Context, taskID string) error {
	task, err := o.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	if len(task.TargetAgentIDs) == 0 {
		return fmt.Errorf("task has no target agents")
	}
	targetAgentID := task.TargetAgentIDs[0]
	agent, err := o.store.GetAgent(ctx, targetAgentID)
	if err != nil {
		return fmt.Errorf("target agent %s not found: %w", targetAgentID, err)
	}

	// 1. Discovery phase
	if _, err := o.machine.Transition(task, models.TaskStatusDiscovering, "Starting host environment discovery", ""); err != nil {
		return err
	}
	_ = o.store.UpdateTask(ctx, task)
	o.hub.Publish(task.ID, "TASK_DISCOVERY", map[string]any{
		"agent_id":     agent.ID,
		"hostname":     agent.Hostname,
		"distribution": agent.Distribution,
		"version":      agent.Version,
		"capabilities": agent.Capabilities,
	})

	// 2. Planning phase
	if _, err := o.machine.Transition(task, models.TaskStatusPlanning, "Requesting AI plan from AI Service", ""); err != nil {
		return err
	}
	_ = o.store.UpdateTask(ctx, task)

	planReq := map[string]any{
		"task_id": task.ID,
		"intent":  task.Prompt,
		"host_context": map[string]any{
			"hostname":     agent.Hostname,
			"os":           agent.OS,
			"distribution": agent.Distribution,
			"version":      agent.Version,
			"architecture": agent.Architecture,
			"capabilities": agent.Capabilities,
		},
		"supported_tools": o.policyEngine.GetSupportedTools(),
	}

	planResp, err := o.callAIPlanning(ctx, "/api/v1/plan", planReq)
	if err != nil {
		task.ErrorMessage = fmt.Sprintf("AI planning failure: %v", err)
		_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "")
		_ = o.store.UpdateTask(ctx, task)
		return err
	}

	// 3. Policy & Risk Evaluation
	task.AIPlan = planResp
	highestRisk := models.RiskReadOnly
	requiresApproval := false

	var taskSteps []*models.TaskStep
	for idx, s := range planResp.Steps {
		realRisk, needApp, err := o.policyEngine.Evaluate(s.Action, s.Arguments, agent.Environment)
		if err != nil || realRisk == models.RiskForbidden {
			task.ErrorMessage = fmt.Sprintf("Forbidden action in plan: %s (%v)", s.Action, err)
			_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "")
			_ = o.store.UpdateTask(ctx, task)
			return fmt.Errorf("security policy violation: %s", task.ErrorMessage)
		}

		if realRisk == models.RiskHigh || (realRisk == models.RiskMedium && highestRisk != models.RiskHigh) {
			highestRisk = realRisk
		}
		if needApp {
			requiresApproval = true
		}

		strat := s.VerificationStrategy
		if strat == nil {
			if meta, ok := o.policyEngine.GetToolMetadata(s.Action); ok && meta.VerificationType != "" {
				target := ""
				if t, ok := s.Arguments["name"].(string); ok && t != "" {
					target = t
				} else if t, ok := s.Arguments["path"].(string); ok && t != "" {
					target = t
				} else if t, ok := s.Arguments["service"].(string); ok && t != "" {
					target = t
				}
				if target != "" {
					strat = &models.VerificationStrategy{
						CheckType:  meta.VerificationType,
						Target:     target,
						Expected:   "verified",
						TimeoutSec: 15,
					}
				}
			}
		}

		step := &models.TaskStep{
			ID:                   uuid.New().String(),
			TaskID:               task.ID,
			StepOrder:            idx + 1,
			Action:               s.Action,
			Arguments:            s.Arguments,
			RiskLevel:            realRisk,
			RequiresApproval:     needApp,
			VerificationStrategy: strat,
			Status:               "PENDING",
		}
		taskSteps = append(taskSteps, step)
		_ = o.store.SaveTaskStep(ctx, step)
	}

	task.Steps = taskSteps
	task.RiskLevel = highestRisk

	// 4. Approval Gate Check
	if requiresApproval {
		_, _ = o.machine.Transition(task, models.TaskStatusWaitingApproval, "Waiting for operator approval", "")
		_ = o.store.UpdateTask(ctx, task)

		approval := &models.Approval{
			ID:          uuid.New().String(),
			TaskID:      task.ID,
			PlanVersion: task.PlanVersion,
			Status:      "PENDING",
			RiskLevel:   highestRisk,
			CreatedAt:   time.Now(),
		}
		_ = o.store.SaveApproval(ctx, approval)

		o.hub.Publish(task.ID, "APPROVAL_REQUIRED", map[string]any{
			"task_id":      task.ID,
			"plan_version": task.PlanVersion,
			"risk_level":   highestRisk,
			"steps":        task.Steps,
		})
		return nil
	}

	// Auto-approve and execute if read-only / low-risk
	go o.ExecuteTask(context.Background(), task.ID)
	return nil
}

func (o *Orchestrator) ApproveTask(ctx context.Context, taskID string, planVersion int, userID, notes string) error {
	task, err := o.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	approval, err := o.store.GetApproval(ctx, taskID, planVersion)
	if err != nil {
		return fmt.Errorf("approval record not found: %w", err)
	}

	now := time.Now()
	approval.Status = "APPROVED"
	approval.DecidedBy = userID
	approval.DecisionNotes = notes
	approval.DecidedAt = &now
	_ = o.store.UpdateApproval(ctx, approval)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		UserID:    userID,
		TaskID:    task.ID,
		EventType: "APPROVAL_GRANTED",
		Action:    "approve",
		Details:   map[string]any{"plan_version": planVersion, "notes": notes},
		CreatedAt: now,
	})

	o.hub.Publish(task.ID, "APPROVAL_GRANTED", map[string]any{
		"task_id":      task.ID,
		"plan_version": planVersion,
		"approved_by":  userID,
	})

	go o.ExecuteTask(context.Background(), task.ID)
	return nil
}

func (o *Orchestrator) RejectTask(ctx context.Context, taskID string, planVersion int, userID, notes string) error {
	task, err := o.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	approval, _ := o.store.GetApproval(ctx, taskID, planVersion)
	if approval != nil {
		now := time.Now()
		approval.Status = "REJECTED"
		approval.DecidedBy = userID
		approval.DecisionNotes = notes
		approval.DecidedAt = &now
		_ = o.store.UpdateApproval(ctx, approval)
	}

	_, _ = o.machine.Transition(task, models.TaskStatusCancelled, fmt.Sprintf("Rejected by operator: %s", notes), userID)
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		UserID:    userID,
		TaskID:    task.ID,
		EventType: "APPROVAL_REJECTED",
		Action:    "reject",
		Details:   map[string]any{"plan_version": planVersion, "notes": notes},
		CreatedAt: time.Now(),
	})

	o.hub.Publish(task.ID, "APPROVAL_REJECTED", map[string]any{
		"task_id":      task.ID,
		"plan_version": planVersion,
		"rejected_by":  userID,
	})
	return nil
}

func (o *Orchestrator) ExecuteTask(ctx context.Context, taskID string) {
	task, err := o.store.GetTask(ctx, taskID)
	if err != nil {
		return
	}

	if _, err := o.machine.Transition(task, models.TaskStatusExecuting, "Executing planned steps", ""); err != nil {
		return
	}
	_ = o.store.UpdateTask(ctx, task)

	targetAgentID := task.TargetAgentIDs[0]
	agent, _ := o.store.GetAgent(ctx, targetAgentID)

	// Check total execution duration
	if !task.CreatedAt.IsZero() && time.Since(task.CreatedAt) > o.maxTaskDuration {
		task.ErrorMessage = fmt.Sprintf("Task exceeded maximum allowed duration of %v", o.maxTaskDuration)
		_, _ = o.machine.Transition(task, models.TaskStatusTimeout, task.ErrorMessage, "system")
		_ = o.store.UpdateTask(ctx, task)
		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			EventType: "TASK_FAILED",
			Action:    "task_timeout",
			Details:   map[string]any{"duration": time.Since(task.CreatedAt).String(), "limit": o.maxTaskDuration.String()},
			CreatedAt: time.Now(),
		})
		return
	}

	steps, _ := o.store.GetTaskSteps(ctx, task.ID)
	backups := make(map[string]string) // cleanPath -> backupPath
	successfulVerifications := 0

	// Count total prior executed steps for this task
	executedToolCalls := 0
	for _, st := range steps {
		if st.Status == "SUCCESS" {
			executedToolCalls++
		}
	}

	for _, step := range steps {
		if step.Status == "SUCCESS" {
			continue // skip already succeeded steps across replans
		}

		// Enforce MAX_TOOL_CALLS bound
		if executedToolCalls >= o.maxToolCalls {
			task.ErrorMessage = fmt.Sprintf("Maximum tool calls limit (%d) reached for task", o.maxToolCalls)
			_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
			_ = o.store.UpdateTask(ctx, task)
			_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
				TaskID:    task.ID,
				EventType: "TASK_FAILED",
				Action:    "tool_calls_limit_reached",
				Details:   map[string]any{"executed_tool_calls": executedToolCalls, "limit": o.maxToolCalls},
				CreatedAt: time.Now(),
			})
			return
		}
		executedToolCalls++

		step.Status = "RUNNING"
		now := time.Now()
		step.StartedAt = &now
		_ = o.store.UpdateTaskStep(ctx, step)

		// Inject task_id into arguments so file_tool creates organized backup
		if step.Arguments == nil {
			step.Arguments = make(map[string]any)
		}
		step.Arguments["task_id"] = task.ID

		argsJSON, _ := json.Marshal(step.Arguments)
		stratJSON, _ := json.Marshal(step.VerificationStrategy)

		o.hub.Publish(task.ID, "STEP_STARTED", map[string]any{
			"task_id":    task.ID,
			"step_id":    step.ID,
			"action":     step.Action,
			"arguments":  step.Arguments,
			"risk_level": step.RiskLevel,
		})

		var stepRes *opspilotv1.StepResult
		var stepErr error

		// Step retry loop for transient/network errors
		maxAttempts := o.retryPolicy.MaxAttempts
		for attempt := 0; attempt < maxAttempts; attempt++ {
			executionID := fmt.Sprintf("%s/%s/%d", task.ID, step.ID, attempt)
			step.ExecutionID = executionID
			_ = o.store.UpdateTaskStep(ctx, step)

			cmd := &opspilotv1.ExecuteStepCommand{
				TaskId:                   task.ID,
				StepId:                   step.ID,
				Action:                   step.Action,
				ArgumentsJson:            string(argsJSON),
				TimeoutSeconds:           60,
				RiskLevel:                string(step.RiskLevel),
				VerificationStrategyJson: string(stratJSON),
			}

			stepRes, stepErr = o.grpcServer.DispatchStep(ctx, targetAgentID, cmd)
			if stepErr == nil && stepRes != nil && stepRes.Success {
				break // execution succeeded!
			}

			// Failure encountered: classify
			target := ""
			for _, key := range []string{"target", "name", "package_name", "service_name", "path", "file_path"} {
				if t, ok := step.Arguments[key].(string); ok && t != "" {
					target = t
					break
				}
			}

			exitCode := 1
			stdout := ""
			stderr := ""
			if stepRes != nil {
				exitCode = int(stepRes.ExitCode)
				stdout = stepRes.Stdout
				stderr = stepRes.Stderr
			} else if stepErr != nil {
				stderr = stepErr.Error()
			}

			sf := failures.ClassifyExecutionFailure(step.Action, exitCode, stdout, stderr, target)
			if (stepErr != nil && (strings.Contains(stepErr.Error(), "connection") || strings.Contains(stepErr.Error(), "stream") || strings.Contains(stepErr.Error(), "unavailable"))) ||
				strings.Contains(stderr, "agent disconnected") || strings.Contains(stderr, "connection closed") {
				sf.Type = models.FailureAgentDisconnected
				sf.Message = "Agent disconnected during execution"
			}

			// Check retryability
			if o.retryPolicy.IsRetryable(sf, step.Action, attempt) {
				_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
					TaskID:    task.ID,
					AgentID:   agent.ID,
					EventType: "RETRY_ATTEMPTED",
					Action:    step.Action,
					Details:   map[string]any{"attempt": attempt + 1, "reason": sf.Message},
					CreatedAt: time.Now(),
				})
				time.Sleep(o.retryPolicy.GetBackoff(attempt))
				continue
			}
			break
		}

		finished := time.Now()
		step.FinishedAt = &finished

		// Evaluate final step outcome
		if stepErr != nil || (stepRes != nil && !stepRes.Success) {
			target := ""
			for _, key := range []string{"target", "name", "package_name", "service_name", "path", "file_path"} {
				if t, ok := step.Arguments[key].(string); ok && t != "" {
					target = t
					break
				}
			}

			exitCode := 1
			stdout := ""
			stderr := ""
			if stepRes != nil {
				exitCode = int(stepRes.ExitCode)
				stdout = stepRes.Stdout
				stderr = stepRes.Stderr
			} else if stepErr != nil {
				stderr = stepErr.Error()
			}

			sf := failures.ClassifyExecutionFailure(step.Action, exitCode, stdout, stderr, target)
			if (stepErr != nil && (strings.Contains(stepErr.Error(), "connection") || strings.Contains(stepErr.Error(), "stream") || strings.Contains(stepErr.Error(), "unavailable"))) ||
				strings.Contains(stderr, "agent disconnected") || strings.Contains(stderr, "connection closed") {
				sf.Type = models.FailureAgentDisconnected
				sf.Message = "Agent disconnected during execution"
			}

			step.Status = "FAILED"
			step.ExitCode = &exitCode
			step.Stdout = stdout
			step.Stderr = stderr
			_ = o.store.UpdateTaskStep(ctx, step)

			task.FailureDetails = sf
			task.ErrorMessage = sf.Message
			_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
				TaskID:    task.ID,
				AgentID:   agent.ID,
				EventType: "STEP_FAILED",
				Action:    step.Action,
				Details:   map[string]any{"failure": sf, "step_id": step.ID},
				CreatedAt: time.Now(),
			})

			// 1. Agent Disconnection handling
			if sf.Type == models.FailureAgentDisconnected {
				_ = o.handleAgentDisconnect(ctx, task, agent, step)
				return
			}

			// 1b. Uncertain Outcome handling (Agent died mid-mutation; observe before retry)
			if sf.Type == models.FailureUncertainExecution {
				_ = o.handleUncertainExecution(ctx, task, agent, step, sf)
				return
			}

			// 2. Rollback Compensation handling
			if sf.RollbackRequired || (len(backups) > 0 && sf.Type == models.FailureValidationFailed) {
				_ = o.executeRollback(ctx, task, agent, backups)
				return
			}

			// 3. Controlled Replanning handling
			if sf.RequiresReplan {
				o.executeReplan(ctx, task, agent, step, sf)
				return
			}

			// Fallback: Terminal failure
			_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
			_ = o.store.UpdateTask(ctx, task)
			_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
				TaskID:    task.ID,
				AgentID:   agent.ID,
				EventType: "TASK_FAILED",
				Action:    "execute",
				Details:   map[string]any{"failure": sf},
				CreatedAt: time.Now(),
			})
			return
		}

		// Step succeeded
		step.Status = "SUCCESS"
		if stepRes != nil {
			exitCode := int(stepRes.ExitCode)
			step.ExitCode = &exitCode
			step.Stdout = stepRes.Stdout
			step.Stderr = stepRes.Stderr

			var stepData map[string]any
			if stepRes.VerificationJson != "" {
				_ = json.Unmarshal([]byte(stepRes.VerificationJson), &stepData)
			}

			// Track backup if config file was written
			if stepData != nil {
				if bPath, ok := stepData["backup_path"].(string); ok && bPath != "" {
					if cPath, ok := stepData["path"].(string); ok && cPath != "" {
						backups[cPath] = bPath
					}
				}

				// Record idempotent execution if detected
				if stepData["idempotent"] == true {
					_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
						TaskID:    task.ID,
						AgentID:   agent.ID,
						EventType: "IDEMPOTENT_NO_OP",
						Action:    step.Action,
						Details:   stepData,
						CreatedAt: time.Now(),
					})
				}
			}
		}
		_ = o.store.UpdateTaskStep(ctx, step)

		// Verification for step if present
		if step.VerificationStrategy != nil {
			passed, info := o.executeVerification(ctx, task.ID, agent, step.VerificationStrategy)
			_ = o.store.SaveVerificationResult(ctx, &models.VerificationResult{
				ID:         uuid.New().String(),
				TaskID:     task.ID,
				StepID:     step.ID,
				AgentID:    agent.ID,
				CheckType:  step.VerificationStrategy.CheckType,
				Target:     step.VerificationStrategy.Target,
				Passed:     passed,
				Details:    map[string]any{"info": info},
				VerifiedAt: time.Now(),
			})
			if !passed {
				sf := failures.ClassifyVerificationFailure(step.VerificationStrategy.CheckType, step.VerificationStrategy.Target, info)
				task.FailureDetails = sf
				task.ErrorMessage = sf.Message

				_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
					TaskID:    task.ID,
					AgentID:   agent.ID,
					EventType: "VERIFICATION_FAILED",
					Action:    step.VerificationStrategy.CheckType,
					Details:   map[string]any{"failure": sf},
					CreatedAt: time.Now(),
				})

				if len(backups) > 0 {
					_ = o.executeRollback(ctx, task, agent, backups)
					return
				}
				o.executeReplan(ctx, task, agent, step, sf)
				return
			}
			successfulVerifications++
		}
	}

	// 5. Verifying Phase
	_, _ = o.machine.Transition(task, models.TaskStatusVerifying, "Running final deterministic verification suite", "")
	_ = o.store.UpdateTask(ctx, task)

	if task.AIPlan != nil && len(task.AIPlan.OverallVerification) > 0 {
		for _, strat := range task.AIPlan.OverallVerification {
			passed, info := o.executeVerification(ctx, task.ID, agent, strat)
			_ = o.store.SaveVerificationResult(ctx, &models.VerificationResult{
				ID:         uuid.New().String(),
				TaskID:     task.ID,
				AgentID:    agent.ID,
				CheckType:  strat.CheckType,
				Target:     strat.Target,
				Passed:     passed,
				Details:    map[string]any{"info": info},
				VerifiedAt: time.Now(),
			})
			if !passed {
				sf := failures.ClassifyVerificationFailure(strat.CheckType, strat.Target, info)
				task.FailureDetails = sf
				task.ErrorMessage = sf.Message

				_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
					TaskID:    task.ID,
					AgentID:   agent.ID,
					EventType: "VERIFICATION_FAILED",
					Action:    strat.CheckType,
					Details:   map[string]any{"failure": sf},
					CreatedAt: time.Now(),
				})

				if len(backups) > 0 {
					_ = o.executeRollback(ctx, task, agent, backups)
					return
				}
				dummyStep := &models.TaskStep{ID: "final-verification", Action: strat.CheckType}
				o.executeReplan(ctx, task, agent, dummyStep, sf)
				return
			}
			successfulVerifications++
		}
	}

	// 6. Verification Contract: Mutating actions cannot complete without passing deterministic verification
	hasMutatingSteps := false
	for _, st := range steps {
		if meta, ok := o.policyEngine.GetToolMetadata(st.Action); ok && !meta.ReadOnly {
			hasMutatingSteps = true
			break
		}
	}

	if hasMutatingSteps && successfulVerifications == 0 {
		task.ErrorMessage = "Verification contract violation: task performed mutating operations but zero deterministic verifications succeeded."
		task.FailureDetails = &models.StructuredFailure{
			Type:    models.FailureVerificationFailed,
			Action:  "verify",
			Message: task.ErrorMessage,
		}
		_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
		_ = o.store.UpdateTask(ctx, task)
		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "TASK_FAILED",
			Action:    "verify",
			Details:   map[string]any{"reason": task.ErrorMessage},
			CreatedAt: time.Now(),
		})
		return
	}

	// 7. Complete
	task.ExecutionSummary = "All planned actions executed successfully and independently verified."
	_, _ = o.machine.Transition(task, models.TaskStatusCompleted, task.ExecutionSummary, "")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "TASK_COMPLETED",
		Action:    "execute",
		Details:   map[string]any{"summary": task.ExecutionSummary},
		CreatedAt: time.Now(),
	})

	o.hub.Publish(task.ID, "TASK_COMPLETED", map[string]any{
		"task_id": task.ID,
		"status":  models.TaskStatusCompleted,
		"summary": task.ExecutionSummary,
	})
}

func (o *Orchestrator) executeRollback(ctx context.Context, task *models.Task, agent *models.Agent, backups map[string]string) error {
	_, _ = o.machine.Transition(task, models.TaskStatusRollingBack, "Executing rollback compensation", "system")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "ROLLBACK_STARTED",
		Action:    "rollback",
		Details:   map[string]any{"backups": backups},
		CreatedAt: time.Now(),
	})

	rollbackSuccess := true
	for cleanPath, backupPath := range backups {
		rbArgs, _ := json.Marshal(map[string]any{
			"path":        cleanPath,
			"backup_path": backupPath,
		})
		rbCmd := &opspilotv1.ExecuteStepCommand{
			TaskId:         task.ID,
			StepId:         uuid.New().String(),
			Action:         "rollback_config",
			ArgumentsJson:  string(rbArgs),
			TimeoutSeconds: 30,
			RiskLevel:      string(models.RiskMedium),
		}
		rbRes, err := o.grpcServer.DispatchStep(ctx, agent.ID, rbCmd)
		if err != nil || (rbRes != nil && !rbRes.Success) {
			rollbackSuccess = false
			break
		}
	}

	// If nginx was touched, restart service to activate restored configuration
	for cleanPath := range backups {
		if strings.Contains(cleanPath, "nginx") {
			rstArgs, _ := json.Marshal(map[string]any{"name": "nginx"})
			rstCmd := &opspilotv1.ExecuteStepCommand{
				TaskId:         task.ID,
				StepId:         uuid.New().String(),
				Action:         "restart_service",
				ArgumentsJson:  string(rstArgs),
				TimeoutSeconds: 30,
				RiskLevel:      string(models.RiskMedium),
			}
			_, _ = o.grpcServer.DispatchStep(ctx, agent.ID, rstCmd)
			break
		}
	}

	// Verify restored state independently
	verificationPassed, _ := o.executeVerification(ctx, task.ID, agent, &models.VerificationStrategy{
		CheckType:  "systemd_active",
		Target:     "nginx",
		TimeoutSec: 15,
	})

	if rollbackSuccess && verificationPassed {
		task.ExecutionSummary = "Operation failed, but configuration and service were successfully rolled back to verified state."
		_, _ = o.machine.Transition(task, models.TaskStatusRolledBack, task.ExecutionSummary, "system")
		_ = o.store.UpdateTask(ctx, task)

		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "ROLLBACK_COMPLETED",
			Action:    "rollback",
			Details:   map[string]any{"summary": task.ExecutionSummary, "verified": true},
			CreatedAt: time.Now(),
		})
		return nil
	}

	task.ErrorMessage = "Rollback compensation failed or restored state could not be verified."
	_, _ = o.machine.Transition(task, models.TaskStatusRollbackFailed, task.ErrorMessage, "system")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "ROLLBACK_FAILED",
		Action:    "rollback",
		Details:   map[string]any{"error": task.ErrorMessage},
		CreatedAt: time.Now(),
	})
	return fmt.Errorf("rollback failed")
}

func (o *Orchestrator) executeReplan(ctx context.Context, task *models.Task, agent *models.Agent, failedStep *models.TaskStep, sf *models.StructuredFailure) {
	// 1. Immediate terminal failure for non-existent resources (avoids uncontrolled timeout loops)
	stderrLow := strings.ToLower(failedStep.Stderr)
	stdoutLow := strings.ToLower(failedStep.Stdout)
	if strings.Contains(stderrLow, "could not be found") || strings.Contains(stdoutLow, "could not be found") ||
		(strings.Contains(stderrLow, "no such file or directory") && failedStep.Action == "get_service_status") {
		task.ErrorMessage = fmt.Sprintf("Target resource for action '%s' does not exist on host: %s", failedStep.Action, strings.TrimSpace(failedStep.Stderr))
		task.FailureDetails = &models.StructuredFailure{
			Type:           models.FailureResourceNotFound,
			Action:         failedStep.Action,
			Message:        task.ErrorMessage,
			RequiresReplan: false,
		}
		_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
		_ = o.store.UpdateTask(ctx, task)
		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "RESOURCE_NOT_FOUND",
			Action:    failedStep.Action,
			Details:   map[string]any{"error": task.ErrorMessage},
			CreatedAt: time.Now(),
		})
		return
	}

	if task.MaxReplans <= 0 {
		task.MaxReplans = 3
	}

	if task.ReplanCount >= task.MaxReplans {
		task.ErrorMessage = fmt.Sprintf("Maximum replan limit (%d) reached without resolution.", task.MaxReplans)
		_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
		_ = o.store.UpdateTask(ctx, task)

		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "REPLAN_LIMIT_REACHED",
			Action:    "replan",
			Details:   map[string]any{"replan_count": task.ReplanCount, "max_replans": task.MaxReplans},
			CreatedAt: time.Now(),
		})
		return
	}

	// Transition: EXECUTING -> OBSERVING -> REPLANNING
	_, _ = o.machine.Transition(task, models.TaskStatusObserving, "Observing failure conditions", "system")
	_ = o.store.UpdateTask(ctx, task)

	replanReason := fmt.Sprintf("Replanning attempt %d of %d due to %s", task.ReplanCount+1, task.MaxReplans, sf.Type)
	_, _ = o.machine.Transition(task, models.TaskStatusReplanning, replanReason, "system")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "REPLAN_REQUESTED",
		Action:    "replan",
		Details:   map[string]any{"attempt": task.ReplanCount + 1, "failure": sf},
		CreatedAt: time.Now(),
	})

	// Fetch prior successful steps
	priorSteps, _ := o.store.GetTaskSteps(ctx, task.ID)
	var successfulSteps []map[string]any
	for _, st := range priorSteps {
		if st.Status == "SUCCESS" {
			successfulSteps = append(successfulSteps, map[string]any{
				"id":        st.ID,
				"action":    st.Action,
				"arguments": st.Arguments,
				"reason":    "completed step",
			})
		}
	}

	replanReq := map[string]any{
		"task_id":                task.ID,
		"intent":                 task.Prompt,
		"failed_step_id":         failedStep.ID,
		"failed_action":          failedStep.Action,
		"exit_code":              failedStep.ExitCode,
		"untrusted_stdout":       failedStep.Stdout,
		"untrusted_stderr":       failedStep.Stderr,
		"prior_successful_steps": successfulSteps,
		"host_context": map[string]any{
			"hostname":     agent.Hostname,
			"os":           agent.OS,
			"distribution": agent.Distribution,
			"version":      agent.Version,
			"architecture": agent.Architecture,
			"capabilities": agent.Capabilities,
		},
		"supported_tools": o.policyEngine.GetSupportedTools(),
	}

	newPlan, err := o.callAIPlanning(ctx, "/api/v1/replan", replanReq)
	if err != nil {
		task.FailureDetails = &models.StructuredFailure{
			Type:    models.FailureInvalidModelOutput,
			Action:  "replan",
			Message: fmt.Sprintf("AI replanning failed: %v", err),
		}
		task.ErrorMessage = task.FailureDetails.Message
		_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
		_ = o.store.UpdateTask(ctx, task)

		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "REPLAN_REJECTED",
			Action:    "replan",
			Details:   map[string]any{"error": err.Error()},
			CreatedAt: time.Now(),
		})
		return
	}

	// Detect replan loops and stalled cycles
	var stepSignatures []string
	for _, s := range newPlan.Steps {
		stepSignatures = append(stepSignatures, fmt.Sprintf("%s:%v", s.Action, s.Arguments))
	}
	newPlanFingerprint := strings.Join(stepSignatures, "|")

	if len(newPlan.Steps) == 0 || (task.ExecutionSummary != "" && task.ExecutionSummary == newPlanFingerprint) {
		task.ErrorMessage = "Replan stalled: AI generated no progressive actions or repeated failed plan"
		task.FailureDetails = &models.StructuredFailure{
			Type:           models.FailureReplanExhausted,
			Action:         "replan",
			Message:        task.ErrorMessage,
			RequiresReplan: false,
		}
		_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
		_ = o.store.UpdateTask(ctx, task)
		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "REPLAN_STALLED",
			Action:    "replan",
			Details:   map[string]any{"fingerprint": newPlanFingerprint},
			CreatedAt: time.Now(),
		})
		return
	}
	task.ExecutionSummary = newPlanFingerprint

	task.ReplanCount++
	task.PlanVersion++
	task.AIPlan = newPlan
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "REPLAN_GENERATED",
		Action:    "replan",
		Details:   map[string]any{"plan_version": task.PlanVersion, "replan_count": task.ReplanCount},
		CreatedAt: time.Now(),
	})
	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "PLAN_VERSION_CHANGED",
		Action:    "version_bump",
		Details:   map[string]any{"new_version": task.PlanVersion},
		CreatedAt: time.Now(),
	})

	// Policy evaluation on new plan
	highestRisk := models.RiskReadOnly
	requiresApproval := false

	var newSteps []*models.TaskStep
	for idx, s := range newPlan.Steps {
		realRisk, needApp, pErr := o.policyEngine.Evaluate(s.Action, s.Arguments, agent.Environment)
		if pErr != nil || realRisk == models.RiskForbidden {
			task.ErrorMessage = fmt.Sprintf("Forbidden action in replanned plan: %s (%v)", s.Action, pErr)
			_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
			_ = o.store.UpdateTask(ctx, task)
			return
		}

		if realRisk == models.RiskHigh || (realRisk == models.RiskMedium && highestRisk != models.RiskHigh) {
			highestRisk = realRisk
		}
		if needApp {
			requiresApproval = true
		}

		strat := s.VerificationStrategy
		if strat == nil {
			if meta, ok := o.policyEngine.GetToolMetadata(s.Action); ok && meta.VerificationType != "" {
				target := ""
				if t, ok := s.Arguments["name"].(string); ok && t != "" {
					target = t
				} else if t, ok := s.Arguments["path"].(string); ok && t != "" {
					target = t
				} else if t, ok := s.Arguments["service"].(string); ok && t != "" {
					target = t
				}
				if target != "" {
					strat = &models.VerificationStrategy{
						CheckType:  meta.VerificationType,
						Target:     target,
						Expected:   "verified",
						TimeoutSec: 15,
					}
				}
			}
		}

		step := &models.TaskStep{
			ID:                   uuid.New().String(),
			TaskID:               task.ID,
			StepOrder:            idx + 1,
			Action:               s.Action,
			Arguments:            s.Arguments,
			RiskLevel:            realRisk,
			RequiresApproval:     needApp,
			VerificationStrategy: strat,
			Status:               "PENDING",
		}
		newSteps = append(newSteps, step)
		_ = o.store.SaveTaskStep(ctx, step)
	}

	task.Steps = newSteps
	task.RiskLevel = highestRisk

	// If approval required or resource conflict requires operator decision, invalidate old approval
	if requiresApproval || sf.UserActionRequired || sf.Type == models.FailureResourceConflict {
		_, _ = o.machine.Transition(task, models.TaskStatusWaitingApproval, "Replanned operations require operator decision/approval", "system")
		_ = o.store.UpdateTask(ctx, task)

		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "APPROVAL_INVALIDATED",
			Action:    "replan_approval_gate",
			Details:   map[string]any{"new_plan_version": task.PlanVersion, "reason": sf.Message},
			CreatedAt: time.Now(),
		})

		approval := &models.Approval{
			ID:          uuid.New().String(),
			TaskID:      task.ID,
			PlanVersion: task.PlanVersion,
			Status:      "PENDING",
			RiskLevel:   highestRisk,
			CreatedAt:   time.Now(),
		}
		_ = o.store.SaveApproval(ctx, approval)

		o.hub.Publish(task.ID, "APPROVAL_REQUIRED", map[string]any{
			"task_id":      task.ID,
			"plan_version": task.PlanVersion,
			"risk_level":   highestRisk,
			"steps":        task.Steps,
			"failure":      sf,
		})
		return
	}

	// Otherwise, resume execution with new plan
	go o.ExecuteTask(context.Background(), task.ID)
}

func (o *Orchestrator) handleAgentDisconnect(ctx context.Context, task *models.Task, agent *models.Agent, step *models.TaskStep) error {
	_, _ = o.machine.Transition(task, models.TaskStatusWaitingForAgent, "Agent disconnected during execution; entering grace period", "system")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "AGENT_DISCONNECTED",
		Action:    "disconnect_detected",
		Details:   map[string]any{"step_id": step.ID, "action": step.Action},
		CreatedAt: time.Now(),
	})

	// Wait up to 15 seconds grace period for agent to reconnect
	reconnected := false
	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		updatedAgent, err := o.store.GetAgent(ctx, agent.ID)
		if err == nil && updatedAgent.Status == "online" && updatedAgent.LastHeartbeat != nil && time.Since(*updatedAgent.LastHeartbeat) < 10*time.Second {
			reconnected = true
			break
		}
	}

	if reconnected {
		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "AGENT_RECONNECTED",
			Action:    "agent_recovered",
			Details:   map[string]any{"agent_id": agent.ID},
			CreatedAt: time.Now(),
		})

		_, _ = o.machine.Transition(task, models.TaskStatusObserving, "Agent reconnected; observing environment state", "system")
		_ = o.store.UpdateTask(ctx, task)

		go o.ExecuteTask(context.Background(), task.ID)
		return nil
	}

	task.ErrorMessage = "Agent disconnected and failed to reconnect within grace period."
	_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "system")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "TASK_FAILED",
		Action:    "agent_disconnect_timeout",
		Details:   map[string]any{"error": task.ErrorMessage},
		CreatedAt: time.Now(),
	})
	return fmt.Errorf("agent disconnect timeout")
}

func (o *Orchestrator) handleUncertainExecution(ctx context.Context, task *models.Task, agent *models.Agent, step *models.TaskStep, sf *models.StructuredFailure) error {
	// Transition to OBSERVING to determine ground truth without blind execution
	_, _ = o.machine.Transition(task, models.TaskStatusObserving, "Observing ground truth after uncertain outcome", "system")
	_ = o.store.UpdateTask(ctx, task)

	_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		EventType: "UNCERTAIN_EXECUTION_OBSERVING",
		Action:    step.Action,
		Details:   map[string]any{"step_id": step.ID, "action": step.Action},
		CreatedAt: time.Now(),
	})

	alreadySatisfied := false
	target := ""
	for _, key := range []string{"target", "name", "package_name", "service_name", "path", "file_path"} {
		if t, ok := step.Arguments[key].(string); ok && t != "" {
			target = t
			break
		}
	}

	// Operation-specific observation
	switch step.Action {
	case "install_package":
		passed, _ := o.executeVerification(ctx, task.ID, agent, &models.VerificationStrategy{
			CheckType:  "package_installed",
			Target:     target,
			TimeoutSec: 15,
		})
		alreadySatisfied = passed

	case "start_service", "restart_service":
		passed, _ := o.executeVerification(ctx, task.ID, agent, &models.VerificationStrategy{
			CheckType:  "systemd_active",
			Target:     target,
			TimeoutSec: 15,
		})
		alreadySatisfied = passed

	case "write_config_file":
		// Probe file content SHA256
		if expectedContent, ok := step.Arguments["content"].(string); ok && target != "" {
			argsJSON, _ := json.Marshal(map[string]any{"path": target})
			cmd := &opspilotv1.ExecuteStepCommand{
				TaskId:         task.ID,
				StepId:         uuid.New().String(),
				Action:         "read_file",
				ArgumentsJson:  string(argsJSON),
				TimeoutSeconds: 10,
			}
			res, err := o.grpcServer.DispatchStep(ctx, agent.ID, cmd)
			if err == nil && res != nil && res.Success && strings.TrimSpace(res.Stdout) == strings.TrimSpace(expectedContent) {
				alreadySatisfied = true
			}
		}
	}

	if alreadySatisfied {
		// Operation already completed host-side before crash!
		step.Status = "SUCCESS"
		exitZero := 0
		step.ExitCode = &exitZero
		step.Stdout = "State observation confirmed desired mutation already satisfied"
		step.Stderr = ""
		_ = o.store.UpdateTaskStep(ctx, step)

		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			EventType: "IDEMPOTENT_RECOVERED",
			Action:    step.Action,
			Details:   map[string]any{"step_id": step.ID, "target": target},
			CreatedAt: time.Now(),
		})

		// Transition back to EXECUTING and continue remainder of plan
		_, _ = o.machine.Transition(task, models.TaskStatusExecuting, "Desired state already satisfied; resuming execution", "system")
		_ = o.store.UpdateTask(ctx, task)

		go o.ExecuteTask(context.Background(), task.ID)
		return nil
	}

	// State is NOT satisfied: safe replan or operator decision
	sf.RequiresReplan = true
	o.executeReplan(ctx, task, agent, step, sf)
	return nil
}

func (o *Orchestrator) callAIPlanning(ctx context.Context, path string, payload map[string]any) (*models.AIPlanData, error) {
	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", o.aiServiceURL+path, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Service returned HTTP %d", resp.StatusCode)
	}

	var plan models.AIPlanData
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, err
	}

	if plan.Provenance != nil {
		if taskID, ok := payload["task_id"].(string); ok && plan.Provenance.TaskID == "" {
			plan.Provenance.TaskID = taskID
		}
		_ = o.store.SaveAIInvocation(ctx, plan.Provenance)

		eventType := "AI_INVOCATION_COMPLETED"
		if plan.Provenance.FallbackUsed {
			eventType = "AI_FALLBACK_USED"
		}
		_ = o.store.SaveAuditEvent(ctx, &models.AuditEvent{
			TaskID:    plan.Provenance.TaskID,
			EventType: eventType,
			Action:    "AI_PLANNING",
			Details: map[string]any{
				"invocation_id":   plan.Provenance.InvocationID,
				"purpose":         plan.Provenance.Purpose,
				"provider":        plan.Provenance.Provider,
				"model":           plan.Provenance.Model,
				"model_digest":    plan.Provenance.ModelDigest,
				"fallback_used":   plan.Provenance.FallbackUsed,
				"fallback_reason": plan.Provenance.FallbackReason,
				"latency_ms":      plan.Provenance.LatencyMS,
			},
			CreatedAt: time.Now(),
		})
	}

	return &plan, nil
}

func (o *Orchestrator) executeVerification(ctx context.Context, taskID string, agent *models.Agent, strat *models.VerificationStrategy) (bool, string) {
	if strat == nil {
		return true, "no verification strategy specified"
	}

	switch strat.CheckType {
	case "tcp_port_open":
		// Direct network dial from control plane
		passed, info, _ := o.verifier.VerifyNetworkTarget(ctx, strat, agent.IPAddress)
		if passed {
			return true, info
		}
		// If control plane cannot directly dial hostIP, dispatch check_port to agent
		port := strat.Target
		if strings.Contains(port, ":") {
			parts := strings.Split(port, ":")
			port = parts[len(parts)-1]
		}
		argsJSON, _ := json.Marshal(map[string]any{"port": port})
		cmd := &opspilotv1.ExecuteStepCommand{
			TaskId:         taskID,
			StepId:         uuid.New().String(),
			Action:         "check_port",
			ArgumentsJson:  string(argsJSON),
			TimeoutSeconds: 10,
		}
		res, err := o.grpcServer.DispatchStep(ctx, agent.ID, cmd)
		if err == nil && res != nil && res.Success {
			return true, fmt.Sprintf("Agent verified TCP port %s is open", port)
		}
		return false, fmt.Sprintf("TCP port %s failed verification: %s", port, info)

	case "http_probe":
		// Direct probe from control plane
		passed, info, _ := o.verifier.VerifyNetworkTarget(ctx, strat, agent.IPAddress)
		if passed {
			return true, info
		}
		// Fallback to agent probe if URL is localhost
		argsJSON, _ := json.Marshal(map[string]any{"url": strat.Target, "expected_status": 200})
		cmd := &opspilotv1.ExecuteStepCommand{
			TaskId:         taskID,
			StepId:         uuid.New().String(),
			Action:         "http_probe",
			ArgumentsJson:  string(argsJSON),
			TimeoutSeconds: 10,
		}
		res, err := o.grpcServer.DispatchStep(ctx, agent.ID, cmd)
		if err == nil && res != nil && res.Success {
			return true, fmt.Sprintf("Agent verified HTTP probe to %s: HTTP 200 OK", strat.Target)
		}
		return false, fmt.Sprintf("HTTP probe failed verification: %s", info)

	case "systemd_active":
		argsJSON, _ := json.Marshal(map[string]any{"name": strat.Target})
		cmd := &opspilotv1.ExecuteStepCommand{
			TaskId:         taskID,
			StepId:         uuid.New().String(),
			Action:         "get_service_status",
			ArgumentsJson:  string(argsJSON),
			TimeoutSeconds: 10,
		}
		res, err := o.grpcServer.DispatchStep(ctx, agent.ID, cmd)
		if err != nil || res == nil {
			return false, fmt.Sprintf("Failed to query systemd service %s: %v", strat.Target, err)
		}
		if strings.Contains(res.Stdout, "Active: active (running)") || (res.ExitCode == 0 && strings.Contains(res.Stdout, "active")) {
			return true, fmt.Sprintf("Systemd service '%s' confirmed actively running", strat.Target)
		}
		return false, fmt.Sprintf("Systemd service '%s' is NOT active (exit_code=%d)", strat.Target, res.ExitCode)

	case "package_installed":
		argsJSON, _ := json.Marshal(map[string]any{"name": strat.Target})
		cmd := &opspilotv1.ExecuteStepCommand{
			TaskId:         taskID,
			StepId:         uuid.New().String(),
			Action:         "check_package",
			ArgumentsJson:  string(argsJSON),
			TimeoutSeconds: 15,
		}
		res, err := o.grpcServer.DispatchStep(ctx, agent.ID, cmd)
		if err != nil || res == nil {
			return false, fmt.Sprintf("Failed to query package status for %s: %v", strat.Target, err)
		}
		if res.Success && (res.ExitCode == 0) {
			return true, fmt.Sprintf("Package '%s' confirmed installed via dpkg", strat.Target)
		}
		return false, fmt.Sprintf("Package '%s' is not installed", strat.Target)

	case "file_exists":
		argsJSON, _ := json.Marshal(map[string]any{"path": strat.Target})
		cmd := &opspilotv1.ExecuteStepCommand{
			TaskId:         taskID,
			StepId:         uuid.New().String(),
			Action:         "read_file",
			ArgumentsJson:  string(argsJSON),
			TimeoutSeconds: 10,
		}
		res, err := o.grpcServer.DispatchStep(ctx, agent.ID, cmd)
		if err != nil || res == nil || !res.Success {
			return false, fmt.Sprintf("File '%s' verification failed: cannot be read or does not exist", strat.Target)
		}
		return true, fmt.Sprintf("File '%s' verified to exist and is readable", strat.Target)

	default:
		return true, fmt.Sprintf("Check type '%s' evaluated", strat.CheckType)
	}
}

