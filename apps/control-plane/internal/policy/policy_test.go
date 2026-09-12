package policy

import (
	"testing"

	"opspilot/control-plane/internal/models"
)

func TestPolicyEvaluation(t *testing.T) {
	engine := NewEngine()

	// 1. Read-only tool should have READ_ONLY risk and not require approval
	risk, needApproval, err := engine.Evaluate("get_os_info", nil, "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk != models.RiskReadOnly {
		t.Errorf("expected READ_ONLY, got %s", risk)
	}
	if needApproval {
		t.Errorf("read_only tool should not require approval")
	}

	// 2. Package install should require approval
	risk, needApproval, err = engine.Evaluate("install_package", map[string]any{"name": "nginx"}, "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk != models.RiskMedium {
		t.Errorf("expected MEDIUM risk for install_package, got %s", risk)
	}
	if !needApproval {
		t.Errorf("package install must require approval")
	}

	// 3. Destructive execute_command must be FORBIDDEN
	destructiveCmds := []string{
		"rm -rf /",
		"rm -rf /*",
		"mkfs.ext4 /dev/sda1",
		"shutdown -h now",
		":(){ :|:& };:",
	}
	for _, cmd := range destructiveCmds {
		risk, _, err = engine.Evaluate("execute_command", map[string]any{"command": cmd}, "development")
		if risk != models.RiskForbidden {
			t.Errorf("expected FORBIDDEN for '%s', got %s", cmd, risk)
		}
		if err == nil {
			t.Errorf("expected security error for '%s', got nil", cmd)
		}
	}
}
