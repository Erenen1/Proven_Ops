# Current Project State

Last Updated: 2026-09-12

## Current Focus
Milestone 2.1 (Failure Recovery Hardening Pass) completed and proven on Ubuntu 24.04 LTS under WSL2 with persistent bbolt execution ledger, uncertain outcome observation recovery, AI failure boundary isolation, and execution bounds.

## Completed
- **Milestone 2 & 2.1 (Failure-Tolerant Execution & Hardening) Verified on Ubuntu 24.04 LTS under WSL2**:
  - **Structured Failure Classification**: 13 structured failure types implemented (including `UNCERTAIN_EXECUTION`); all executed tests passed.
  - **Persistent Execution Ledger**: Local embedded `bbolt` KV store (`/var/lib/opspilot/execution_ledger.db`) storing execution lifecycle (`RECEIVED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `UNKNOWN`). Prevents duplicate mutating operations across agent restarts.
  - **Uncertain Execution Recovery**: When an agent crashes mid-mutation, status is classified as `UNCERTAIN_EXECUTION`. The Control Plane executes operation-specific observation (`dpkg -s`, `systemctl is-active`, file SHA256) and recovers via `IDEMPOTENT_RECOVERED` without blind retry.
  - **AI Failure Boundary Isolation**: Schema and policy guards strictly isolate the executor from malformed JSON, hallucinated/unsupported tools, and destructive actions (`rm -rf /`), with zero agent dispatch.
  - **Configurable Task Execution Bounds**: Enforces `MAX_REPLANS = 3`, `MAX_TOOL_CALLS = 20`, and `MAX_TOTAL_TASK_DURATION = 10m`, terminating cleanly with structured errors when limits are exceeded.
  - **Transactional Config & Verified Rollback**: Pre-flight backup (`/var/lib/opspilot/backups/{task_id}/`), in-tool atomic syntax validation (`nginx -t`), auto-revert, state machine transitions (`ROLLING_BACK` -> `ROLLED_BACK`), and independent state verification. Validated specifically for Nginx configuration workflows.
  - **Multi-Tier Idempotency**: API `Idempotency-Key` header with PostgreSQL unique constraint; Agent persistent step execution ledger; tool pre-checks returning `ALREADY_SATISFIED` (`IDEMPOTENT_NO_OP` recorded in audit trail).
  - **Agent Disconnect Recovery**: Mid-execution stream disconnect transitions to `WAITING_FOR_AGENT` with grace period; reconnect re-observes and resumes safely without duplicate side effects.
  - **Comprehensive Audit Events**: `TASK_CREATED`, `APPROVAL_GRANTED`, `APPROVAL_INVALIDATED`, `STEP_FAILED`, `RETRY_ATTEMPTED`, `REPLAN_REQUESTED`, `REPLAN_GENERATED`, `PLAN_VERSION_CHANGED`, `ROLLBACK_STARTED`, `ROLLBACK_COMPLETED`, `IDEMPOTENT_NO_OP`, `IDEMPOTENT_RECOVERED`, `AGENT_DISCONNECTED`, `AGENT_RECONNECTED`, `VERIFICATION_FAILED`.
  - **Zero Regressions**: Original vertical slice ("Install nginx on this server and expose it on port 8080.") verified end-to-end with HTTP 200 pass.
- **Milestone 1 (Vertical Slice)**: Live Ubuntu 24.04 LTS under WSL2 vertical slice verified with mTLS, PostgreSQL store, Ollama Qwen planning, and deterministic verification.

## In Progress
- Finalizing Milestone 2 technical validation report and session handoff.

## Next
1. Add automated VM integration test scripts to CI pipeline.
2. Build UI test harness for Dashboard verification against live backend failure states.

## Blocked
- None. All components build and pass unit tests.

## Important Paths
- Entry Point & Memory: `GEMINI.md`, `docs/context/`
- Workspace Rules: `.agents/rules/`
- Control Plane Entry: `apps/control-plane/cmd/server/main.go`
- Server Agent Entry: `agent/cmd/agent/main.go`
- AI Service Entry: `apps/ai-service/app/main.py`
- Dashboard Entry: `apps/dashboard/src/App.tsx`
- Database Schema: `db/migrations/001_init.sql`
- Agent Installer: `scripts/install-agent.sh`
