package statemachine

import (
	"fmt"
	"sync"
	"time"

	"opspilot/control-plane/internal/models"
)

var validTransitions = map[models.TaskStatus][]models.TaskStatus{
	models.TaskStatusCreated: {
		models.TaskStatusDiscovering,
		models.TaskStatusPlanning,
		models.TaskStatusCancelled,
	},
	models.TaskStatusDiscovering: {
		models.TaskStatusPlanning,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},
	models.TaskStatusPlanning: {
		models.TaskStatusWaitingApproval,
		models.TaskStatusExecuting,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusWaitingApproval: {
		models.TaskStatusExecuting,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},
	models.TaskStatusExecuting: {
		models.TaskStatusObserving,
		models.TaskStatusVerifying,
		models.TaskStatusRollingBack,
		models.TaskStatusWaitingForAgent,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},
	models.TaskStatusObserving: {
		models.TaskStatusExecuting,
		models.TaskStatusVerifying,
		models.TaskStatusReplanning,
		models.TaskStatusRollingBack,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusVerifying: {
		models.TaskStatusCompleted,
		models.TaskStatusPartialSuccess,
		models.TaskStatusReplanning,
		models.TaskStatusRollingBack,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusReplanning: {
		models.TaskStatusPlanning,
		models.TaskStatusWaitingApproval,
		models.TaskStatusExecuting,
		models.TaskStatusRollingBack,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusRollingBack: {
		models.TaskStatusRolledBack,
		models.TaskStatusRollbackFailed,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusWaitingForAgent: {
		models.TaskStatusObserving,
		models.TaskStatusExecuting,
		models.TaskStatusRollingBack,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},
}

type Machine struct {
	mu          sync.Mutex
	history     []*models.TaskTransition
	subscribers []func(transition *models.TaskTransition)
}

func NewMachine() *Machine {
	return &Machine{
		history: make([]*models.TaskTransition, 0),
	}
}

func (m *Machine) IsValidTransition(from, to models.TaskStatus) bool {
	if from == to {
		return true
	}
	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}
	for _, target := range allowed {
		if target == to {
			return true
		}
	}
	return false
}

func (m *Machine) Transition(task *models.Task, to models.TaskStatus, reason, triggeredBy string) (*models.TaskTransition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	from := task.Status
	if !m.IsValidTransition(from, to) {
		return nil, fmt.Errorf("illegal state transition from %s to %s", from, to)
	}

	task.Status = to
	task.UpdatedAt = time.Now()
	if to == models.TaskStatusCompleted || to == models.TaskStatusFailed || to == models.TaskStatusCancelled {
		now := time.Now()
		task.CompletedAt = &now
	}

	transition := &models.TaskTransition{
		TaskID:         task.ID,
		FromStatus:     from,
		ToStatus:       to,
		Reason:         reason,
		TriggeredBy:    triggeredBy,
		TransitionedAt: time.Now(),
	}

	m.history = append(m.history, transition)

	for _, sub := range m.subscribers {
		go sub(transition)
	}

	return transition, nil
}

func (m *Machine) Subscribe(callback func(transition *models.TaskTransition)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers = append(m.subscribers, callback)
}

func (m *Machine) GetHistory(taskID string) []*models.TaskTransition {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*models.TaskTransition
	for _, t := range m.history {
		if t.TaskID == taskID {
			result = append(result, t)
		}
	}
	return result
}
