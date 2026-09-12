package failures

import (
	"time"

	"opspilot/control-plane/internal/models"
)

type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:    2,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
	}
}

// IsRetryable determines whether an action should be retried based on failure classification and side-effects.
func (p *RetryPolicy) IsRetryable(failure *models.StructuredFailure, action string, currentAttempt int) bool {
	if currentAttempt >= p.MaxAttempts {
		return false
	}

	if failure == nil || !failure.Retryable {
		return false
	}

	// Never blindly retry mutating actions with potential irreversible side effects
	// unless it was purely a transient dependency network fetch failure (like apt update/fetch)
	switch action {
	case "write_config_file", "delete_file", "stop_service", "disable_service":
		return false
	case "install_package":
		return failure.Type == models.FailureDependencyFailure
	default:
		// Read-only or idempotent probes are safe to retry
		return true
	}
}

// GetBackoff calculates exponential backoff duration for given attempt.
func (p *RetryPolicy) GetBackoff(attempt int) time.Duration {
	backoff := p.InitialBackoff * time.Duration(1<<attempt)
	if backoff > p.MaxBackoff {
		backoff = p.MaxBackoff
	}
	return backoff
}
