# Current Project State

Last Updated: 2026-09-16

## Current Focus
Milestone 5 (Production-Grade Infrastructure Orchestration Engine) completed. OpsPilot has transitioned from a prototype to a hardened, enterprise-grade infrastructure operations platform across all 44 specifications with comprehensive multi-node testing, state durability, and zero-blind-trust guardrails intact.

## Completed
- **Milestone 5 — Production-Grade Infrastructure Orchestration Engine**:
  - **1. Durable 16-State State Machine (`apps/control-plane/internal/statemachine`)**:
    - Complete lifecycle: `PENDING`, `PRECHECKING`, `SKIPPED`, `WAITING_APPROVAL`, `QUEUED`, `DISPATCHED`, `RUNNING`, `VERIFYING`, `SUCCEEDED`, `FAILED`, `RETRYING`, `COMPENSATING`, `COMPENSATED`, `PARTIALLY_COMPENSATED`, `MANUAL_INTERVENTION_REQUIRED`, `CANCELLED`.
    - Optimistic concurrency control (`optimistic_lock_version`) on tasks and steps to prevent race conditions.
    - Database migration `006_production_hardening.sql` applied to PostgreSQL 16.
  - **2. Idempotency & Desired-State Reconciliation (`agent/internal/tools/ensure_tools.go`)**:
    - Abstractions: `ensure_package`, `ensure_service`, `ensure_file`, `ensure_directory`, `ensure_port_state`.
    - Precheck lifecycle returning `SKIPPED_ALREADY_DESIRED` when host is already in target state without performing destructive mutations.
    - Atomic file replacement (`tmp` -> `fsync` -> `rename`) preventing TOCTOU race conditions.
    - Idempotency key deduplication at task, step, and operation levels.
  - **3. DAG Dependency Engine (`apps/control-plane/internal/dag`)**:
    - Directed Acyclic Graph supporting steps declaring `depends_on: ["step_1", "step_2"]`.
    - Cycle detection using Kahn's topological sort algorithm.
    - Independent steps batched for parallel dispatch; failed upstream steps mark downstream dependents as `BLOCKED_BY_DEPENDENCY`.
  - **4. Saga LIFO Compensation / Rollback Engine (`apps/control-plane/internal/saga`)**:
    - Explicit reversibility metadata (`FULL`, `PARTIAL`, `NONE`) on all typed actions.
    - Automated pre-flight snapshotting and LIFO rollback restoration on verification failure.
    - Rollback actions subjected to deterministic post-verification.
  - **5. Verification Evidence Contract (`apps/control-plane/internal/verification`)**:
    - Strict `EXECUTED != VERIFIED` separation.
    - Structured evidence (`CheckType`, `Target`, `Expected`, `Actual`, `Passed`, `DurationMS`, `AgentID`) recorded in PostgreSQL `verification_results`.
  - **6. Standard 7-Class Error Taxonomy & Jitter Retry Engine (`apps/control-plane/internal/failures`)**:
    - Classification: `TRANSIENT`, `PERMANENT`, `POLICY`, `VERIFICATION`, `CONNECTIVITY`, `TIMEOUT`, `UNKNOWN`.
    - Exponential backoff with random jitter (±20%) for transient and connectivity errors; permanent and policy errors fail-fast.
  - **7. Security, AST Guard & Secret Redaction (`apps/control-plane/internal/security`, `agent/internal/executor`)**:
    - AST guard checks blocking subshells (`bash -c`), `eval`, `find -exec`, `xargs`, `curl | sh`, dangerous device writes (`> /dev/sd*`), and path traversal (`../`).
    - Dedicated `security.RedactString` and `security.RedactMap` masking private keys, JWTs, URI credentials, and API tokens across logs and audit events.
    - PKI Certificate Revocation List (CRL) validation rejecting revoked agent certificates.
  - **8. Multi-Node Linux Lab (`docker-compose.yml`, `agent/Dockerfile`)**:
    - Network isolation: `frontend-net`, `backend-net`, `agent-net`.
    - Multi-node simulation profile: `docker compose --profile lab up --build` spinning up 5 real Linux agent nodes (`node-01` to `node-05`) with mTLS gRPC communication.
  - **9. Comprehensive Failure Injection & Race Detector Test Suites**:
    - `TestFailureInjection_*` in `internal/orchestrator` passing all 10 critical scenarios (duplicate requests, canary halt, Saga rollback, revoked certificates, DAG failure propagation, secret redaction, budget exhaustion).
    - Data race detection clean (`go test -race`).
    - Pytest 8/8 passing, Vite TypeScript production build clean.
  - **10. Production Documentation**:
    - `docs/THREAT_MODEL.md`
    - `docs/FAILURE_MODEL.md`
    - `docs/PRODUCTION_READINESS.md`

## Blocked
- None. All unit, race, and integration tests pass 100%.

## Important Paths
- Entry Point & Memory: `GEMINI.md`, `docs/context/`
- Threat Model: `docs/THREAT_MODEL.md`
- Failure Model: `docs/FAILURE_MODEL.md`
- Production Readiness: `docs/PRODUCTION_READINESS.md`
- Control Plane Entry: `apps/control-plane/cmd/server/main.go`
- Server Agent Entry: `agent/cmd/agent/main.go`
- AI Service Entry: `apps/ai-service/app/main.py`
- Database Migrations: `db/migrations/001_init.sql` through `006_production_hardening.sql`
