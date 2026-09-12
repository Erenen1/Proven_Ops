package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"opspilot/agent/internal/tools"
)

type Runner struct {
	registry       *tools.Registry
	guard          *CommandGuard
	ledger         *Ledger
	executionCache sync.Map // fallback in-memory cache
}

func NewRunner(registry *tools.Registry, guard *CommandGuard) *Runner {
	ledger, _ := NewLedger("")
	return &Runner{
		registry: registry,
		guard:    guard,
		ledger:   ledger,
	}
}

func NewRunnerWithLedger(registry *tools.Registry, guard *CommandGuard, ledger *Ledger) *Runner {
	return &Runner{
		registry: registry,
		guard:    guard,
		ledger:   ledger,
	}
}

type StepExecutionResult struct {
	Action     string
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMS int64
	Success    bool
	Data       map[string]any
}

func (r *Runner) Execute(ctx context.Context, action string, argsJSON string, timeoutSec int) (*StepExecutionResult, error) {
	return r.ExecuteWithID(ctx, "", action, argsJSON, timeoutSec)
}

func (r *Runner) isMutatingAction(action string) bool {
	readOnly := map[string]bool{
		"get_os_info": true, "get_system_info": true, "get_cpu_usage": true,
		"get_memory_usage": true, "get_disk_usage": true, "get_load_average": true,
		"list_processes": true, "get_process_details": true, "get_service_status": true,
		"get_service_logs": true, "get_journal_logs": true, "check_package": true,
		"get_open_ports": true, "check_port": true, "http_probe": true, "dns_lookup": true,
		"read_file": true, "docker_info": true, "docker_ps": true, "docker_logs": true, "docker_inspect": true,
	}
	return !readOnly[action]
}

func (r *Runner) ExecuteWithID(ctx context.Context, executionID string, action string, argsJSON string, timeoutSec int) (*StepExecutionResult, error) {
	if executionID != "" {
		// 1. Check persistent ledger first
		if r.ledger != nil {
			rec, err := r.ledger.Get(executionID)
			if err == nil && rec != nil {
				switch rec.Status {
				case StatusSucceeded:
					if rec.Result != nil {
						copyData := make(map[string]any)
						for k, v := range rec.Result.Data {
							copyData[k] = v
						}
						copyData["idempotent"] = true
						copyData["cached_execution"] = true
						copyData["persistent_ledger"] = true
						return &StepExecutionResult{
							Action:     rec.Result.Action,
							ExitCode:   rec.Result.ExitCode,
							Stdout:     rec.Result.Stdout,
							Stderr:     rec.Result.Stderr,
							DurationMS: 0,
							Success:    true,
							Data:       copyData,
						}, nil
					}
				case StatusUnknown:
					// Process crashed mid-execution. For mutating action, DO NOT re-run blindly!
					if r.isMutatingAction(action) {
						return &StepExecutionResult{
							Action:     action,
							ExitCode:   1,
							Stderr:     "UNCERTAIN_EXECUTION: process crashed mid-execution; manual observation required before retry",
							DurationMS: 0,
							Success:    false,
							Data: map[string]any{
								"uncertain_execution": true,
								"status":              string(StatusUnknown),
								"action":              action,
							},
						}, nil
					}
				}
			}
		}

		// 2. Fallback in-memory cache check
		if val, ok := r.executionCache.Load(executionID); ok {
			cached := val.(*StepExecutionResult)
			copyData := make(map[string]any)
			for k, v := range cached.Data {
				copyData[k] = v
			}
			copyData["idempotent"] = true
			copyData["cached_execution"] = true
			return &StepExecutionResult{
				Action:     cached.Action,
				ExitCode:   cached.ExitCode,
				Stdout:     cached.Stdout,
				Stderr:     cached.Stderr,
				DurationMS: 0,
				Success:    cached.Success,
				Data:       copyData,
			}, nil
		}
	}
	start := time.Now()

	// Record start in persistent ledger
	if executionID != "" && r.ledger != nil {
		parts := strings.Split(executionID, "/")
		taskID := ""
		stepID := ""
		if len(parts) >= 2 {
			taskID = parts[0]
			stepID = parts[1]
		}
		_ = r.ledger.RecordStart(executionID, taskID, stepID, action)
	}

	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("invalid arguments JSON: %v", err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 1. Check if typed tool exists
	if tool, err := r.registry.Get(action); err == nil {
		res, err := tool.Execute(ctxWithTimeout, args)
		duration := time.Since(start).Milliseconds()
		if err != nil {
			return &StepExecutionResult{
				Action:     action,
				ExitCode:   1,
				Stderr:     err.Error(),
				DurationMS: duration,
				Success:    false,
			}, nil
		}
		out := &StepExecutionResult{
			Action:     action,
			ExitCode:   res.ExitCode,
			Stdout:     res.Stdout,
			Stderr:     res.Stderr,
			DurationMS: duration,
			Success:    res.Success,
			Data:       res.Data,
		}
		if executionID != "" {
			if res.Success {
				r.executionCache.Store(executionID, out)
				if r.ledger != nil {
					_ = r.ledger.RecordCompletion(executionID, StatusSucceeded, out)
				}
			} else {
				if r.ledger != nil {
					_ = r.ledger.RecordCompletion(executionID, StatusFailed, out)
				}
			}
		}
		return out, nil
	}

	// 2. Handle raw execute_command fallback
	if action == "execute_command" {
		cmdStr, _ := args["command"].(string)
		if err := r.guard.Validate(cmdStr); err != nil {
			duration := time.Since(start).Milliseconds()
			secOut := &StepExecutionResult{
				Action:     action,
				ExitCode:   126, // Command invoked cannot execute (security rejection)
				Stderr:     fmt.Sprintf("Command blocked by security policy: %v", err),
				DurationMS: duration,
				Success:    false,
			}
			if executionID != "" && r.ledger != nil {
				_ = r.ledger.RecordCompletion(executionID, StatusFailed, secOut)
			}
			return secOut, nil
		}

		cmd := exec.CommandContext(ctxWithTimeout, "bash", "-c", cmdStr)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		duration := time.Since(start).Milliseconds()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		out := &StepExecutionResult{
			Action:     action,
			ExitCode:   exitCode,
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			DurationMS: duration,
			Success:    exitCode == 0,
		}
		if executionID != "" {
			if exitCode == 0 {
				r.executionCache.Store(executionID, out)
				if r.ledger != nil {
					_ = r.ledger.RecordCompletion(executionID, StatusSucceeded, out)
				}
			} else {
				if r.ledger != nil {
					_ = r.ledger.RecordCompletion(executionID, StatusFailed, out)
				}
			}
		}
		return out, nil
	}

	return nil, fmt.Errorf("unsupported action: %s", action)
}
