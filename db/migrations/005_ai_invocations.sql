-- 005_ai_invocations.sql
-- Migration: AI Provider Provenance and Invocations Log

CREATE TABLE IF NOT EXISTS ai_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invocation_id VARCHAR(64) UNIQUE NOT NULL,
    task_id VARCHAR(64),
    scenario_id VARCHAR(64),
    purpose VARCHAR(32) NOT NULL, -- PLAN, REPLAN, DIAGNOSIS
    provider VARCHAR(64) NOT NULL, -- ollama, openai, synthetic_test, heuristic_fallback
    model VARCHAR(64) NOT NULL,
    model_digest VARCHAR(128),
    fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_reason TEXT,
    request_started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    request_finished_at TIMESTAMP WITH TIME ZONE NOT NULL,
    latency_ms BIGINT NOT NULL,
    success BOOLEAN NOT NULL DEFAULT TRUE,
    schema_valid BOOLEAN NOT NULL DEFAULT TRUE,
    error_type VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_invocations_task_id ON ai_invocations(task_id);
CREATE INDEX IF NOT EXISTS idx_ai_invocations_created_at ON ai_invocations(created_at);
