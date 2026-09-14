package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"opspilot/control-plane/internal/models"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// -------------------------------------------------------------
// AGENTS
// -------------------------------------------------------------

func (s *PostgresStore) SaveAgent(ctx context.Context, agent *models.Agent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO agents (
			id, hostname, ip_address, os, distribution, version,
			architecture, status, environment, last_heartbeat_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			ip_address = EXCLUDED.ip_address,
			os = EXCLUDED.os,
			distribution = EXCLUDED.distribution,
			version = EXCLUDED.version,
			architecture = EXCLUDED.architecture,
			status = EXCLUDED.status,
			environment = EXCLUDED.environment,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at,
			updated_at = EXCLUDED.updated_at
	`
	now := time.Now()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}

	_, err = tx.Exec(ctx, query,
		agent.ID, agent.Hostname, agent.IPAddress, agent.OS, agent.Distribution,
		agent.Version, agent.Architecture, agent.Status, agent.Environment,
		agent.LastHeartbeat, agent.CreatedAt, now,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert agent: %w", err)
	}

	// Insert capabilities
	for _, cap := range agent.Capabilities {
		_, err = tx.Exec(ctx, `
			INSERT INTO agent_capabilities (agent_id, capability)
			VALUES ($1, $2)
			ON CONFLICT (agent_id, capability) DO NOTHING
		`, agent.ID, cap)
		if err != nil {
			return fmt.Errorf("failed to insert capability %s: %w", cap, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, hostname, ip_address, os, distribution, version,
		       architecture, status, environment, last_heartbeat_at, created_at
		FROM agents WHERE id = $1
	`, id)

	var a models.Agent
	var ip, env *string
	err := row.Scan(
		&a.ID, &a.Hostname, &ip, &a.OS, &a.Distribution, &a.Version,
		&a.Architecture, &a.Status, &env, &a.LastHeartbeat, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("agent not found: %s", id)
		}
		return nil, err
	}
	if ip != nil {
		a.IPAddress = *ip
	}
	if env != nil {
		a.Environment = *env
	}

	// Fetch capabilities
	caps, err := s.getAgentCapabilities(ctx, id)
	if err == nil {
		a.Capabilities = caps
	}

	return &a, nil
}

func (s *PostgresStore) ListAgents(ctx context.Context) ([]*models.Agent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, hostname, ip_address, os, distribution, version,
		       architecture, status, environment, last_heartbeat_at, created_at
		FROM agents ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*models.Agent
	for rows.Next() {
		var a models.Agent
		var ip, env *string
		if err := rows.Scan(
			&a.ID, &a.Hostname, &ip, &a.OS, &a.Distribution, &a.Version,
			&a.Architecture, &a.Status, &env, &a.LastHeartbeat, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		if ip != nil {
			a.IPAddress = *ip
		}
		if env != nil {
			a.Environment = *env
		}
		agents = append(agents, &a)
	}

	for _, a := range agents {
		if caps, err := s.getAgentCapabilities(ctx, a.ID); err == nil {
			a.Capabilities = caps
		}
	}

	return agents, nil
}

func (s *PostgresStore) getAgentCapabilities(ctx context.Context, agentID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT capability FROM agent_capabilities WHERE agent_id = $1", agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var caps []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err == nil {
			caps = append(caps, c)
		}
	}
	return caps, nil
}

