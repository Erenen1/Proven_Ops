package tools

import (
	"context"
	"fmt"
	"sync"
)

type ExecutionResult struct {
	ExitCode int            `json:"exit_code"`
	Stdout   string         `json:"stdout"`
	Stderr   string         `json:"stderr"`
	Success  bool           `json:"success"`
	Data     map[string]any `json:"data,omitempty"`
}

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]any) (*ExecutionResult, error)
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}
	r.registerDefaultTools()
	return r
}

func (r *Registry) registerDefaultTools() {
	// System Tools
	for _, action := range []string{
		"get_os_info", "get_system_info", "get_cpu_usage",
		"get_memory_usage", "get_disk_usage", "get_load_average", "list_processes",
	} {
		r.Register(NewSystemTool(action))
	}

	// Systemd Tools
	for _, action := range []string{
		"get_service_status", "start_service", "stop_service",
		"restart_service", "enable_service", "get_service_logs",
	} {
		r.Register(NewSystemdTool(action))
	}

	// Package Tools
	r.Register(NewPackageTool("check_package"))
	r.Register(NewPackageTool("install_package"))

	// Network Tools
	for _, action := range []string{
		"check_port", "http_probe", "get_open_ports", "dns_lookup",
	} {
		r.Register(NewNetworkTool(action))
	}

	// File Tools
	r.Register(NewFileTool("read_file"))
	r.Register(NewFileTool("write_config_file"))
	r.Register(NewFileTool("rollback_config"))

	// Docker Tools
	for _, action := range []string{
		"docker_info", "docker_ps", "docker_logs", "docker_inspect",
	} {
		r.Register(NewDockerTool(action))
	}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool '%s' is not registered", name)
	}
	return t, nil
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []string
	for k := range r.tools {
		list = append(list, k)
	}
	return list
}
