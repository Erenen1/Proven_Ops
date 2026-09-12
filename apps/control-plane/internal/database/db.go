package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opspilot/control-plane/internal/models"
)

type Store interface {
	// Agent methods
	SaveAgent(ctx context.Context, agent *models.Agent) error
	GetAgent(ctx context.Context, id string) (*models.Agent, error)
	ListAgents(ctx context.Context) ([]*models.Agent, error)
	UpdateAgentStatus(ctx context.Context, id, status string) error
	SaveHeartbeat(ctx context.Context, agentID string, metrics *models.AgentMetrics) error

	// Task methods
	SaveTask(ctx context.Context, task *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	GetTaskByIdempotencyKey(ctx context.Context, key string) (*models.Task, error)
	ListTasks(ctx context.Context) ([]*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error

	// Steps & Executions
	SaveTaskStep(ctx context.Context, step *models.TaskStep) error
	GetTaskSteps(ctx context.Context, taskID string) ([]*models.TaskStep, error)
	UpdateTaskStep(ctx context.Context, step *models.TaskStep) error

	// Approvals
	SaveApproval(ctx context.Context, app *models.Approval) error
	GetApproval(ctx context.Context, taskID string, version int) (*models.Approval, error)
	UpdateApproval(ctx context.Context, app *models.Approval) error

	// Audit Logs
	SaveAuditEvent(ctx context.Context, event *models.AuditEvent) error
	ListAuditEvents(ctx context.Context, limit int) ([]*models.AuditEvent, error)

	// Runbooks
	SaveRunbook(ctx context.Context, rb *models.Runbook) error
	GetRunbook(ctx context.Context, idOrSlug string) (*models.Runbook, error)
	ListRunbooks(ctx context.Context) ([]*models.Runbook, error)

	// Verification Results
	SaveVerificationResult(ctx context.Context, res *models.VerificationResult) error
	GetVerificationResults(ctx context.Context, taskID string) ([]*models.VerificationResult, error)
}

type MemoryStore struct {
	mu            sync.RWMutex
	agents        map[string]*models.Agent
	tasks         map[string]*models.Task
	steps         map[string][]*models.TaskStep
	approvals     map[string]*models.Approval // key: taskID:version
	auditEvents   []*models.AuditEvent
	runbooks      map[string]*models.Runbook
	verifications map[string][]*models.VerificationResult
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		agents:        make(map[string]*models.Agent),
		tasks:         make(map[string]*models.Task),
		steps:         make(map[string][]*models.TaskStep),
		approvals:     make(map[string]*models.Approval),
		auditEvents:   make([]*models.AuditEvent, 0),
		runbooks:      make(map[string]*models.Runbook),
		verifications: make(map[string][]*models.VerificationResult),
	}
}

func (s *MemoryStore) SaveAgent(ctx context.Context, agent *models.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[agent.ID] = agent
	return nil
}

func (s *MemoryStore) GetAgent(ctx context.Context, id string) (*models.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	return agent, nil
}

func (s *MemoryStore) ListAgents(ctx context.Context) ([]*models.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Agent
	for _, a := range s.agents {
		list = append(list, a)
	}
	return list, nil
}

func (s *MemoryStore) UpdateAgentStatus(ctx context.Context, id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if agent, ok := s.agents[id]; ok {
		agent.Status = status
	}
	return nil
}

func (s *MemoryStore) SaveHeartbeat(ctx context.Context, agentID string, metrics *models.AgentMetrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if agent, ok := s.agents[agentID]; ok {
		agent.Status = "online"
		now := time.Now()
		agent.LastHeartbeat = &now
		agent.LastMetrics = metrics
	}
	return nil
}

func (s *MemoryStore) SaveTask(ctx context.Context, task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *MemoryStore) GetTask(ctx context.Context, id string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if steps, ok := s.steps[id]; ok {
		task.Steps = steps
	}
	return task, nil
}

func (s *MemoryStore) GetTaskByIdempotencyKey(ctx context.Context, key string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.IdempotencyKey != nil && *t.IdempotencyKey == key {
			if steps, ok := s.steps[t.ID]; ok {
				t.Steps = steps
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("task with idempotency key %s not found", key)
}

func (s *MemoryStore) ListTasks(ctx context.Context) ([]*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*models.Task
	for _, t := range s.tasks {
		if steps, ok := s.steps[t.ID]; ok {
			t.Steps = steps
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *MemoryStore) UpdateTask(ctx context.Context, task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *MemoryStore) SaveTaskStep(ctx context.Context, step *models.TaskStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps[step.TaskID] = append(s.steps[step.TaskID], step)
	return nil
}

func (s *MemoryStore) GetTaskSteps(ctx context.Context, taskID string) ([]*models.TaskStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.steps[taskID], nil
}

func (s *MemoryStore) UpdateTaskStep(ctx context.Context, step *models.TaskStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := s.steps[step.TaskID]
	for i, st := range steps {
		if st.ID == step.ID {
			steps[i] = step
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) SaveApproval(ctx context.Context, app *models.Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%d", app.TaskID, app.PlanVersion)
	s.approvals[key] = app
	return nil
}

func (s *MemoryStore) GetApproval(ctx context.Context, taskID string, version int) (*models.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := fmt.Sprintf("%s:%d", taskID, version)
	app, ok := s.approvals[key]
	if !ok {
		return nil, fmt.Errorf("approval not found for %s", key)
	}
	return app, nil
}

func (s *MemoryStore) UpdateApproval(ctx context.Context, app *models.Approval) error {
	return s.SaveApproval(ctx, app)
}

func (s *MemoryStore) SaveAuditEvent(ctx context.Context, event *models.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event.ID = int64(len(s.auditEvents) + 1)
	s.auditEvents = append([]*models.AuditEvent{event}, s.auditEvents...)
	return nil
}

func (s *MemoryStore) ListAuditEvents(ctx context.Context, limit int) ([]*models.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.auditEvents) {
		limit = len(s.auditEvents)
	}
	return s.auditEvents[:limit], nil
}

func (s *MemoryStore) SaveRunbook(ctx context.Context, rb *models.Runbook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runbooks[rb.ID] = rb
	s.runbooks[rb.Slug] = rb
	return nil
}

func (s *MemoryStore) GetRunbook(ctx context.Context, idOrSlug string) (*models.Runbook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rb, ok := s.runbooks[idOrSlug]
	if !ok {
		return nil, fmt.Errorf("runbook not found: %s", idOrSlug)
	}
	return rb, nil
}

func (s *MemoryStore) ListRunbooks(ctx context.Context) ([]*models.Runbook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	var list []*models.Runbook
	for _, rb := range s.runbooks {
		if !seen[rb.ID] {
			seen[rb.ID] = true
			list = append(list, rb)
		}
	}
	return list, nil
}

func (s *MemoryStore) SaveVerificationResult(ctx context.Context, res *models.VerificationResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifications[res.TaskID] = append(s.verifications[res.TaskID], res)
	return nil
}

func (s *MemoryStore) GetVerificationResults(ctx context.Context, taskID string) ([]*models.VerificationResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.verifications[taskID], nil
}

// ConnectPool attempts PostgreSQL connection or returns nil if unavailable
func ConnectPool(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return pgxpool.New(ctxWithTimeout, dbURL)
}