func (s *PostgresStore) UpdateAgentStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE agents SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2
	`, status, id)
	return err
}

func (s *PostgresStore) SaveHeartbeat(ctx context.Context, agentID string, metrics *models.AgentMetrics) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE agents SET status = 'online', last_heartbeat_at = $1, updated_at = $1 WHERE id = $2
	`, now, agentID)
	if err != nil {
		return err
	}

	if metrics != nil {
		_, err = s.pool.Exec(ctx, `
			INSERT INTO agent_heartbeats (
				agent_id, cpu_usage_percent, memory_usage_bytes, memory_total_bytes,
				disk_usage_percent, load_avg_1m, active_tasks, recorded_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, agentID, metrics.CPUUsagePercent, metrics.MemoryUsageBytes, metrics.MemoryTotalBytes,
			metrics.DiskUsagePercent, metrics.LoadAvg1m, metrics.ActiveTasks, now)
	}
	return err
}

// -------------------------------------------------------------
// TASKS
// -------------------------------------------------------------

func (s *PostgresStore) SaveTask(ctx context.Context, task *models.Task) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var planJSON []byte
	if task.AIPlan != nil {
		planJSON, _ = json.Marshal(task.AIPlan)
	}
	var failureJSON []byte
	if task.FailureDetails != nil {
		failureJSON, _ = json.Marshal(task.FailureDetails)
	}

	maxReplans := task.MaxReplans
	if maxReplans <= 0 {
		maxReplans = 3
	}

	query := `
		INSERT INTO tasks (
			id, title, prompt, status, plan_version, idempotency_key, replan_count, max_replans,
			ai_plan, risk_level, error_message, execution_summary, failure_details,
			created_at, updated_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			prompt = EXCLUDED.prompt,
			status = EXCLUDED.status,
			plan_version = EXCLUDED.plan_version,
			idempotency_key = EXCLUDED.idempotency_key,
			replan_count = EXCLUDED.replan_count,
			max_replans = EXCLUDED.max_replans,
			ai_plan = EXCLUDED.ai_plan,
			risk_level = EXCLUDED.risk_level,
			error_message = EXCLUDED.error_message,
			execution_summary = EXCLUDED.execution_summary,
			failure_details = EXCLUDED.failure_details,
			updated_at = EXCLUDED.updated_at,
			completed_at = EXCLUDED.completed_at
	`

	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	_, err = tx.Exec(ctx, query,
		task.ID, task.Title, task.Prompt, string(task.Status), task.PlanVersion,
		task.IdempotencyKey, task.ReplanCount, maxReplans,
		planJSON, string(task.RiskLevel), task.ErrorMessage, task.ExecutionSummary, failureJSON,
		task.CreatedAt, task.UpdatedAt, task.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	// Insert task targets
	for _, agentID := range task.TargetAgentIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO task_targets (task_id, agent_id)
			VALUES ($1, $2)
			ON CONFLICT (task_id, agent_id) DO NOTHING
		`, task.ID, agentID)
		if err != nil {
			return fmt.Errorf("failed to insert task target %s: %w", agentID, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) scanTaskRow(row pgx.Row) (*models.Task, error) {
	var t models.Task
	var planJSON, failureJSON []byte
	var errMsg, execSum, idempKey *string
	var statusStr, riskStr string

	err := row.Scan(
		&t.ID, &t.Title, &t.Prompt, &statusStr, &t.PlanVersion, &idempKey, &t.ReplanCount, &t.MaxReplans,
		&planJSON, &riskStr, &errMsg, &execSum, &failureJSON, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Status = models.TaskStatus(statusStr)
	t.RiskLevel = models.RiskLevel(riskStr)
	t.IdempotencyKey = idempKey
	if errMsg != nil {
		t.ErrorMessage = *errMsg
	}
	if execSum != nil {
		t.ExecutionSummary = *execSum
	}
	if len(planJSON) > 0 {
		_ = json.Unmarshal(planJSON, &t.AIPlan)
	}
	if len(failureJSON) > 0 {
		_ = json.Unmarshal(failureJSON, &t.FailureDetails)
	}
	return &t, nil
}

func (s *PostgresStore) GetTask(ctx context.Context, id string) (*models.Task, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, title, prompt, status, plan_version, idempotency_key, replan_count, max_replans,
		       ai_plan, risk_level, error_message, execution_summary, failure_details,
		       created_at, updated_at, completed_at
		FROM tasks WHERE id = $1
	`, id)

	t, err := s.scanTaskRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, err
	}

	// Targets
	targetRows, err := s.pool.Query(ctx, "SELECT agent_id FROM task_targets WHERE task_id = $1", t.ID)
	if err == nil {
		defer targetRows.Close()
		for targetRows.Next() {
			var aID string
			if err := targetRows.Scan(&aID); err == nil {
				t.TargetAgentIDs = append(t.TargetAgentIDs, aID)
			}
		}
	}

	// Steps
	steps, _ := s.GetTaskSteps(ctx, t.ID)
	t.Steps = steps

	return t, nil
}

func (s *PostgresStore) GetTaskByIdempotencyKey(ctx context.Context, key string) (*models.Task, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, title, prompt, status, plan_version, idempotency_key, replan_count, max_replans,
		       ai_plan, risk_level, error_message, execution_summary, failure_details,
		       created_at, updated_at, completed_at
		FROM tasks WHERE idempotency_key = $1
	`, key)

	t, err := s.scanTaskRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("task with idempotency key %s not found", key)
		}
		return nil, err
	}

	targetRows, err := s.pool.Query(ctx, "SELECT agent_id FROM task_targets WHERE task_id = $1", t.ID)
	if err == nil {
		defer targetRows.Close()
		for targetRows.Next() {
			var aID string
			if err := targetRows.Scan(&aID); err == nil {
				t.TargetAgentIDs = append(t.TargetAgentIDs, aID)
			}
		}
	}

	steps, _ := s.GetTaskSteps(ctx, t.ID)
	t.Steps = steps

	return t, nil
}

func (s *PostgresStore) ListTasks(ctx context.Context) ([]*models.Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, prompt, status, plan_version, idempotency_key, replan_count, max_replans,
		       ai_plan, risk_level, error_message, execution_summary, failure_details,
		       created_at, updated_at, completed_at
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := s.scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	for _, t := range tasks {
		targetRows, err := s.pool.Query(ctx, "SELECT agent_id FROM task_targets WHERE task_id = $1", t.ID)
		if err == nil {
			for targetRows.Next() {
				var aID string
				if err := targetRows.Scan(&aID); err == nil {
					t.TargetAgentIDs = append(t.TargetAgentIDs, aID)
				}
			}
			targetRows.Close()
		}
		steps, _ := s.GetTaskSteps(ctx, t.ID)
		t.Steps = steps
	}

	return tasks, nil
}

func (s *PostgresStore) UpdateTask(ctx context.Context, task *models.Task) error {
	return s.SaveTask(ctx, task)
}

// -------------------------------------------------------------
// TASK STEPS
// -------------------------------------------------------------

func (s *PostgresStore) SaveTaskStep(ctx context.Context, step *models.TaskStep) error {
	argsJSON, _ := json.Marshal(step.Arguments)
	var stratJSON []byte
	if step.VerificationStrategy != nil {
		stratJSON, _ = json.Marshal(step.VerificationStrategy)
	}

	query := `
		INSERT INTO task_steps (
			id, task_id, step_order, action, execution_id, arguments, risk_level,
			requires_approval, verification_strategy, status, exit_code,
			stdout, stderr, started_at, finished_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			step_order = EXCLUDED.step_order,
			action = EXCLUDED.action,
			execution_id = EXCLUDED.execution_id,
			arguments = EXCLUDED.arguments,
			risk_level = EXCLUDED.risk_level,
			requires_approval = EXCLUDED.requires_approval,
			verification_strategy = EXCLUDED.verification_strategy,
			status = EXCLUDED.status,
			exit_code = EXCLUDED.exit_code,
			stdout = EXCLUDED.stdout,
			stderr = EXCLUDED.stderr,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at
	`
	now := time.Now()
	_, err := s.pool.Exec(ctx, query,
		step.ID, step.TaskID, step.StepOrder, step.Action, step.ExecutionID, argsJSON, string(step.RiskLevel),
		step.RequiresApproval, stratJSON, step.Status, step.ExitCode,
		step.Stdout, step.Stderr, step.StartedAt, step.FinishedAt, now,
	)
	return err
}

func (s *PostgresStore) GetTaskSteps(ctx context.Context, taskID string) ([]*models.TaskStep, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, task_id, step_order, action, COALESCE(execution_id, ''), arguments, risk_level,
		       requires_approval, verification_strategy, status, exit_code,
		       stdout, stderr, started_at, finished_at
		FROM task_steps WHERE task_id = $1 ORDER BY step_order ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*models.TaskStep
	for rows.Next() {
		var st models.TaskStep
		var argsJSON, stratJSON []byte
		var riskStr, execID string
		var stdout, stderr *string

		if err := rows.Scan(
			&st.ID, &st.TaskID, &st.StepOrder, &st.Action, &execID, &argsJSON, &riskStr,
			&st.RequiresApproval, &stratJSON, &st.Status, &st.ExitCode,
			&stdout, &stderr, &st.StartedAt, &st.FinishedAt,
		); err != nil {
			return nil, err
		}

		st.ExecutionID = execID
		st.RiskLevel = models.RiskLevel(riskStr)
		if stdout != nil {
			st.Stdout = *stdout
		}
		if stderr != nil {
			st.Stderr = *stderr
		}
		if len(argsJSON) > 0 {
			_ = json.Unmarshal(argsJSON, &st.Arguments)
		}
		if len(stratJSON) > 0 {
			_ = json.Unmarshal(stratJSON, &st.VerificationStrategy)
		}
		steps = append(steps, &st)
	}

	return steps, nil
}

func (s *PostgresStore) UpdateTaskStep(ctx context.Context, step *models.TaskStep) error {
	return s.SaveTaskStep(ctx, step)
}

// -------------------------------------------------------------
// APPROVALS
// -------------------------------------------------------------

func (s *PostgresStore) SaveApproval(ctx context.Context, app *models.Approval) error {
	var decidedByUUID *uuid.UUID
	if parsed, err := uuid.Parse(app.DecidedBy); err == nil {
		decidedByUUID = &parsed
	}

	query := `
		INSERT INTO approvals (
			id, task_id, plan_version, status, risk_level,
			decided_by, decision_notes, decided_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (task_id, plan_version) DO UPDATE SET
			status = EXCLUDED.status,
			risk_level = EXCLUDED.risk_level,
			decided_by = EXCLUDED.decided_by,
			decision_notes = EXCLUDED.decision_notes,
			decided_at = EXCLUDED.decided_at
	`
	now := time.Now()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}

	_, err := s.pool.Exec(ctx, query,
		app.ID, app.TaskID, app.PlanVersion, app.Status, string(app.RiskLevel),
		decidedByUUID, app.DecisionNotes, app.DecidedAt, app.CreatedAt,
	)
	return err
}

