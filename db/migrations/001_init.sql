-- OpsPilot Initial Database Schema (PostgreSQL 16)
-- Migration: 001_init.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ===================================================
-- 1. USERS, ROLES & RBAC
-- ===================================================

CREATE TABLE IF NOT EXISTS roles (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO roles (id, name, description) VALUES
    ('admin', 'Administrator', 'Full access to users, agents, policies, tasks, and approvals'),
    ('operator', 'Operator', 'Can create and execute tasks, and approve authorized operations'),
    ('viewer', 'Viewer', 'Read-only access to fleet, tasks, and audit logs')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(150),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id VARCHAR(50) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);

-- ===================================================
-- 2. AGENTS & FLEET
-- ===================================================

CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(100) PRIMARY KEY, -- Agent UUID or Unique Host Identifier
    hostname VARCHAR(255) NOT NULL,
    ip_address VARCHAR(100),
    os VARCHAR(50) NOT NULL DEFAULT 'linux',
    distribution VARCHAR(50) NOT NULL DEFAULT 'ubuntu',
    version VARCHAR(50) NOT NULL,
    architecture VARCHAR(50) NOT NULL DEFAULT 'amd64',
    status VARCHAR(50) NOT NULL DEFAULT 'offline', -- 'online', 'offline', 'busy', 'degraded'
    bootstrap_token_hash VARCHAR(255),
    client_cert_serial VARCHAR(100),
    environment VARCHAR(50) NOT NULL DEFAULT 'production', -- 'production', 'staging', 'development'
    last_heartbeat_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_capabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id VARCHAR(100) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability VARCHAR(100) NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_id, capability)
);

CREATE TABLE IF NOT EXISTS agent_heartbeats (
    id BIGSERIAL PRIMARY KEY,
    agent_id VARCHAR(100) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    cpu_usage_percent DOUBLE PRECISION,
    memory_usage_bytes BIGINT,
    memory_total_bytes BIGINT,
    disk_usage_percent DOUBLE PRECISION,
    load_avg_1m DOUBLE PRECISION,
    active_tasks INT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_agent_heartbeats_agent_time ON agent_heartbeats(agent_id, recorded_at DESC);

-- ===================================================
-- 3. POLICIES & RISK RULES
-- ===================================================

CREATE TABLE IF NOT EXISTS policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(150) NOT NULL,
    description TEXT,
    scope VARCHAR(50) NOT NULL DEFAULT 'global', -- 'global', 'environment', 'agent'
    scope_value VARCHAR(100), -- NULL for global, or 'production', or agent_id
    action_pattern VARCHAR(100) NOT NULL, -- e.g. 'install_package', 'restart_service', '*'
    max_risk_level VARCHAR(50) NOT NULL DEFAULT 'LOW', -- 'READ_ONLY', 'LOW', 'MEDIUM', 'HIGH', 'FORBIDDEN'
    requires_approval BOOLEAN NOT NULL DEFAULT TRUE,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Default baseline policies
INSERT INTO policies (name, description, scope, action_pattern, max_risk_level, requires_approval, is_enabled) VALUES
    ('Allow Read-Only Operations Globally', 'Read-only discovery tools run without approval', 'global', 'get_*', 'READ_ONLY', FALSE, TRUE),
    ('Require Approval for Package Installation', 'Installing packages modifies system state', 'global', 'install_package', 'MEDIUM', TRUE, TRUE),
    ('Require Approval for Service Modification', 'Starting, stopping, restarting services', 'global', '*_service', 'MEDIUM', TRUE, TRUE),
    ('High Risk Raw Command Fallback', 'Any execute_command fallback requires high security scrutiny', 'global', 'execute_command', 'HIGH', TRUE, TRUE)
ON CONFLICT DO NOTHING;

-- ===================================================
-- 4. TASKS, STATE MACHINE & TARGETS
-- ===================================================

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    prompt TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'CREATED', 
    -- 'CREATED', 'DISCOVERING', 'PLANNING', 'WAITING_APPROVAL', 'EXECUTING', 
    -- 'OBSERVING', 'VERIFYING', 'REPLANNING', 'COMPLETED', 'PARTIAL_SUCCESS', 
    -- 'FAILED', 'ROLLED_BACK', 'CANCELLED', 'TIMEOUT'
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    plan_version INT NOT NULL DEFAULT 1,
    ai_plan JSONB,
    risk_level VARCHAR(50) NOT NULL DEFAULT 'LOW', -- Calculated by Policy Engine
    error_message TEXT,
    execution_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS task_targets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id VARCHAR(100) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'IN_PROGRESS', 'SUCCESS', 'FAILED', 'TIMEOUT', 'OFFLINE'
    output TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (task_id, agent_id)
);

