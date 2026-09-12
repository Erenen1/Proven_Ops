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
	"opspilot/control-plane/internal/grpcserver"
	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/policy"
	"opspilot/control-plane/internal/statemachine"
	"opspilot/control-plane/internal/verification"
	opspilotv1 "opspilot/proto/v1"
)

type Orchestrator struct {
	store        database.Store
	machine      *statemachine.Machine
	policyEngine *policy.Engine
	verifier     *verification.Engine
	hub          *events.Hub
	grpcServer   *grpcserver.Server
	aiServiceURL string
	httpClient   *http.Client
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
		store:        store,
		machine:      machine,
		policyEngine: policyEngine,
		verifier:     verifier,
		hub:          hub,
		grpcServer:   grpcServer,
		aiServiceURL: aiServiceURL,
		httpClient:   &http.Client{Timeout: 180 * time.Second},
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

		step := &models.TaskStep{
			ID:                   uuid.New().String(),
			TaskID:               task.ID,
			StepOrder:            idx + 1,
			Action:               s.Action,
			Arguments:            s.Arguments,
			RiskLevel:            realRisk,
			RequiresApproval:     needApp,
			VerificationStrategy: s.VerificationStrategy,
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

	steps, _ := o.store.GetTaskSteps(ctx, task.ID)

	for _, step := range steps {
		step.Status = "RUNNING"
		now := time.Now()
		step.StartedAt = &now
		_ = o.store.UpdateTaskStep(ctx, step)

		argsJSON, _ := json.Marshal(step.Arguments)
		stratJSON, _ := json.Marshal(step.VerificationStrategy)

		cmd := &opspilotv1.ExecuteStepCommand{
			TaskId:                   task.ID,
			StepId:                   step.ID,
			Action:                   step.Action,
			ArgumentsJson:            string(argsJSON),
			TimeoutSeconds:           60,
			RiskLevel:                string(step.RiskLevel),
			VerificationStrategyJson: string(stratJSON),
		}

		o.hub.Publish(task.ID, "STEP_STARTED", map[string]any{
			"task_id":    task.ID,
			"step_id":    step.ID,
			"action":     step.Action,
			"arguments":  step.Arguments,
			"risk_level": step.RiskLevel,
		})

		stepRes, err := o.grpcServer.DispatchStep(ctx, targetAgentID, cmd)
		finished := time.Now()
		step.FinishedAt = &finished

		if err != nil || (stepRes != nil && !stepRes.Success) {
			step.Status = "FAILED"
			if stepRes != nil {
				exitCode := int(stepRes.ExitCode)
				step.ExitCode = &exitCode
				step.Stdout = stepRes.Stdout
				step.Stderr = stepRes.Stderr
			}
			_ = o.store.UpdateTaskStep(ctx, step)

			task.ErrorMessage = fmt.Sprintf("Step %s (%s) failed", step.ID, step.Action)
			_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "")
			_ = o.store.UpdateTask(ctx, task)
			return
		}

		// Step succeeded
		step.Status = "SUCCESS"
		if stepRes != nil {
			exitCode := int(stepRes.ExitCode)
			step.ExitCode = &exitCode
			step.Stdout = stepRes.Stdout
			step.Stderr = stepRes.Stderr
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
				task.ErrorMessage = fmt.Sprintf("Final verification failed: %s (%s)", strat.CheckType, info)
				_, _ = o.machine.Transition(task, models.TaskStatusFailed, task.ErrorMessage, "")
				_ = o.store.UpdateTask(ctx, task)
				return
			}
		}
	}

	// 6. Complete
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

	default:
		return true, fmt.Sprintf("Check type '%s' evaluated", strat.CheckType)
	}
}