func (s *PostgresStore) GetApproval(ctx context.Context, taskID string, version int) (*models.Approval, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, task_id, plan_version, status, risk_level,
		       decided_by, decision_notes, decided_at, created_at
		FROM approvals WHERE task_id = $1 AND plan_version = $2
	`, taskID, version)

	var app models.Approval
	var decidedByUUID *uuid.UUID
	var notes *string
	var riskStr string

	err := row.Scan(
		&app.ID, &app.TaskID, &app.PlanVersion, &app.Status, &riskStr,
		&decidedByUUID, &notes, &app.DecidedAt, &app.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("approval not found: %s version %d", taskID, version)
		}
		return nil, err
	}

	app.RiskLevel = models.RiskLevel(riskStr)
	if decidedByUUID != nil {
		app.DecidedBy = decidedByUUID.String()
	}
	if notes != nil {
		app.DecisionNotes = *notes
	}

	return &app, nil
}

func (s *PostgresStore) UpdateApproval(ctx context.Context, app *models.Approval) error {
	return s.SaveApproval(ctx, app)
}

// -------------------------------------------------------------
// AUDIT LOGS
// -------------------------------------------------------------

func (s *PostgresStore) SaveAuditEvent(ctx context.Context, event *models.AuditEvent) error {
	var taskUUID *uuid.UUID
	if event.TaskID != "" {
		if parsed, err := uuid.Parse(event.TaskID); err == nil {
			taskUUID = &parsed
		}
	}

	var userUUID *uuid.UUID
	if event.UserID != "" {
		if parsed, err := uuid.Parse(event.UserID); err == nil {
			userUUID = &parsed
		}
	}

	detailsJSON, _ := json.Marshal(event.Details)
	now := time.Now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}

	query := `
		INSERT INTO audit_events (
			user_id, username, agent_id, task_id, event_type, action, details, ip_address, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	return s.pool.QueryRow(ctx, query,
		userUUID, event.Username, event.AgentID, taskUUID, event.EventType,
		event.Action, detailsJSON, event.IPAddress, event.CreatedAt,
	).Scan(&event.ID)
}

func (s *PostgresStore) ListAuditEvents(ctx context.Context, limit int) ([]*models.AuditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, username, agent_id, task_id, event_type, action, details, ip_address, created_at
		FROM audit_events ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.AuditEvent
	for rows.Next() {
		var ev models.AuditEvent
		var userUUID, taskUUID *uuid.UUID
		var username, agentID, action, ipAddr *string
		var detailsJSON []byte

		if err := rows.Scan(
			&ev.ID, &userUUID, &username, &agentID, &taskUUID, &ev.EventType,
			&action, &detailsJSON, &ipAddr, &ev.CreatedAt,
		); err != nil {
			return nil, err
		}

		if userUUID != nil {
			ev.UserID = userUUID.String()
		}
		if taskUUID != nil {
			ev.TaskID = taskUUID.String()
		}
		if username != nil {
			ev.Username = *username
		}
		if agentID != nil {
			ev.AgentID = *agentID
		}
		if action != nil {
			ev.Action = *action
		}
		if ipAddr != nil {
			ev.IPAddress = *ipAddr
		}
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &ev.Details)
		}
		list = append(list, &ev)
	}

	return list, nil
}

