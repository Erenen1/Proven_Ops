package policy

import (
	"fmt"
	"strings"

	"opspilot/control-plane/internal/models"
)

type ToolMetadata struct {
	Name                  string
	Description           string
	DefaultRisk           models.RiskLevel
	ReadOnly              bool
	RequiresApproval      bool
	SupportedCapabilities []string
	TimeoutSec            int
	VerificationType      string
}

type Engine struct {
	toolRegistry map[string]*ToolMetadata
	policies     []*models.Policy
}

func NewEngine() *Engine {
	e := &Engine{
		toolRegistry: make(map[string]*ToolMetadata),
		policies:     make([]*models.Policy, 0),
	}
	e.registerDefaultTools()
	e.registerDefaultPolicies()
	return e
}

func (e *Engine) registerDefaultTools() {
	readOnlyTools := []string{
		"get_os_info", "get_system_info", "get_cpu_usage", "get_memory_usage",
		"get_disk_usage", "get_load_average", "list_processes", "get_process_details",
		"get_service_status", "get_service_logs", "get_journal_logs", "check_package",
		"get_open_ports", "check_port", "http_probe", "dns_lookup", "read_file",
		"docker_info", "docker_ps", "docker_logs", "docker_inspect",
	}

	for _, name := range readOnlyTools {
		e.toolRegistry[name] = &ToolMetadata{
			Name:             name,
			DefaultRisk:      models.RiskReadOnly,
			ReadOnly:         true,
			RequiresApproval: false,
			TimeoutSec:       15,
		}
	}

	e.toolRegistry["start_service"] = &ToolMetadata{
		Name:             "start_service",
		DefaultRisk:      models.RiskLow,
		ReadOnly:         false,
		RequiresApproval: false,
		TimeoutSec:       30,
		VerificationType: "systemd_active",
	}

	e.toolRegistry["stop_service"] = &ToolMetadata{
		Name:             "stop_service",
		DefaultRisk:      models.RiskMedium,
		ReadOnly:         false,
		RequiresApproval: true,
		TimeoutSec:       30,
	}

	e.toolRegistry["restart_service"] = &ToolMetadata{
		Name:             "restart_service",
		DefaultRisk:      models.RiskMedium,
		ReadOnly:         false,
		RequiresApproval: true,
		TimeoutSec:       30,
		VerificationType: "systemd_active",
	}

	e.toolRegistry["enable_service"] = &ToolMetadata{
		Name:             "enable_service",
		DefaultRisk:      models.RiskLow,
		ReadOnly:         false,
		RequiresApproval: false,
		TimeoutSec:       15,
	}

	e.toolRegistry["install_package"] = &ToolMetadata{
		Name:             "install_package",
		DefaultRisk:      models.RiskMedium,
		ReadOnly:         false,
		RequiresApproval: true,
		TimeoutSec:       120,
		VerificationType: "package_installed",
	}

	e.toolRegistry["write_config_file"] = &ToolMetadata{
		Name:             "write_config_file",
		DefaultRisk:      models.RiskMedium,
		ReadOnly:         false,
		RequiresApproval: true,
		TimeoutSec:       20,
		VerificationType: "file_exists",
	}

	e.toolRegistry["execute_command"] = &ToolMetadata{
		Name:             "execute_command",
		DefaultRisk:      models.RiskHigh,
		ReadOnly:         false,
		RequiresApproval: true,
		TimeoutSec:       60,
	}
}

func (e *Engine) registerDefaultPolicies() {
	e.policies = append(e.policies,
		&models.Policy{
			ID:               "pol-1",
			Name:             "Auto-approve Read Only Tools",
			Scope:            "global",
			ActionPattern:    "get_*,check_*,read_*,http_probe,docker_ps,docker_logs,docker_info",
			MaxRiskLevel:     models.RiskReadOnly,
			RequiresApproval: false,
			IsEnabled:        true,
		},
		&models.Policy{
			ID:               "pol-2",
			Name:             "Production Package Install Guard",
			Scope:            "environment",
			ScopeValue:       "production",
			ActionPattern:    "install_package",
			MaxRiskLevel:     models.RiskMedium,
			RequiresApproval: true,
			IsEnabled:        true,
		},
	)
}

func (e *Engine) IsDestructiveCommand(cmd string) (bool, string) {
	cmdLower := strings.ToLower(strings.TrimSpace(cmd))
	destructivePrefixes := []string{
		"rm -rf /", "rm -rf /*", "mkfs", "fdisk", "dd if=", "shutdown", "reboot",
		"init 0", "init 6", "userdel", "groupdel", "iptables -f", "nft flush ruleset",
		":(){ :|:& };:", "curl http", "curl -s http", "wget http",
	}

	for _, pattern := range destructivePrefixes {
		if strings.Contains(cmdLower, pattern) {
			return true, fmt.Sprintf("command contains destructive pattern '%s'", pattern)
		}
	}
	return false, ""
}

func (e *Engine) Evaluate(action string, args map[string]any, env string) (models.RiskLevel, bool, error) {
	meta, exists := e.toolRegistry[action]
	if !exists {
		return models.RiskForbidden, true, fmt.Errorf("unknown or unsupported action: %s", action)
	}

	// Check if action is execute_command and check content
	if action == "execute_command" {
		cmdVal, ok := args["command"].(string)
		if !ok || strings.TrimSpace(cmdVal) == "" {
			return models.RiskForbidden, true, fmt.Errorf("execute_command requires non-empty 'command' argument")
		}
		if isDestructive, reason := e.IsDestructiveCommand(cmdVal); isDestructive {
			return models.RiskForbidden, true, fmt.Errorf("security violation: %s", reason)
		}
	}

	risk := meta.DefaultRisk
	requiresApproval := meta.RequiresApproval

	// Production environment policy escalation
	if strings.EqualFold(env, "production") {
		if risk == models.RiskLow {
			risk = models.RiskMedium
			requiresApproval = true
		}
	}

	return risk, requiresApproval, nil
}

func (e *Engine) GetSupportedTools() []string {
	var list []string
	for k := range e.toolRegistry {
		list = append(list, k)
	}
	return list
}

func (e *Engine) GetToolMetadata(name string) (*ToolMetadata, bool) {
	meta, ok := e.toolRegistry[name]
	return meta, ok
}
