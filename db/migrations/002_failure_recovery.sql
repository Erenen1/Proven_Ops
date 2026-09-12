-- OpsPilot Failure Recovery & Idempotency Schema (PostgreSQL 16)
-- Migration: 002_failure_recovery.sql

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255) UNIQUE;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS replan_count INT NOT NULL DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS max_replans INT NOT NULL DEFAULT 3;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS failure_details JSONB;

ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS execution_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_tasks_idempotency_key ON tasks(idempotency_key);
