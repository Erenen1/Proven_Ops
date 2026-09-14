package models

import (
	"encoding/json"
	"time"
)

type TaskStatus string

const (
	TaskStatusCreated         TaskStatus = "CREATED"
	TaskStatusDiscovering     TaskStatus = "DISCOVERING"
	TaskStatusPlanning        TaskStatus = "PLANNING"
	TaskStatusWaitingApproval TaskStatus = "WAITING_APPROVAL"
	TaskStatusExecuting       TaskStatus = "EXECUTING"
	TaskStatusObserving       TaskStatus = "OBSERVING"
	TaskStatusVerifying       TaskStatus = "VERIFYING"
	TaskStatusReplanning      TaskStatus = "REPLANNING"
	TaskStatusCompleted       TaskStatus = "COMPLETED"
	TaskStatusPartialSuccess  TaskStatus = "PARTIAL_SUCCESS"
	TaskStatusFailed          TaskStatus = "FAILED"
	TaskStatusRollingBack     TaskStatus = "ROLLING_BACK"
	TaskStatusRolledBack      TaskStatus = "ROLLED_BACK"
	TaskStatusRollbackFailed  TaskStatus = "ROLLBACK_FAILED"
	TaskStatusWaitingForAgent TaskStatus = "WAITING_FOR_AGENT"
	TaskStatusCancelled       TaskStatus = "CANCELLED"
	TaskStatusTimeout         TaskStatus = "TIMEOUT"
)

type FailureType string

const (
	FailureExecutionFailed    FailureType = "EXECUTION_FAILED"
	FailureValidationFailed   FailureType = "VALIDATION_FAILED"
	FailureVerificationFailed FailureType = "VERIFICATION_FAILED"
	FailureTimeout            FailureType = "TIMEOUT"
	FailureAgentDisconnected  FailureType = "AGENT_DISCONNECTED"
	FailurePolicyDenied       FailureType = "POLICY_DENIED"
	FailurePermissionDenied   FailureType = "PERMISSION_DENIED"
	FailureResourceConflict   FailureType = "RESOURCE_CONFLICT"
	FailureInvalidModelOutput FailureType = "INVALID_MODEL_OUTPUT"
	FailureUnsupportedOp      FailureType = "UNSUPPORTED_OPERATION"
	FailureDependencyFailure  FailureType = "DEPENDENCY_FAILURE"
	FailureCancelled          FailureType = "CANCELLED"
	FailureUncertainExecution FailureType = "UNCERTAIN_EXECUTION"
	FailureResourceNotFound   FailureType = "RESOURCE_NOT_FOUND"
	FailureReplanExhausted    FailureType = "REPLAN_EXHAUSTED"
)

type StructuredFailure struct {
	Type               FailureType    `json:"type"`
	Action             string         `json:"action"`
	Target             string         `json:"target,omitempty"`
	Message            string         `json:"message"`
	Retryable          bool           `json:"retryable"`
	RequiresReplan     bool           `json:"requires_replan"`
	RollbackRequired   bool           `json:"rollback_required"`
	UserActionRequired bool           `json:"user_action_required"`
	Details            map[string]any `json:"details,omitempty"`
}

type RiskLevel string

