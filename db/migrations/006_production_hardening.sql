-- OpsPilot Production Hardening Migration
-- Migration: 006_production_hardening.sql

-- 1. Tasks Table Enhancements
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS optimistic_lock_version INT NOT NULL DEFAULT 1;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS remediation_budget JSONB DEFAULT '{"max_attempts": 2, "max_risk": "MEDIUM", "max_runtime_sec": 300, "attempts_used": 0}'::jsonb;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS cancel_reason TEXT;

-- 2. Task Steps Table Enhancements
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS optimistic_lock_version INT NOT NULL DEFAULT 1;
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS depends_on JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS reversibility VARCHAR(20) NOT NULL DEFAULT 'NONE'; -- 'FULL', 'PARTIAL', 'NONE'
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS compensation_action JSONB;
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS precheck_result JSONB;
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0;
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 3;
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS error_class VARCHAR(50); -- 'TRANSIENT', 'PERMANENT', 'POLICY', 'VERIFICATION', 'CONNECTIVITY', 'TIMEOUT', 'UNKNOWN'
ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS rollback_status VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_task_steps_idempotency ON task_steps(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_task_steps_task_status ON task_steps(task_id, status);

-- 3. Verification Results Table Enhancements
ALTER TABLE verification_results ADD COLUMN IF NOT EXISTS evidence JSONB DEFAULT '{}'::jsonb;
ALTER TABLE verification_results ADD COLUMN IF NOT EXISTS duration_ms BIGINT DEFAULT 0;

-- 4. Single-Use Bootstrap Tokens Table (Replay Protection & Expiry)
CREATE TABLE IF NOT EXISTS bootstrap_tokens (
    token_hash VARCHAR(255) PRIMARY KEY,
    description VARCHAR(255),
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    used_at TIMESTAMPTZ,
    used_by_agent_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_bootstrap_tokens_expiry ON bootstrap_tokens(expires_at, used);

-- 5. Certificate Revocation List (CRL) Table
CREATE TABLE IF NOT EXISTS revoked_certificates (
    serial VARCHAR(100) PRIMARY KEY,
    agent_id VARCHAR(100) NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reason TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_revoked_certs_agent ON revoked_certificates(agent_id);

-- 6. Durable Fleet Rollout Persistence (Survives Control Plane Restarts)
CREATE TABLE IF NOT EXISTS fleet_rollouts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    strategy VARCHAR(50) NOT NULL, -- 'CANARY', 'ROLLING', 'ALL_AT_ONCE'
    batch_size INT NOT NULL DEFAULT 1,
    canary_nodes INT NOT NULL DEFAULT 1,
    max_failures INT NOT NULL DEFAULT 0,
    max_failure_percentage DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    pause_between_sec INT NOT NULL DEFAULT 0,
    total_hosts INT NOT NULL DEFAULT 0,
    completed_hosts INT NOT NULL DEFAULT 0,
    failed_hosts INT NOT NULL DEFAULT 0,
    halted BOOLEAN NOT NULL DEFAULT FALSE,
    halt_reason TEXT,
    host_states JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(task_id)
);
CREATE INDEX IF NOT EXISTS idx_fleet_rollouts_task ON fleet_rollouts(task_id);
