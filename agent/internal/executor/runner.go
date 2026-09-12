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
	executionCache sync.Map // executionID -> *StepExecutionResult
}

func NewRunner(registry *tools.Registry, guard *CommandGuard) *Runner {
	return &Runner{
		registry: registry,
		guard:    guard,
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

func (r *Runner) ExecuteWithID(ctx context.Context, executionID string, action string, argsJSON string, timeoutSec int) (*StepExecutionResult, error) {
	if executionID != "" {
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
		if executionID != "" && res.Success {
			r.executionCache.Store(executionID, out)
		}
		return out, nil
	}

	// 2. Handle raw execute_command fallback
	if action == "execute_command" {
		cmdStr, _ := args["command"].(string)
		if err := r.guard.Validate(cmdStr); err != nil {
			duration := time.Since(start).Milliseconds()
			return &StepExecutionResult{
				Action:     action,
				ExitCode:   126, // Command invoked cannot execute (security rejection)
				Stderr:     fmt.Sprintf("Command blocked by security policy: %v", err),
				DurationMS: duration,
				Success:    false,
			}, nil
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
		if executionID != "" && exitCode == 0 {
			r.executionCache.Store(executionID, out)
		}
		return out, nil
	}

	return nil, fmt.Errorf("unsupported action: %s", action)
}