// -------------------------------------------------------------
// RUNBOOKS
// -------------------------------------------------------------

func (s *PostgresStore) SaveRunbook(ctx context.Context, rb *models.Runbook) error {
	var taskUUID *uuid.UUID
	if rb.CreatedFromTask != "" {
		if parsed, err := uuid.Parse(rb.CreatedFromTask); err == nil {
			taskUUID = &parsed
		}
	}

	query := `
		INSERT INTO runbooks (
			id, slug, title, description, created_from_task, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			updated_at = EXCLUDED.updated_at
	`
	now := time.Now()
	_, err := s.pool.Exec(ctx, query,
		rb.ID, rb.Slug, rb.Title, rb.Description, taskUUID, now, now,
	)
	return err
}

func (s *PostgresStore) GetRunbook(ctx context.Context, idOrSlug string) (*models.Runbook, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, slug, title, description, created_from_task, created_at, updated_at
		FROM runbooks WHERE id::text = $1 OR slug = $1
	`, idOrSlug)

	var rb models.Runbook
	var taskUUID *uuid.UUID
	var desc *string

	err := row.Scan(&rb.ID, &rb.Slug, &rb.Title, &desc, &taskUUID, &rb.CreatedAt, &rb.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("runbook not found: %s", idOrSlug)
		}
		return nil, err
	}
	if desc != nil {
		rb.Description = *desc
	}
	if taskUUID != nil {
		rb.CreatedFromTask = taskUUID.String()
	}
	return &rb, nil
}

func (s *PostgresStore) ListRunbooks(ctx context.Context) ([]*models.Runbook, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, slug, title, description, created_from_task, created_at, updated_at
		FROM runbooks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.Runbook
	for rows.Next() {
		var rb models.Runbook
		var taskUUID *uuid.UUID
		var desc *string
		if err := rows.Scan(&rb.ID, &rb.Slug, &rb.Title, &desc, &taskUUID, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
			return nil, err
		}
		if desc != nil {
			rb.Description = *desc
		}
		if taskUUID != nil {
			rb.CreatedFromTask = taskUUID.String()
		}
		list = append(list, &rb)
	}
	return list, nil
}

// -------------------------------------------------------------
// VERIFICATION RESULTS
// -------------------------------------------------------------

func (s *PostgresStore) SaveVerificationResult(ctx context.Context, res *models.VerificationResult) error {
	var stepUUID *uuid.UUID
	if res.StepID != "" {
		if parsed, err := uuid.Parse(res.StepID); err == nil {
			stepUUID = &parsed
		}
	}

	detailsJSON, _ := json.Marshal(res.Details)
	now := time.Now()

	query := `
		INSERT INTO verification_results (
			id, task_id, step_id, agent_id, check_type, target, passed, details, verified_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := s.pool.Exec(ctx, query,
		res.ID, res.TaskID, stepUUID, res.AgentID, res.CheckType, res.Target,
		res.Passed, detailsJSON, now,
	)
	return err
}

func (s *PostgresStore) GetVerificationResults(ctx context.Context, taskID string) ([]*models.VerificationResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, task_id, step_id, agent_id, check_type, target, passed, details, verified_at
		FROM verification_results WHERE task_id = $1 ORDER BY verified_at ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.VerificationResult
	for rows.Next() {
		var vr models.VerificationResult
		var stepUUID *uuid.UUID
		var detailsJSON []byte

		if err := rows.Scan(
			&vr.ID, &vr.TaskID, &stepUUID, &vr.AgentID, &vr.CheckType, &vr.Target,
			&vr.Passed, &detailsJSON, &vr.VerifiedAt,
		); err != nil {
			return nil, err
		}

		if stepUUID != nil {
			vr.StepID = stepUUID.String()
		}
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &vr.Details)
		}
		list = append(list, &vr)
	}
	return list, nil
}

