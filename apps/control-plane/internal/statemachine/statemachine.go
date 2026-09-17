package statemachine

import (
	"fmt"
	"sync"
	"time"

	"opspilot/control-plane/internal/models"
)

var validTransitions = map[models.TaskStatus][]models.TaskStatus{
	// 1. PENDING / CREATED
	models.TaskStatusPending: {
		models.TaskStatusPrechecking,
		models.TaskStatusDiscovering,
		models.TaskStatusPlanning,
		models.TaskStatusWaitingApproval,
		models.TaskStatusQueued,
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},

	// 2. PRECHECKING
	models.TaskStatusPrechecking: {
		models.TaskStatusSkipped,
		models.TaskStatusWaitingApproval,
		models.TaskStatusQueued,
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},

	// 3. SKIPPED (Idempotent already satisfied)
	models.TaskStatusSkipped: {
		models.TaskStatusSucceeded,
	},

	// 4. WAITING_APPROVAL
	models.TaskStatusWaitingApproval: {
		models.TaskStatusQueued,
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},

	// 5. QUEUED
	models.TaskStatusQueued: {
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},

	// 6. DISPATCHED
	models.TaskStatusDispatched: {
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusRetrying,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},

	// 7. RUNNING / EXECUTING
	models.TaskStatusRunning: {
		models.TaskStatusObserving,
		models.TaskStatusVerifying,
		models.TaskStatusRetrying,
		models.TaskStatusCompensating,
		models.TaskStatusRollingBack,
		models.TaskStatusWaitingForAgent,
		models.TaskStatusManualInterventionRequired,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},

	// 8. VERIFYING
	models.TaskStatusVerifying: {
		models.TaskStatusSucceeded,
		models.TaskStatusCompleted,
		models.TaskStatusPartialSuccess,
		models.TaskStatusReplanning,
		models.TaskStatusRetrying,
		models.TaskStatusCompensating,
		models.TaskStatusRollingBack,
		models.TaskStatusManualInterventionRequired,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},

	// 9. RETRYING
	models.TaskStatusRetrying: {
		models.TaskStatusQueued,
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusCompensating,
		models.TaskStatusManualInterventionRequired,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},

	// 10. COMPENSATING / ROLLING_BACK
	models.TaskStatusCompensating: {
		models.TaskStatusCompensated,
		models.TaskStatusPartiallyCompensated,
		models.TaskStatusManualInterventionRequired,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},

	// 11. COMPENSATED / ROLLED_BACK
	models.TaskStatusCompensated: {},

	// 12. PARTIALLY_COMPENSATED / ROLLBACK_FAILED
	models.TaskStatusPartiallyCompensated: {
		models.TaskStatusManualInterventionRequired,
	},

	// 13. MANUAL_INTERVENTION_REQUIRED
	models.TaskStatusManualInterventionRequired: {
		models.TaskStatusRetrying,
		models.TaskStatusCompensating,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},

	// 14. SUCCEEDED / COMPLETED
	models.TaskStatusSucceeded: {},

	// 15. FAILED
	models.TaskStatusFailed: {
		models.TaskStatusCompensating,
		models.TaskStatusManualInterventionRequired,
	},

	// 16. CANCELLED
	models.TaskStatusCancelled: {
		models.TaskStatusCompensating,
	},

	// Discovery & Planning intermediate states (Backwards compatibility)
	models.TaskStatusCreated: {
		models.TaskStatusPrechecking,
		models.TaskStatusDiscovering,
		models.TaskStatusPlanning,
		models.TaskStatusWaitingApproval,
		models.TaskStatusQueued,
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusCancelled,
		models.TaskStatusFailed,
	},
	models.TaskStatusExecuting: {
		models.TaskStatusObserving,
		models.TaskStatusVerifying,
		models.TaskStatusRetrying,
		models.TaskStatusCompensating,
		models.TaskStatusRollingBack,
		models.TaskStatusWaitingForAgent,
		models.TaskStatusManualInterventionRequired,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},
	models.TaskStatusRollingBack: {
		models.TaskStatusRolledBack,
		models.TaskStatusRollbackFailed,
		models.TaskStatusCompensated,
		models.TaskStatusPartiallyCompensated,
		models.TaskStatusManualInterventionRequired,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusWaitingForAgent: {
		models.TaskStatusObserving,
		models.TaskStatusExecuting,
		models.TaskStatusRunning,
		models.TaskStatusRollingBack,
		models.TaskStatusCompensating,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},
	models.TaskStatusDiscovering: {
		models.TaskStatusPlanning,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
		models.TaskStatusTimeout,
	},
	models.TaskStatusPlanning: {
		models.TaskStatusWaitingApproval,
		models.TaskStatusQueued,
		models.TaskStatusDispatched,
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusObserving: {
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusVerifying,
		models.TaskStatusReplanning,
		models.TaskStatusCompensating,
		models.TaskStatusRollingBack,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	},
	models.TaskStatusReplanning: {
		models.TaskStatusPlanning,
		models.TaskStatusWaitingApproval,
		models.TaskStatusRunning,
		models.TaskStatusExecuting,
		models.TaskStatusCompensating,
		models.TaskStatusRollingBack,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
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
	task.OptimisticLockVersion++
	task.UpdatedAt = time.Now()
	if to == models.TaskStatusCompleted || to == models.TaskStatusSucceeded || to == models.TaskStatusFailed || to == models.TaskStatusCancelled {
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
