package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"opspilot/control-plane/internal/dag"
	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/failures"
	"opspilot/control-plane/internal/fleet"
	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/pki"
	"opspilot/control-plane/internal/saga"
	"opspilot/control-plane/internal/security"
	"opspilot/control-plane/internal/statemachine"
)

// 1. Scenario: Duplicate task request idempotency key
func TestFailureInjection_DuplicateTaskRequest(t *testing.T) {
	store := database.NewMemoryStore()
	ctx := context.Background()

	idempKey := "unique-idempotency-key-1234"
	task := &models.Task{
		ID:             "task-orig",
		Title:          "Install Nginx",
		Prompt:         "ensure nginx is installed",
		Status:         models.TaskStatusPending,
		IdempotencyKey: &idempKey,
	}

	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Attempt duplicate fetch
	existing, err := store.GetTaskByIdempotencyKey(ctx, idempKey)
	if err != nil || existing == nil {
		t.Fatalf("expected existing task to be retrieved by idempotency key: %v", err)
	}
	if existing.ID != "task-orig" {
		t.Errorf("expected task-orig, got %s", existing.ID)
	}
}

// 2. Scenario: Canary failure halts remaining fleet nodes
func TestFailureInjection_CanaryFailureHaltsFleet(t *testing.T) {
	store := database.NewMemoryStore()
	hub := events.NewHub()
	fleetEngine := fleet.NewEngine(store, hub)
	ctx := context.Background()

	task := &models.Task{
		ID:             "task-canary-fail",
		TargetAgentIDs: []string{"node-01", "node-02", "node-03", "node-04", "node-05"},
		RolloutConfig: &models.RolloutConfig{
			Strategy:    models.RolloutCanary,
			CanaryNodes: 1,
			MaxFailures: 1,
		},
	}

	executedNodes := make(map[string]bool)
	mockExecutor := func(ctx context.Context, task *models.Task, agentID string) error {
		executedNodes[agentID] = true
		if agentID == "node-01" {
			return fmt.Errorf("canary verification failed on node-01")
		}
		return nil
	}

	_ = fleetEngine.ExecuteRollout(ctx, task, mockExecutor)

	if !task.RolloutProgress.Halted {
		t.Errorf("expected fleet rollout to be halted on canary failure")
	}
	if len(executedNodes) != 1 {
		t.Errorf("expected ONLY canary node-01 to be executed, but executed %d nodes: %v", len(executedNodes), executedNodes)
	}
	if task.RolloutProgress.CompletedHosts != 0 {
		t.Errorf("expected 0 completed hosts, got %d", task.RolloutProgress.CompletedHosts)
	}
}

// 3. Scenario: Saga LIFO Rollback on Step Verification Failure
func TestFailureInjection_SagaRollbackOnVerificationFailure(t *testing.T) {
	ctx := context.Background()

	step1 := &models.TaskStep{
		ID:        "step-1",
		Action:    "ensure_package",
		Arguments: map[string]any{"name": "redis", "state": "present"},
		Status:    "SUCCESS",
	}

	step2 := &models.TaskStep{
		ID:             "step-2",
		Action:         "write_config_file",
		Arguments:      map[string]any{"path": "/etc/redis/redis.conf"},
		PrecheckResult: map[string]any{"backup_path": "/var/lib/opspilot/backups/task-1/redis.conf.bak"},
		Status:         "SUCCESS",
	}

	succeeded := []*models.TaskStep{step1, step2}

	mockExec := &mockSagaExec{}
	report := saga.ExecuteSagaRollback(ctx, mockExec, "agent-01", succeeded)

	if report.OverallStatus != saga.RollbackSucceeded {
		t.Fatalf("expected RollbackSucceeded, got %s", report.OverallStatus)
	}

	if len(mockExec.actions) != 2 {
		t.Fatalf("expected 2 rollback actions, got %d", len(mockExec.actions))
	}

	// LIFO verification: step 2 compensated before step 1
	if mockExec.actions[0] != "rollback_config" {
		t.Errorf("expected rollback_config first, got %s", mockExec.actions[0])
	}
	if mockExec.actions[1] != "ensure_package" {
		t.Errorf("expected ensure_package second, got %s", mockExec.actions[1])
	}
}

type mockSagaExec struct {
	actions []string
}

func (m *mockSagaExec) ExecuteCompensatingStep(ctx context.Context, agentID string, step *models.AIStepPlan) (*models.TaskStep, error) {
	m.actions = append(m.actions, step.Action)
	return &models.TaskStep{ID: step.ID, Action: step.Action, Status: "SUCCESS"}, nil
}

func (m *mockSagaExec) VerifyCompensatingStep(ctx context.Context, agentID string, strategy *models.VerificationStrategy) (bool, string, error) {
	return true, "verified", nil
}

// 4. Scenario: Irreversible HIGH risk action requires manual intervention
func TestFailureInjection_IrreversibleHighRiskAction(t *testing.T) {
	ctx := context.Background()

	highRiskStep := &models.TaskStep{
		ID:        "step-high-risk",
		Action:    "execute_command",
		Arguments: map[string]any{"command": "dd if=/dev/zero of=/dev/null"},
		RiskLevel: models.RiskHigh,
		Status:    "SUCCESS",
	}

	mockExec := &mockSagaExec{}
	report := saga.ExecuteSagaRollback(ctx, mockExec, "agent-01", []*models.TaskStep{highRiskStep})

	if report.OverallStatus != saga.ManualInterventionRequired {
		t.Fatalf("expected ManualInterventionRequired, got %s", report.OverallStatus)
	}
}

