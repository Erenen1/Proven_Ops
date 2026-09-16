package failures

import (
	"math/rand"
	"strings"
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
		MaxAttempts:    3,
		InitialBackoff: 300 * time.Millisecond,
		MaxBackoff:     3 * time.Second,
	}
}

// ClassifyStandardError maps execution details to the standard 7 error classifications:
// TRANSIENT, PERMANENT, POLICY, VERIFICATION, CONNECTIVITY, TIMEOUT, UNKNOWN
func ClassifyStandardError(action string, exitCode int, stdout, stderr string) string {
	combined := strings.ToLower(stdout + "\n" + stderr)

	// Connectivity and network fetch failures
	if strings.Contains(combined, "failed to fetch") || strings.Contains(combined, "could not resolve") ||
		strings.Contains(combined, "connection refused") || strings.Contains(combined, "network is unreachable") ||
		strings.Contains(combined, "agent disconnected") || strings.Contains(combined, "broken pipe") ||
		strings.Contains(combined, "temporary failure in name resolution") || strings.Contains(combined, "could not resolve host") {
		return models.ErrorClassConnectivity
	}

	// Timeout
	if strings.Contains(combined, "timeout") || strings.Contains(combined, "timed out") || strings.Contains(combined, "deadline exceeded") {
		return models.ErrorClassTimeout
	}

	// Policy denial
	if strings.Contains(combined, "policy denied") || strings.Contains(combined, "forbidden") ||
		strings.Contains(combined, "command blocked by ast guard") || strings.Contains(combined, "permission denied") {
		return models.ErrorClassPolicy
	}

	// Verification
	if strings.Contains(combined, "verification failed") || strings.Contains(combined, "syntax error") ||
		strings.Contains(combined, "test failed") || strings.Contains(combined, "expected") && strings.Contains(combined, "actual") {
		return models.ErrorClassVerification
	}

	// Transient
	if strings.Contains(combined, "resource temporarily unavailable") || strings.Contains(combined, "try again") ||
		strings.Contains(combined, "lock is held") || strings.Contains(combined, "could not get lock /var/lib/dpkg/lock") ||
		strings.Contains(combined, "temporarily unable to satisfy") {
		return models.ErrorClassTransient
	}

	// Permanent
	if strings.Contains(combined, "no such file or directory") || strings.Contains(combined, "command not found") ||
		strings.Contains(combined, "package not found") || strings.Contains(combined, "illegal package name") ||
		strings.Contains(combined, "invalid argument") || strings.Contains(combined, "unsupported operation") {
		return models.ErrorClassPermanent
	}

	return models.ErrorClassUnknown
}

// IsRetryable determines whether an action should be retried based on error classification
func (p *RetryPolicy) IsRetryable(errorClass, action string, currentAttempt int) bool {
	if currentAttempt >= p.MaxAttempts {
		return false
	}

	// Policy and Permanent failures must NEVER be retried
	if errorClass == models.ErrorClassPolicy || errorClass == models.ErrorClassPermanent {
		return false
	}

	// Transient and Connectivity failures can be retried
	if errorClass == models.ErrorClassTransient || errorClass == models.ErrorClassConnectivity {
		return true
	}

	// Read-only probes with timeout or unknown error can be retried
	if (errorClass == models.ErrorClassTimeout || errorClass == models.ErrorClassUnknown) && isReadOnlyProbe(action) {
		return true
	}

	return false
}

func isReadOnlyProbe(action string) bool {
	switch action {
	case "check_port", "http_probe", "get_service_status", "check_package", "ensure_port_state", "get_os_info":
		return true
	default:
		return false
	}
}

// GetBackoff calculates backoff for given attempt
func (p *RetryPolicy) GetBackoff(attempt int) time.Duration {
	return p.GetBackoffWithJitter(attempt)
}

// GetBackoffWithJitter calculates exponential backoff with random jitter (±20%)
func (p *RetryPolicy) GetBackoffWithJitter(attempt int) time.Duration {
	base := p.InitialBackoff * time.Duration(1<<attempt)
	if base > p.MaxBackoff {
		base = p.MaxBackoff
	}

	// Apply jitter between 0.8 and 1.2
	jitterRatio := 0.8 + rand.Float64()*0.4
	duration := time.Duration(float64(base) * jitterRatio)
	if duration < 50*time.Millisecond {
		duration = 50 * time.Millisecond
	}
	return duration
}

