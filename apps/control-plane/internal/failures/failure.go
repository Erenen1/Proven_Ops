package failures

import (
	"fmt"
	"strings"

	"opspilot/control-plane/internal/models"
)

// ClassifyExecutionFailure inspects exit code, action, and output streams to produce a structured failure.
func ClassifyExecutionFailure(action string, exitCode int, stdout, stderr string, target string) *models.StructuredFailure {
	combined := strings.ToLower(stdout + "\n" + stderr)

	// 0. Agent disconnection
	if strings.Contains(combined, "agent disconnected") || strings.Contains(combined, "connection closed") || strings.Contains(combined, "stream terminated") {
		return &models.StructuredFailure{
			Type:               models.FailureAgentDisconnected,
			Action:             action,
			Target:             target,
			Message:            "Agent disconnected during execution",
			Retryable:          false,
			RequiresReplan:     false,
			RollbackRequired:   false,
			UserActionRequired: false,
			Details: map[string]any{
				"exit_code": exitCode,
				"error":     "AGENT_DISCONNECTED",
			},
		}
	}

	// 1. Resource conflicts (e.g. port already bound)
	if strings.Contains(combined, "address already in use") ||
		strings.Contains(combined, "bind() to") ||
		strings.Contains(combined, "port is already allocated") ||
		strings.Contains(combined, "already bound") {
		return &models.StructuredFailure{
			Type:               models.FailureResourceConflict,
			Action:             action,
			Target:             target,
			Message:            fmt.Sprintf("Resource conflict detected during '%s': port or socket already in use", action),
			Retryable:          false,
			RequiresReplan:     true,
			RollbackRequired:   false,
			UserActionRequired: true,
			Details: map[string]any{
				"exit_code": exitCode,
				"error":     "RESOURCE_CONFLICT",
			},
		}
	}

	// 2. Configuration syntax / validation failures (e.g. nginx -t failed)
	if strings.Contains(combined, "nginx: [emerg]") ||
		strings.Contains(combined, "configuration file") && strings.Contains(combined, "test failed") ||
		strings.Contains(combined, "syntax error") ||
		strings.Contains(combined, "unknown directive") ||
		strings.Contains(combined, "invalid configuration") {
		return &models.StructuredFailure{
			Type:               models.FailureValidationFailed,
			Action:             action,
			Target:             target,
			Message:            fmt.Sprintf("Configuration validation failed during '%s'", action),
			Retryable:          false,
			RequiresReplan:     true,
			RollbackRequired:   true,
			UserActionRequired: false,
			Details: map[string]any{
				"exit_code": exitCode,
				"error":     "SYNTAX_VALIDATION_FAILED",
			},
		}
	}

	// 3. Transient Dependency / Network Failures
	if strings.Contains(combined, "failed to fetch") ||
		strings.Contains(combined, "could not resolve") ||
		strings.Contains(combined, "temporary failure resolving") ||
		strings.Contains(combined, "connection timed out") ||
		strings.Contains(combined, "network is unreachable") ||
		strings.Contains(combined, "connection refused") && action == "http_probe" {
		return &models.StructuredFailure{
			Type:               models.FailureDependencyFailure,
			Action:             action,
			Target:             target,
			Message:            fmt.Sprintf("Transient dependency/network failure during '%s'", action),
			Retryable:          true,
			RequiresReplan:     false,
			RollbackRequired:   false,
			UserActionRequired: false,
			Details: map[string]any{
				"exit_code": exitCode,
				"error":     "TRANSIENT_NETWORK_FAILURE",
			},
		}
	}

	// 4. Permission / privilege issues
	if strings.Contains(combined, "permission denied") ||
		strings.Contains(combined, "are you root") ||
		strings.Contains(combined, "operation not permitted") ||
		strings.Contains(combined, "access denied") {
		return &models.StructuredFailure{
			Type:               models.FailurePermissionDenied,
			Action:             action,
			Target:             target,
			Message:            fmt.Sprintf("Permission denied during '%s'", action),
			Retryable:          false,
			RequiresReplan:     false,
			RollbackRequired:   false,
			UserActionRequired: true,
			Details: map[string]any{
				"exit_code": exitCode,
				"error":     "PERMISSION_DENIED",
			},
		}
	}

	// 5. Default generic execution failure
	return &models.StructuredFailure{
		Type:               models.FailureExecutionFailed,
		Action:             action,
		Target:             target,
		Message:            fmt.Sprintf("Command failed during '%s' with exit code %d", action, exitCode),
		Retryable:          false,
		RequiresReplan:     true,
		RollbackRequired:   false,
		UserActionRequired: false,
		Details: map[string]any{
			"exit_code": exitCode,
		},
	}
}

// ClassifyVerificationFailure formats an explicit verification failure.
func ClassifyVerificationFailure(checkType, target, details string) *models.StructuredFailure {
	return &models.StructuredFailure{
		Type:               models.FailureVerificationFailed,
		Action:             checkType,
		Target:             target,
		Message:            fmt.Sprintf("Deterministic verification failed for '%s' on '%s': %s", checkType, target, details),
		Retryable:          false,
		RequiresReplan:     true,
		RollbackRequired:   true,
		UserActionRequired: false,
		Details: map[string]any{
			"check_type": checkType,
			"target":     target,
			"details":    details,
		},
	}
}