CREATE TABLE IF NOT EXISTS task_transitions (
    id BIGSERIAL PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_status VARCHAR(50) NOT NULL,
    to_status VARCHAR(50) NOT NULL,
    reason TEXT,
    triggered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    transitioned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_task_transitions_task ON task_transitions(task_id, transitioned_at ASC);

CREATE TABLE IF NOT EXISTS task_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    action VARCHAR(100) NOT NULL,
    arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
    risk_level VARCHAR(50) NOT NULL,
    requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
    verification_strategy JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'SKIPPED'
    exit_code INT,
    stdout TEXT,
    stderr TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_task_steps_task_order ON task_steps(task_id, step_order ASC);

CREATE TABLE IF NOT EXISTS task_events (
    id BIGSERIAL PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL, -- 'DISCOVERY_UPDATE', 'PLAN_GENERATED', 'STEP_STARTED', 'STEP_OUTPUT', 'STEP_COMPLETED', 'VERIFICATION_PASSED', etc.
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_task_events_task_time ON task_events(task_id, created_at ASC);

-- ===================================================
-- 5. APPROVALS
-- ===================================================

CREATE TABLE IF NOT EXISTS approvals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    plan_version INT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'APPROVED', 'REJECTED', 'EXPIRED'
    risk_level VARCHAR(50) NOT NULL,
    decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    decision_notes TEXT,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (task_id, plan_version)
);

-- ===================================================
-- 6. TOOL EXECUTIONS & VERIFICATION RESULTS
-- ===================================================

CREATE TABLE IF NOT EXISTS tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    step_id UUID REFERENCES task_steps(id) ON DELETE SET NULL,
    agent_id VARCHAR(100) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
    exit_code INT,
    stdout TEXT,
    stderr TEXT,
    duration_ms BIGINT,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS verification_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    step_id UUID REFERENCES task_steps(id) ON DELETE SET NULL,
    agent_id VARCHAR(100) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    check_type VARCHAR(100) NOT NULL, -- 'systemd_active', 'tcp_port_open', 'http_probe', 'package_installed', 'checksum'
    target VARCHAR(255) NOT NULL, -- e.g. 'nginx', '8080', 'http://localhost:8080'
    passed BOOLEAN NOT NULL,
    details JSONB DEFAULT '{}'::jsonb,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ===================================================
-- 7. AUDIT LOGS
-- ===================================================

CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    username VARCHAR(100),
    agent_id VARCHAR(100),
    task_id UUID,
    event_type VARCHAR(100) NOT NULL, -- 'TASK_CREATED', 'APPROVAL_GRANTED', 'COMMAND_EXECUTED', 'POLICY_VIOLATION', etc.
    action VARCHAR(100),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_events_time ON audit_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_task ON audit_events(task_id);

-- ===================================================
-- 8. RUNBOOKS
-- ===================================================

CREATE TABLE IF NOT EXISTS runbooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(100) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    created_from_task UUID REFERENCES tasks(id) ON DELETE SET NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS runbook_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    runbook_id UUID NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    version INT NOT NULL,
    variables JSONB NOT NULL DEFAULT '[]'::jsonb, -- list of variable descriptors: {name, default, required, description}
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,     -- deterministic sequence of actions and args
    verification_spec JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (runbook_id, version)
);

CREATE TABLE IF NOT EXISTS runbook_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    runbook_id UUID NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    version INT NOT NULL,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    executed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