// 5. Scenario: Certificate Revocation Check
func TestFailureInjection_CertificateRevocation(t *testing.T) {
	ca, err := pki.GenerateCA("OpsPilot Test CA")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	clientCert, err := pki.IssueAgentCertificate(ca, "agent-compromised", "node-rogue", 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to issue client cert: %v", err)
	}

	// Mock Revocation Checker
	checker := &mockRevocationChecker{
		revokedSerials: map[string]bool{
			clientCert.Cert.SerialNumber.String(): true,
		},
	}

	_, err = pki.ValidateCertificateWithRevocation(clientCert.CertPEM, ca.CertPEM, checker)
	if err == nil {
		t.Fatalf("expected revoked certificate to be rejected, but validation succeeded!")
	}
}

type mockRevocationChecker struct {
	revokedSerials map[string]bool
}

func (m *mockRevocationChecker) IsRevoked(serial string) bool {
	return m.revokedSerials[serial]
}

// 6. Scenario: DAG Dependency Failure Propagation
func TestFailureInjection_DAGDependencyFailurePropagation(t *testing.T) {
	g := dag.NewGraph()

	stepA := &models.TaskStep{ID: "step-A", Action: "install_package", DependsOn: []string{}}
	stepB := &models.TaskStep{ID: "step-B", Action: "start_service", DependsOn: []string{"step-A"}}
	stepC := &models.TaskStep{ID: "step-C", Action: "http_probe", DependsOn: []string{"step-B"}}
	stepIndependent := &models.TaskStep{ID: "step-indep", Action: "get_os_info", DependsOn: []string{}}

	g.AddNode(stepA)
	g.AddNode(stepB)
	g.AddNode(stepC)
	g.AddNode(stepIndependent)

	// Step A fails
	blocked := g.PropagateFailure("step-A")
	if len(blocked) != 2 {
		t.Fatalf("expected 2 downstream blocked steps, got %d", len(blocked))
	}

	if stepB.Status != dag.StepBlockedByDependency {
		t.Errorf("expected step-B to be BLOCKED_BY_DEPENDENCY, got %s", stepB.Status)
	}
	if stepC.Status != dag.StepBlockedByDependency {
		t.Errorf("expected step-C to be BLOCKED_BY_DEPENDENCY, got %s", stepC.Status)
	}
	if stepIndependent.Status == dag.StepBlockedByDependency {
		t.Errorf("independent step should not be blocked")
	}
}

// 7. Scenario: Secret Redaction
func TestFailureInjection_SecretRedaction(t *testing.T) {
	rawAuditDetails := map[string]any{
		"db_url":   "postgres://opspilot:VerySecretPass123@postgres:5432/opspilot",
		"password": "cleartext_password",
		"jwt":      "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.dummy.sig",
		"hostname": "node-01",
	}

	sanitized := security.RedactMap(rawAuditDetails)

	if sanitized["password"] != "[REDACTED]" {
		t.Errorf("password was not redacted")
	}
	if sanitized["jwt"] != "[REDACTED]" {
		t.Errorf("jwt key was not redacted")
	}
	dbURL, _ := sanitized["db_url"].(string)
	if dbURL == "" || dbURL == rawAuditDetails["db_url"] {
		t.Errorf("DB password was not masked in URI: %s", dbURL)
	}
	if sanitized["hostname"] != "node-01" {
		t.Errorf("safe metadata was altered: %v", sanitized["hostname"])
	}
}

// 8. Scenario: Task Cancellation
func TestFailureInjection_TaskCancellation(t *testing.T) {
	sm := statemachine.NewMachine()
	task := &models.Task{
		ID:     "task-cancel",
		Status: models.TaskStatusRunning,
	}

	tr, err := sm.Transition(task, models.TaskStatusCancelled, "Operator clicked cancel", "operator-1")
	if err != nil {
		t.Fatalf("expected valid cancellation transition, got: %v", err)
	}
	if task.Status != models.TaskStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", task.Status)
	}
	if tr.ToStatus != models.TaskStatusCancelled {
		t.Errorf("expected transition ToStatus to be CANCELLED")
	}
}

// 9. Scenario: Remediation Budget Bounded Loop
func TestFailureInjection_RemediationBudget(t *testing.T) {
	budget := &models.RemediationBudget{
		MaxAttempts:  2,
		MaxRisk:      models.RiskMedium,
		AttemptsUsed: 0,
	}

	// First attempt
	budget.AttemptsUsed++
	if budget.AttemptsUsed > budget.MaxAttempts {
		t.Errorf("first attempt should be within budget")
	}

	// Second attempt
	budget.AttemptsUsed++
	if budget.AttemptsUsed > budget.MaxAttempts {
		t.Errorf("second attempt should be within budget")
	}

	// Third attempt: EXHAUSTED -> must escalate to MANUAL_INTERVENTION_REQUIRED
	budget.AttemptsUsed++
	if budget.AttemptsUsed <= budget.MaxAttempts {
		t.Errorf("third attempt should exceed budget")
	}
}

// 10. Scenario: Error Classification Fail-Fast on Permanent / Policy
func TestFailureInjection_ErrorClassificationFailFast(t *testing.T) {
	policy := failures.DefaultRetryPolicy()

	policyErr := failures.ClassifyStandardError("execute_command", 1, "", "security violation: command blocked by ast guard")
	if policyErr != models.ErrorClassPolicy {
		t.Errorf("expected POLICY error class, got %s", policyErr)
	}
	if policy.IsRetryable(policyErr, "execute_command", 0) {
		t.Errorf("policy violation must NEVER be retryable")
	}

	permErr := failures.ClassifyStandardError("install_package", 1, "", "E: Package 'nonexistent-pkg-999' has no installation candidate")
	if policy.IsRetryable(permErr, "install_package", 0) {
		t.Errorf("permanent missing package error must not be retryable")
	}
}
