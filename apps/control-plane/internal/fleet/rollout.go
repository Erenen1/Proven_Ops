package fleet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/models"
)

// Engine handles batching, canary steps, and fleet-wide execution bounds.
type Engine struct {
	store database.Store
	hub   *events.Hub
}

func NewEngine(store database.Store, hub *events.Hub) *Engine {
	return &Engine{
		store: store,
		hub:   hub,
	}
}

// ComputeBatches splits the target agents into sequential batches according to strategy.
func (e *Engine) ComputeBatches(agentIDs []string, cfg *models.RolloutConfig) [][]string {
	if len(agentIDs) == 0 {
		return nil
	}

	if cfg == nil || cfg.Strategy == models.RolloutAllAtOnce || cfg.Strategy == "" {
		return [][]string{agentIDs}
	}

	var batches [][]string

	if cfg.Strategy == models.RolloutCanary {
		canarySize := cfg.CanaryNodes
		if canarySize <= 0 {
			canarySize = 1
		}
		if canarySize > len(agentIDs) {
			canarySize = len(agentIDs)
		}

		// Canary batch
		batches = append(batches, agentIDs[:canarySize])

		// Remaining agents
		remaining := agentIDs[canarySize:]
		if len(remaining) > 0 {
			batchSize := cfg.BatchSize
			if batchSize <= 0 {
				batches = append(batches, remaining)
			} else {
				for i := 0; i < len(remaining); i += batchSize {
					end := i + batchSize
					if end > len(remaining) {
						end = len(remaining)
					}
					batches = append(batches, remaining[i:end])
				}
			}
		}
		return batches
	}

	// Rolling rollout
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	for i := 0; i < len(agentIDs); i += batchSize {
		end := i + batchSize
		if end > len(agentIDs) {
			end = len(agentIDs)
		}
		batches = append(batches, agentIDs[i:end])
	}

	return batches
}

// HostExecutor defines the callback to execute the plan on a single host.
type HostExecutor func(ctx context.Context, task *models.Task, agentID string) error

// ExecuteRollout drives multi-host execution across batches with blast-radius control.
func (e *Engine) ExecuteRollout(
	ctx context.Context,
	task *models.Task,
	executor HostExecutor,
) error {
	cfg := task.RolloutConfig
	if cfg == nil {
		cfg = &models.RolloutConfig{
			Strategy:    models.RolloutAllAtOnce,
			MaxFailures: 0,
		}
		task.RolloutConfig = cfg
	}

	batches := e.ComputeBatches(task.TargetAgentIDs, cfg)

	progress := &models.FleetRolloutProgress{
		TotalHosts:     len(task.TargetAgentIDs),
		CompletedHosts: 0,
		FailedHosts:    0,
		InFlightHosts:  0,
		PendingHosts:   len(task.TargetAgentIDs),
		Halted:         false,
		HostStates:     make(map[string]*models.HostExecutionState),
	}

	for _, aid := range task.TargetAgentIDs {
		progress.HostStates[aid] = &models.HostExecutionState{
			AgentID:   aid,
			Status:    models.TaskStatusCreated,
			StartedAt: time.Now(),
		}
	}
	task.RolloutProgress = progress
	_ = e.store.UpdateTask(ctx, task)

	var mu sync.Mutex

	for batchIdx, batch := range batches {
		// 1. Check if failure threshold exceeded before launching next batch
		mu.Lock()
		if cfg.MaxFailures > 0 && progress.FailedHosts >= cfg.MaxFailures {
			progress.Halted = true
			progress.HaltReason = fmt.Sprintf("Rollout halted: failure count (%d) reached threshold (%d)", progress.FailedHosts, cfg.MaxFailures)
			mu.Unlock()
			break
		}
		progress.InFlightHosts += len(batch)
		progress.PendingHosts -= len(batch)
		for _, aid := range batch {
			progress.HostStates[aid].Status = models.TaskStatusExecuting
		}
		mu.Unlock()

		e.hub.Publish(task.ID, "FLEET_ROLLOUT_PROGRESS", map[string]any{
			"task_id":   task.ID,
			"progress":  progress,
			"batch_idx": batchIdx + 1,
			"batches":   len(batches),
		})

		// 2. Execute current batch concurrently across fleet nodes
		var wg sync.WaitGroup
		for _, aid := range batch {
			wg.Add(1)
			go func(agentID string) {
				defer wg.Done()

				err := executor(ctx, task, agentID)

				mu.Lock()
				defer mu.Unlock()

				now := time.Now()
				state := progress.HostStates[agentID]
				state.FinishedAt = &now
				progress.InFlightHosts--

				if err != nil {
					state.Status = models.TaskStatusFailed
					state.ErrorMessage = err.Error()
					progress.FailedHosts++
				} else {
					state.Status = models.TaskStatusCompleted
					progress.CompletedHosts++
				}
			}(aid)
		}
		wg.Wait()

		_ = e.store.UpdateTask(ctx, task)
		e.hub.Publish(task.ID, "FLEET_ROLLOUT_PROGRESS", map[string]any{
			"task_id":   task.ID,
			"progress":  progress,
			"batch_idx": batchIdx + 1,
			"batches":   len(batches),
		})

		// 3. Canary Pause
		if cfg.PauseBetween > 0 && batchIdx < len(batches)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(cfg.PauseBetween) * time.Second):
			}
		}
	}

	// Final Status Assignment
	if progress.Halted {
		task.Status = models.TaskStatusFailed
		task.ErrorMessage = progress.HaltReason
	} else if progress.FailedHosts > 0 {
		if progress.CompletedHosts > 0 {
			task.Status = models.TaskStatusPartialSuccess
		} else {
			task.Status = models.TaskStatusFailed
		}
	} else {
		task.Status = models.TaskStatusCompleted
	}

	now := time.Now()
	task.CompletedAt = &now
	_ = e.store.UpdateTask(ctx, task)

	_ = e.store.SaveAuditEvent(ctx, &models.AuditEvent{
		TaskID:    task.ID,
		EventType: "FLEET_ROLLOUT_COMPLETED",
		Action:    "fleet_rollout",
		Details: map[string]any{
			"total":     progress.TotalHosts,
			"completed": progress.CompletedHosts,
			"failed":    progress.FailedHosts,
			"halted":    progress.Halted,
			"strategy":  cfg.Strategy,
		},
		CreatedAt: now,
	})

	return nil
}
