# Current Project State

Last Updated: 2026-09-12

## Current Focus
Milestone 2 (Failure-Tolerant Agent Execution) completed and proven on Ubuntu 24.04 with PostgreSQL persistence, real Ollama planning, mTLS agent communication, and deterministic verification.

## Completed
- **Milestone 2 (Failure-Tolerant Execution) Verified on Ubuntu 24.04**:
  - **Structured Failure Classification**: 12 failure types implemented and unit tested (100% pass).
  - **Bounded Controlled Replanning**: `MAX_REPLANS = 3`, `plan_version` incrementing, approval invalidation upon plan mutation, strict intent integrity (no silent port alterations).
  - **Transactional Config & Verified Rollback**: Pre-flight backup (`/var/lib/opspilot/backups/{task_id}/`), in-tool atomic syntax validation (`nginx -t`), auto-revert, state machine transitions (`ROLLING_BACK` -> `ROLLED_BACK`), and independent state verification.
  - **Multi-Tier Idempotency**: API `Idempotency-Key` header with PostgreSQL unique constraint; Agent step execution cache (`execution_id = task_id/step_id/attempt`); tool pre-checks returning `ALREADY_SATISFIED` (`IDEMPOTENT_NO_OP` recorded in audit trail).
  - **Agent Disconnect Recovery**: Mid-execution stream disconnect transitions to `WAITING_FOR_AGENT` with grace period; reconnect re-observes and resumes safely without duplicate side effects.
  - **Comprehensive Audit Events**: `TASK_CREATED`, `APPROVAL_GRANTED`, `APPROVAL_INVALIDATED`, `STEP_FAILED`, `RETRY_ATTEMPTED`, `REPLAN_REQUESTED`, `REPLAN_GENERATED`, `PLAN_VERSION_CHANGED`, `ROLLBACK_STARTED`, `ROLLBACK_COMPLETED`, `IDEMPOTENT_NO_OP`, `AGENT_DISCONNECTED`, `AGENT_RECONNECTED`, `VERIFICATION_FAILED`.
  - **Zero Regressions**: Original vertical slice ("Install nginx on this server and expose it on port 8080.") verified end-to-end with HTTP 200 pass.
- **Milestone 1 (Vertical Slice)**: Live Ubuntu 24.04 vertical slice verified with mTLS, PostgreSQL store, Ollama Qwen planning, and deterministic verification.

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