// -------------------------------------------------------------
// AI INVOCATIONS PROVENANCE
// -------------------------------------------------------------

func (s *PostgresStore) SaveAIInvocation(ctx context.Context, inv *models.AIProvenanceData) error {
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = time.Now()
	}
	query := `
		INSERT INTO ai_invocations (
			invocation_id, task_id, scenario_id, purpose, provider, model, model_digest,
			fallback_used, fallback_reason, request_started_at, request_finished_at,
			latency_ms, success, schema_valid, error_type, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16
		) ON CONFLICT (invocation_id) DO NOTHING`
	started := inv.CreatedAt.Add(-time.Duration(inv.LatencyMS) * time.Millisecond)
	_, err := s.pool.Exec(ctx, query,
		inv.InvocationID, inv.TaskID, inv.ScenarioID, inv.Purpose, inv.Provider, inv.Model, inv.ModelDigest,
		inv.FallbackUsed, inv.FallbackReason, started, inv.CreatedAt,
		inv.LatencyMS, true, inv.SchemaValid, inv.ErrorType, inv.CreatedAt,
	)
	return err
}

func (s *PostgresStore) ListAIInvocations(ctx context.Context, taskID string) ([]*models.AIProvenanceData, error) {
	query := `
		SELECT invocation_id, COALESCE(task_id, ''), COALESCE(scenario_id, ''), purpose, provider, model, COALESCE(model_digest, ''),
		       fallback_used, COALESCE(fallback_reason, ''), latency_ms, schema_valid, COALESCE(error_type, ''), created_at
		FROM ai_invocations
		WHERE ($1 = '' OR task_id = $1)
		ORDER BY created_at ASC`
	rows, err := s.pool.Query(ctx, query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.AIProvenanceData
	for rows.Next() {
		var inv models.AIProvenanceData
		if err := rows.Scan(
			&inv.InvocationID, &inv.TaskID, &inv.ScenarioID, &inv.Purpose, &inv.Provider, &inv.Model, &inv.ModelDigest,
			&inv.FallbackUsed, &inv.FallbackReason, &inv.LatencyMS, &inv.SchemaValid, &inv.ErrorType, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, &inv)
	}
	return list, nil
}
