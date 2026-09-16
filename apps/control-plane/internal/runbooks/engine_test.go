package runbooks

import (
	"testing"

	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/policy"
)

func TestValidateAndResolveParameters(t *testing.T) {
	pe := policy.NewEngine()
	eng := NewEngine(pe)

	vars := []models.RunbookVariable{
		{Name: "port", Default: "8080", Required: true},
		{Name: "domain", Required: true},
		{Name: "optional_note", Default: "none", Required: false},
	}

	// Case 1: Missing required parameter with no default
	_, err := eng.ValidateAndResolveParameters(vars, map[string]string{"port": "9000"})
	if err == nil {
		t.Fatalf("expected error for missing required parameter 'domain', got nil")
	}

	// Case 2: Success with default and override
	resolved, err := eng.ValidateAndResolveParameters(vars, map[string]string{
		"domain": "example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved["port"] != "8080" {
		t.Errorf("expected port 8080, got %s", resolved["port"])
	}
	if resolved["domain"] != "example.com" {
		t.Errorf("expected domain example.com, got %s", resolved["domain"])
	}
	if resolved["optional_note"] != "none" {
		t.Errorf("expected optional_note none, got %s", resolved["optional_note"])
	}
}

func TestInterpolateStepAndCompilePlan(t *testing.T) {
	pe := policy.NewEngine()
	eng := NewEngine(pe)

	rb := &models.Runbook{
		ID:    "test-rb",
		Slug:  "test-slug",
		Title: "Configure Service on {{port}}",
		Variables: []models.RunbookVariable{
			{Name: "port", Default: "8080", Required: true},
			{Name: "service", Default: "nginx", Required: true},
		},
		Steps: []*models.AIStepPlan{
			{
				ID:        "s1",
				Action:    "write_config_file",
				Arguments: map[string]any{"path": "/etc/nginx/conf.d/{{port}}.conf", "content": "listen {{port}};"},
				Reason:    "Configure port {{port}} for {{service}}",
				VerificationStrategy: &models.VerificationStrategy{
					CheckType: "tcp_port_open",
					Target:    "{{port}}",
					Expected:  "open",
				},
			},
		},
		OverallVerification: []*models.VerificationStrategy{
			{CheckType: "tcp_port_open", Target: "{{port}}", Expected: "open"},
		},
	}

	plan, err := eng.CompilePlan(rb, map[string]string{"port": "8081", "service": "web-srv"})
	if err != nil {
		t.Fatalf("CompilePlan failed: %v", err)
	}

	if plan.Goal != "Configure Service on 8081" {
		t.Errorf("unexpected goal: %s", plan.Goal)
	}

	step := plan.Steps[0]
	if step.Arguments["path"] != "/etc/nginx/conf.d/8081.conf" {
		t.Errorf("step path not interpolated: %v", step.Arguments["path"])
	}
	if step.Arguments["content"] != "listen 8081;" {
		t.Errorf("step content not interpolated: %v", step.Arguments["content"])
	}
	if step.Reason != "Configure port 8081 for web-srv" {
		t.Errorf("step reason not interpolated: %s", step.Reason)
	}
	if step.VerificationStrategy.Target != "8081" {
		t.Errorf("verification target not interpolated: %s", step.VerificationStrategy.Target)
	}
	if plan.OverallVerification[0].Target != "8081" {
		t.Errorf("overall verification target not interpolated: %s", plan.OverallVerification[0].Target)
	}
}

func TestDryRunPolicyPassAndForbidden(t *testing.T) {
	pe := policy.NewEngine()
	eng := NewEngine(pe)

	// Safe runbook
	safeRb := &models.Runbook{
		ID:    "safe-rb",
		Slug:  "safe-rb",
		Title: "Inspect ports",
		Steps: []*models.AIStepPlan{
			{
				ID:        "s1",
				Action:    "get_open_ports",
				Arguments: map[string]any{},
			},
		},
	}

	res, err := eng.DryRun(safeRb, nil)
	if err != nil {
		t.Fatalf("DryRun error: %v", err)
	}
	if !res.PolicyPass {
		t.Errorf("expected policy pass for read only step")
	}
	if res.MaxRisk != models.RiskReadOnly {
		t.Errorf("expected max risk READ_ONLY, got %s", res.MaxRisk)
	}

	// Dangerous runbook containing forbidden command
	forbiddenRb := &models.Runbook{
		ID:    "forbidden-rb",
		Slug:  "forbidden-rb",
		Title: "Destructive cleanup",
		Steps: []*models.AIStepPlan{
			{
				ID:        "s1",
				Action:    "execute_command",
				Arguments: map[string]any{"command": "rm -rf /"},
			},
		},
	}

	resForbidden, err := eng.DryRun(forbiddenRb, nil)
	if err != nil {
		t.Fatalf("DryRun error: %v", err)
	}
	if resForbidden.PolicyPass {
		t.Errorf("expected forbidden command rm -rf / to fail policy check")
	}
	if resForbidden.MaxRisk != models.RiskForbidden {
		t.Errorf("expected max risk FORBIDDEN, got %s", resForbidden.MaxRisk)
	}
}

func TestDefaultRunbooksCompilation(t *testing.T) {
	pe := policy.NewEngine()
	eng := NewEngine(pe)

	runbooks := GetDefaultRunbooks()
	if len(runbooks) < 3 {
		t.Errorf("expected at least 3 default SRE runbooks, got %d", len(runbooks))
	}

	for _, rb := range runbooks {
		res, err := eng.DryRun(rb, nil)
		if err != nil {
			t.Fatalf("runbook %s dry run failed: %v", rb.Slug, err)
		}
		if !res.PolicyPass {
			t.Errorf("default runbook %s failed policy checks: %v", rb.Slug, res.Validations)
		}
		if res.Plan == nil || len(res.Plan.Steps) == 0 {
			t.Errorf("default runbook %s produced empty plan", rb.Slug)
		}
	}
}
