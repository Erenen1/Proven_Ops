package runbooks

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/policy"
)

var paramRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_-]+)\s*\}\}`)

// Engine handles validation, parameter templating, and policy evaluation for runbooks.
type Engine struct {
	policyEngine *policy.Engine
}

func NewEngine(policyEngine *policy.Engine) *Engine {
	return &Engine{policyEngine: policyEngine}
}

// ValidateAndResolveParameters merges user inputs with runbook variable defaults and validates required fields.
func (e *Engine) ValidateAndResolveParameters(vars []models.RunbookVariable, inputs map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)

	for _, v := range vars {
		val, ok := inputs[v.Name]
		if !ok || strings.TrimSpace(val) == "" {
			if v.Required && v.Default == "" {
				return nil, fmt.Errorf("missing required runbook parameter: %s (%s)", v.Name, v.Description)
			}
			resolved[v.Name] = v.Default
		} else {
			resolved[v.Name] = strings.TrimSpace(val)
		}
	}

	// Also allow arbitrary extra inputs passed by operator
	for k, v := range inputs {
		if _, exists := resolved[k]; !exists {
			resolved[k] = v
		}
	}

	return resolved, nil
}

// InterpolateText replaces all {{variable}} placeholders with resolved parameter values.
func (e *Engine) InterpolateText(text string, params map[string]string) string {
	return paramRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := paramRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			varName := submatches[1]
			if val, ok := params[varName]; ok {
				return val
			}
		}
		return match
	})
}

// InterpolateStep deep-interpolates arguments, reason, and verification target/expected.
func (e *Engine) InterpolateStep(step *models.AIStepPlan, params map[string]string) (*models.AIStepPlan, error) {
	interpolated := &models.AIStepPlan{
		ID:            step.ID,
		Action:        step.Action,
		Reason:        e.InterpolateText(step.Reason, params),
		SuggestedRisk: step.SuggestedRisk,
	}

	if step.Arguments != nil {
		argsBytes, err := json.Marshal(step.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal step arguments: %w", err)
		}
		interpolatedJSON := e.InterpolateText(string(argsBytes), params)
		var interpolatedArgs map[string]any
		if err := json.Unmarshal([]byte(interpolatedJSON), &interpolatedArgs); err != nil {
			return nil, fmt.Errorf("failed to parse interpolated step arguments: %w", err)
		}
		interpolated.Arguments = interpolatedArgs
	}

	if step.VerificationStrategy != nil {
		interpolated.VerificationStrategy = &models.VerificationStrategy{
			CheckType:  step.VerificationStrategy.CheckType,
			Target:     e.InterpolateText(step.VerificationStrategy.Target, params),
			Expected:   e.InterpolateText(step.VerificationStrategy.Expected, params),
			TimeoutSec: step.VerificationStrategy.TimeoutSec,
		}
	}

	return interpolated, nil
}

// CompilePlan converts a Runbook and resolved parameters into a runnable models.AIPlanData.
func (e *Engine) CompilePlan(rb *models.Runbook, inputs map[string]string) (*models.AIPlanData, error) {
	resolvedParams, err := e.ValidateAndResolveParameters(rb.Variables, inputs)
	if err != nil {
		return nil, err
	}

	plan := &models.AIPlanData{
		Goal:      e.InterpolateText(rb.Title, resolvedParams),
		Reasoning: fmt.Sprintf("Runbook %s executed with parameters %v", rb.Slug, resolvedParams),
	}

	for _, st := range rb.Steps {
		interpolatedStep, err := e.InterpolateStep(st, resolvedParams)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate step %s: %w", st.ID, err)
		}
		plan.Steps = append(plan.Steps, interpolatedStep)
	}

	for _, ov := range rb.OverallVerification {
		plan.OverallVerification = append(plan.OverallVerification, &models.VerificationStrategy{
			CheckType:  ov.CheckType,
			Target:     e.InterpolateText(ov.Target, resolvedParams),
			Expected:   e.InterpolateText(ov.Expected, resolvedParams),
			TimeoutSec: ov.TimeoutSec,
		})
	}

	return plan, nil
}

// DryRun evaluates runbook execution without modifying any host state.
func (e *Engine) DryRun(rb *models.Runbook, inputs map[string]string) (*models.RunbookExecutionResult, error) {
	plan, err := e.CompilePlan(rb, inputs)
	if err != nil {
		return &models.RunbookExecutionResult{
			DryRun:     true,
			PolicyPass: false,
			Error:      err.Error(),
		}, nil
	}

	res := &models.RunbookExecutionResult{
		DryRun:      true,
		Plan:        plan,
		PolicyPass:  true,
		MaxRisk:     models.RiskReadOnly,
		Validations: make([]string, 0),
	}

	riskWeight := map[models.RiskLevel]int{
		models.RiskReadOnly:  0,
		models.RiskLow:       1,
		models.RiskMedium:    2,
		models.RiskHigh:      3,
		models.RiskForbidden: 4,
	}

	for _, step := range plan.Steps {
		risk, _, err := e.policyEngine.Evaluate(step.Action, step.Arguments, "standard")
		if err != nil {
			res.PolicyPass = false
			res.MaxRisk = models.RiskForbidden
			res.Validations = append(res.Validations, fmt.Sprintf("Step %s (%s) failed policy evaluation: %v", step.ID, step.Action, err))
			continue
		}

		if riskWeight[risk] > riskWeight[res.MaxRisk] {
			res.MaxRisk = risk
		}

		if risk == models.RiskForbidden {
			res.PolicyPass = false
			res.Validations = append(res.Validations, fmt.Sprintf("Step %s (%s) is FORBIDDEN by security policy", step.ID, step.Action))
		} else {
			res.Validations = append(res.Validations, fmt.Sprintf("Step %s (%s) passed policy check [Risk: %s]", step.ID, step.Action, risk))
		}
	}

	return res, nil
}

// GetDefaultRunbooks returns production SRE standard runbooks.
func GetDefaultRunbooks() []*models.Runbook {
	now := time.Now()
	return []*models.Runbook{
		{
			ID:          "rb-sre-nginx-deploy",
			Slug:        "nginx-custom-port-deploy",
			Title:       "Deploy Nginx Web Server on Port {{port}}",
			Description: "Idempotently installs Nginx, applies virtual host config listening on {{port}}, and verifies HTTP 200 response.",
			Variables: []models.RunbookVariable{
				{
					Name:        "port",
					Description: "Target TCP port for Nginx web server",
					Default:     "8080",
					Required:    true,
				},
				{
					Name:        "server_name",
					Description: "Nginx server_name virtual host directive",
					Default:     "_",
					Required:    false,
				},
			},
			Steps: []*models.AIStepPlan{
				{
					ID:            "step-1",
					Action:        "install_package",
					Arguments:     map[string]any{"name": "nginx"},
					Reason:        "Ensure nginx web server package is installed",
					SuggestedRisk: "MEDIUM",
					VerificationStrategy: &models.VerificationStrategy{
						CheckType:  "package_installed",
						Target:     "nginx",
						Expected:   "installed",
						TimeoutSec: 30,
					},
				},
				{
					ID:     "step-2",
					Action: "write_config_file",
					Arguments: map[string]any{
						"path":    "/etc/nginx/sites-available/default",
						"content": "server {\n    listen {{port}} default_server;\n    listen [::]:{{port}} default_server;\n    root /var/www/html;\n    index index.html index.nginx-debian.html;\n    server_name {{server_name}};\n    location / {\n        try_files $uri $uri/ =404;\n    }\n}\n",
					},
					Reason:        "Apply custom port configuration",
					SuggestedRisk: "MEDIUM",
				},
				{
					ID:            "step-3",
					Action:        "restart_service",
					Arguments:     map[string]any{"name": "nginx"},
					Reason:        "Restart nginx to bind custom port",
					SuggestedRisk: "MEDIUM",
					VerificationStrategy: &models.VerificationStrategy{
						CheckType:  "systemd_active",
						Target:     "nginx",
						Expected:   "active",
						TimeoutSec: 15,
					},
				},
			},
			OverallVerification: []*models.VerificationStrategy{
				{CheckType: "systemd_active", Target: "nginx", Expected: "active", TimeoutSec: 10},
				{CheckType: "tcp_port_open", Target: "{{port}}", Expected: "open", TimeoutSec: 10},
				{CheckType: "http_probe", Target: "http://127.0.0.1:{{port}}", Expected: "200", TimeoutSec: 10},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          "rb-sre-disk-cleanup",
			Slug:        "sre-disk-log-cleanup",
			Title:       "System Disk & Journal Log Pressure Remediation",
			Description: "Vacuums journalctl logs to {{vacuum_size}} and clears apt package cache to relieve disk pressure.",
			Variables: []models.RunbookVariable{
				{
					Name:        "vacuum_size",
					Description: "Target max retention size for journal logs",
					Default:     "100M",
					Required:    true,
				},
			},
			Steps: []*models.AIStepPlan{
				{
					ID:            "step-1",
					Action:        "execute_command",
					Arguments:     map[string]any{"command": "journalctl --vacuum-size={{vacuum_size}}"},
					Reason:        "Vacuum systemd journal archive logs to recover disk space",
					SuggestedRisk: "LOW",
				},
				{
					ID:            "step-2",
					Action:        "execute_command",
					Arguments:     map[string]any{"command": "apt-get clean"},
					Reason:        "Clean downloaded deb package archives from local cache",
					SuggestedRisk: "LOW",
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          "rb-sre-service-restart",
			Slug:        "sre-service-restart-diagnose",
			Title:       "Safely Diagnose & Restart Service {{service_name}}",
			Description: "Inspects status, restarts systemd service {{service_name}}, and deterministically verifies active state.",
			Variables: []models.RunbookVariable{
				{
					Name:        "service_name",
					Description: "Systemd unit name to restart and verify",
					Default:     "nginx",
					Required:    true,
				},
			},
			Steps: []*models.AIStepPlan{
				{
					ID:            "step-1",
					Action:        "get_service_status",
					Arguments:     map[string]any{"name": "{{service_name}}"},
					Reason:        "Inspect current systemd unit status and uptime",
					SuggestedRisk: "READ_ONLY",
				},
				{
					ID:            "step-2",
					Action:        "restart_service",
					Arguments:     map[string]any{"name": "{{service_name}}"},
					Reason:        "Restart systemd unit",
					SuggestedRisk: "MEDIUM",
					VerificationStrategy: &models.VerificationStrategy{
						CheckType:  "systemd_active",
						Target:     "{{service_name}}",
						Expected:   "active",
						TimeoutSec: 15,
					},
				},
			},
			OverallVerification: []*models.VerificationStrategy{
				{CheckType: "systemd_active", Target: "{{service_name}}", Expected: "active", TimeoutSec: 10},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// Helper to generate UUIDs
func NewUUID() string {
	return uuid.New().String()
}
