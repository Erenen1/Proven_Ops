package failures

import (
	"testing"

	"opspilot/control-plane/internal/models"
)

func TestClassifyResourceConflict(t *testing.T) {
	sf := ClassifyExecutionFailure("restart_service", 1, "", "nginx: [emerg] bind() to 0.0.0.0:8080 failed (98: Address already in use)", "nginx")
	if sf.Type != models.FailureResourceConflict {
		t.Errorf("expected FailureResourceConflict, got %s", sf.Type)
	}
	if sf.Retryable {
		t.Errorf("resource conflict should not be retryable")
	}
	if !sf.RequiresReplan {
		t.Errorf("resource conflict should require replan")
	}
	if !sf.UserActionRequired {
		t.Errorf("resource conflict should require user decision")
	}
}

func TestClassifySyntaxValidationFailure(t *testing.T) {
	sf := ClassifyExecutionFailure("write_config_file", 1, "", "nginx: [emerg] unknown directive \"foobar\" in /etc/nginx/sites-available/default:2\nnginx: configuration file /etc/nginx/nginx.conf test failed", "/etc/nginx/sites-available/default")
	if sf.Type != models.FailureValidationFailed {
		t.Errorf("expected FailureValidationFailed, got %s", sf.Type)
	}
	if sf.Retryable {
		t.Errorf("syntax validation failure should not be retryable")
	}
	if !sf.RollbackRequired {
		t.Errorf("syntax validation failure must trigger rollback")
	}
}

func TestClassifyTransientNetworkFailure(t *testing.T) {
	sf := ClassifyExecutionFailure("install_package", 100, "", "E: Failed to fetch http://archive.ubuntu.com/pool/main/n/nginx.deb  Connection timed out", "nginx")
	if sf.Type != models.FailureDependencyFailure {
		t.Errorf("expected FailureDependencyFailure, got %s", sf.Type)
	}
	if !sf.Retryable {
		t.Errorf("transient network failure should be retryable")
	}

	policy := DefaultRetryPolicy()
	errClass := ClassifyStandardError("install_package", 100, "", "E: Failed to fetch http://archive.ubuntu.com/pool/main/n/nginx.deb  Connection timed out")
	if errClass != models.ErrorClassConnectivity && errClass != models.ErrorClassTransient {
		t.Errorf("expected Connectivity or Transient, got %s", errClass)
	}
	if !policy.IsRetryable(errClass, "install_package", 0) {
		t.Errorf("first attempt of transient package fetch should be retryable")
	}
	if policy.IsRetryable(errClass, "install_package", 3) {
		t.Errorf("attempt beyond MaxAttempts should not be retryable")
	}
}

func TestNonRetryableMutatingActions(t *testing.T) {
	policy := DefaultRetryPolicy()

	// Policy error should never be retried
	if policy.IsRetryable(models.ErrorClassPolicy, "write_config_file", 0) {
		t.Errorf("policy error must never be retried")
	}

	// Permanent error should never be retried
	if policy.IsRetryable(models.ErrorClassPermanent, "install_package", 0) {
		t.Errorf("permanent error must never be retried")
	}

	// Jitter backoff test
	b1 := policy.GetBackoffWithJitter(1)
	if b1 <= 0 {
		t.Errorf("expected positive backoff, got %v", b1)
	}
}