const (
	RiskReadOnly  RiskLevel = "READ_ONLY"
	RiskLow       RiskLevel = "LOW"
	RiskMedium    RiskLevel = "MEDIUM"
	RiskHigh      RiskLevel = "HIGH"
	RiskForbidden RiskLevel = "FORBIDDEN"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Roles        []string  `json:"roles"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

type Agent struct {
	ID             string            `json:"id"`
	Hostname       string            `json:"hostname"`
	IPAddress      string            `json:"ip_address"`
	OS             string            `json:"os"`
	Distribution   string            `json:"distribution"`
	Version        string            `json:"version"`
	Architecture   string            `json:"architecture"`
	Status         string            `json:"status"` // "online", "offline", "busy"
	Environment    string            `json:"environment"`
	Capabilities   []string          `json:"capabilities"`
	LastHeartbeat  *time.Time        `json:"last_heartbeat"`
	LastMetrics    *AgentMetrics     `json:"last_metrics,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

type AgentMetrics struct {
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`
	MemoryUsageBytes int64     `json:"memory_usage_bytes"`
	MemoryTotalBytes int64     `json:"memory_total_bytes"`
	DiskUsagePercent float64   `json:"disk_usage_percent"`
	LoadAvg1m        float64   `json:"load_avg_1m"`
	ActiveTasks      int       `json:"active_tasks"`
	RecordedAt       time.Time `json:"recorded_at"`
}

type Task struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	Prompt           string             `json:"prompt"`
	Status           TaskStatus         `json:"status"`
	CreatedBy        string             `json:"created_by"`
	TargetAgentIDs   []string           `json:"target_agent_ids"`
	PlanVersion      int                `json:"plan_version"`
	IdempotencyKey   *string            `json:"idempotency_key,omitempty"`
	ReplanCount      int                `json:"replan_count"`
	MaxReplans       int                `json:"max_replans"`
	AIPlan           *AIPlanData        `json:"ai_plan,omitempty"`
	RiskLevel        RiskLevel          `json:"risk_level"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	ExecutionSummary string             `json:"execution_summary,omitempty"`
	FailureDetails   *StructuredFailure `json:"failure_details,omitempty"`
	Steps            []*TaskStep        `json:"steps"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	CompletedAt      *time.Time         `json:"completed_at,omitempty"`
}

type AIPlanData struct {
	Goal                string                  `json:"goal"`
	Reasoning           string                  `json:"reasoning"`
	Steps               []*AIStepPlan           `json:"steps"`
	OverallVerification []*VerificationStrategy `json:"overall_verification,omitempty"`
}

type AIStepPlan struct {
	ID                   string                `json:"id"`
	Action               string                `json:"action"`
	Arguments            map[string]any        `json:"arguments"`
	Reason               string                `json:"reason"`
	SuggestedRisk        RiskLevel             `json:"suggested_risk"`
	VerificationStrategy *VerificationStrategy `json:"verification_strategy,omitempty"`
}

type VerificationStrategy struct {
	CheckType  string `json:"check_type"` // "systemd_active", "tcp_port_open", "http_probe", "package_installed"
	Target     string `json:"target"`     // e.g. "nginx", "8080", "http://127.0.0.1:8080"
	Expected   string `json:"expected"`
	TimeoutSec int    `json:"timeout_sec"`
}

type TaskStep struct {
	ID                   string                `json:"id"`
	TaskID               string                `json:"task_id"`
	StepOrder            int                   `json:"step_order"`
	Action               string                `json:"action"`
	ExecutionID          string                `json:"execution_id,omitempty"`
	Arguments            map[string]any        `json:"arguments"`
	RiskLevel            RiskLevel             `json:"risk_level"`
	RequiresApproval     bool                  `json:"requires_approval"`
	VerificationStrategy *VerificationStrategy `json:"verification_strategy,omitempty"`
	Status               string                `json:"status"` // "PENDING", "RUNNING", "SUCCESS", "FAILED", "SKIPPED"
	ExitCode             *int                  `json:"exit_code,omitempty"`
	Stdout               string                `json:"stdout,omitempty"`
	Stderr               string                `json:"stderr,omitempty"`
	StartedAt            *time.Time            `json:"started_at,omitempty"`
	FinishedAt           *time.Time            `json:"finished_at,omitempty"`
}

type TaskEvent struct {
	ID        int64          `json:"id"`
	TaskID    string         `json:"task_id"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type TaskTransition struct {
	ID             int64      `json:"id"`
	TaskID         string     `json:"task_id"`
	FromStatus     TaskStatus `json:"from_status"`
	ToStatus       TaskStatus `json:"to_status"`
	Reason         string     `json:"reason"`
	TriggeredBy    string     `json:"triggered_by,omitempty"`
	TransitionedAt time.Time  `json:"transitioned_at"`
}

type Approval struct {
	ID            string     `json:"id"`
	TaskID        string     `json:"task_id"`
	PlanVersion   int        `json:"plan_version"`
	Status        string     `json:"status"` // "PENDING", "APPROVED", "REJECTED", "EXPIRED"
	RiskLevel     RiskLevel  `json:"risk_level"`
	DecidedBy     string     `json:"decided_by,omitempty"`
	DecisionNotes string     `json:"decision_notes,omitempty"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Policy struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Scope            string    `json:"scope"` // "global", "environment", "agent"
	ScopeValue       string    `json:"scope_value,omitempty"`
	ActionPattern    string    `json:"action_pattern"`
	MaxRiskLevel     RiskLevel `json:"max_risk_level"`
	RequiresApproval bool      `json:"requires_approval"`
	IsEnabled        bool      `json:"is_enabled"`
}

type VerificationResult struct {
	ID         string         `json:"id"`
	TaskID     string         `json:"task_id"`
	StepID     string         `json:"step_id,omitempty"`
	AgentID    string         `json:"agent_id"`
	CheckType  string         `json:"check_type"`
	Target     string         `json:"target"`
	Passed     bool           `json:"passed"`
	Details    map[string]any `json:"details"`
	VerifiedAt time.Time      `json:"verified_at"`
}

type AuditEvent struct {
	ID        int64          `json:"id"`
	UserID    string         `json:"user_id,omitempty"`
	Username  string         `json:"username,omitempty"`
	AgentID   string         `json:"agent_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	EventType string         `json:"event_type"`
	Action    string         `json:"action,omitempty"`
	Details   map[string]any `json:"details"`
	IPAddress string         `json:"ip_address,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Runbook struct {
	ID              string           `json:"id"`
	Slug            string           `json:"slug"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	CreatedFromTask string           `json:"created_from_task,omitempty"`
	LatestVersion   int              `json:"latest_version"`
	Variables       []map[string]any `json:"variables"`
	Steps           []*AIStepPlan    `json:"steps"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

func (t *Task) MarshalPlan() string {
	if t.AIPlan == nil {
		return "{}"
	}
	b, _ := json.Marshal(t.AIPlan)
	return string(b)
}
