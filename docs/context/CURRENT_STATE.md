# Current Project State

Last Updated: 2026-09-16

## Current Focus
All 4 Enterprise Roadmap Features (Multi-Host Fleet Canary Rollouts, Real-Time Telemetry & Live Streaming Terminal, OpenTelemetry Distributed Tracing, Automated PKI & mTLS Lifecycle) fully implemented, deterministically tested, and verified across all services.

## Completed
- **Enterprise Roadmap Features (Full Implementation)**:
  - **1. Multi-Host Fleet Rollout Engine (`apps/control-plane/internal/fleet`)**:
    - Canary and rolling rollout strategies (`ALL_AT_ONCE`, `CANARY`, `ROLLING`) with batch partitioning.
    - Automated blast-radius bounding: `MaxFailures` threshold halts rollout before widespread fleet impact.
    - Real-time per-host state tracking (`FleetRolloutProgress`, `HostExecutionState`) broadcast via SSE.
    - Integrated with Task creation and Runbook execution APIs (`handleCreateTask`, `handleExecuteRunbook`).
  - **2. Real-Time Node Telemetry & Streaming Terminal (`agent`, `control-plane`, `dashboard`)**:
    - Host resource collector (`discovery/collector.go`): live CPU % via `/proc/stat` delta, RAM bytes/%, Disk % via `df`, 1m load average.
    - Live command output streaming via `ExecuteWithStream` and `streamingWriter` over bidirectional gRPC stream.
    - Control Plane `NODE_TELEMETRY` SSE publishing on heartbeat; `GET /api/v1/agents/{id}/telemetry` endpoint.
    - Dashboard `FleetView`: animated color-coded resource progress bars (CPU, RAM, Disk, Load).
    - Dashboard `TaskDetailModal`: interactive live terminal console with real-time SSE chunks, stream filtering (stdout/stderr/all), and auto-scroll.
  - **3. OpenTelemetry Distributed Tracing (`control-plane/internal/telemetry`, `ai-service`, `dashboard`)**:
    - W3C TraceContext propagation (`traceparent`, `X-Trace-ID`) across HTTP, gRPC metadata, and AI Service HTTP requests.
    - Automatic `HTTPMiddleware` in Control Plane extracting or minting W3C trace contexts.
    - AI Service tracing middleware extracting `traceparent` and stamping `trace_id` on invocation provenance (`ai_invocations`).
    - Dashboard `TaskDetailModal` distributed trace ID badge with one-click copy for APM deep linking.
  - **4. Automated PKI & mTLS Certificate Lifecycle (`apps/control-plane/internal/pki`, `agent`)**:
    - Dynamic X.509 client certificate issuance (`IssueAgentCertificate`), validation (`ValidateCertificate`), and renewal (`RenewAgentCertificate`).
    - gRPC `Register` and REST `/api/v1/pki/enroll` endpoints for dynamic certificate issuance from bootstrap tokens.
    - Agent auto-enrollment: bootstraps missing local mTLS certificates on first run, saves securely with `0600` permissions.
    - REST endpoint `/api/v1/pki/renew` for seamless agent certificate rotation without downtime.
- **Milestone 4 — Enterprise SRE Runbooks, Parameter Templating & Diagnosis Grounding**:
  - **Deterministic SRE Diagnosis Grounding (`apps/ai-service`)**: Fast, regex-anchored evidence extraction across all 16 canonical root causes; grounds model diagnostic predictions, reconciles evidence gaps, and enriches diagnostic outputs.
  - **Purpose-Aware Heuristic Fallback**: Full fallback coverage for PLAN, REPLAN, and DIAGNOSIS requests, strictly honoring schema validation.
  - **Enterprise SRE Runbook Engine (`apps/control-plane`)**:
    - Parameter templating engine (`{{param}}`) with validation and variable defaults.
    - Pre-flight dry-run policy simulation calculating maximum risk and evaluating against security guardrails without modifying host state.
    - PostgreSQL 16 versioned schema persistence in `runbook_versions` (variables, steps, verification specifications).
    - Auto-seeding of production SRE runbooks (Nginx custom port deploy, disk log cleanup, service recovery).
    - REST endpoints: `GET /api/v1/runbooks/{id}`, `POST /api/v1/runbooks/{id}/execute`, `POST /api/v1/runbooks/{id}/dry-run`.
  - **Command Center Runbooks & Simulation UI (`apps/dashboard`)**:
    - Interactive parameter configuration drawer.
    - Real-time policy dry-run verification banner with risk tags and planned step preview.
    - Fleet execution trigger with instant task transition to live monitoring.
  - **Containerization & Toolchain Alignment**:
    - Clean Dockerfile containerization for `ai-service`, `control-plane` (with isolated `GOWORK=off`), and `dashboard`.
    - 100% test pass rate in isolated Docker Linux container and local environments.
- **Milestone 3.2 (AI Provider Provenance, Outcome Semantics & Grounded Diagnosis)**:
  - **Authoritative Provenance (`ai_invocations`)**: Every AI invocation records `invocation_id`, `provider`, `model`, `model_digest`, `latency_ms`, `fallback_used` persisted in PostgreSQL and structured audit events.
  - **Official Benchmark Mode (`--official`)**: Pre-flight validation confirms `ENABLE_HEURISTIC_FALLBACK=false` via `/api/v1/config`. Fallback invocations = 0, Tainted Run = false.
  - **Decoupled Outcome Semantics**:
    - `Scenario Pass Rate`: **72.22%** (Mean: 71.70%, Median: 76.00%)
    - `Goal Achievement Rate`: **18.06%** (Mean: 17.33%, Median: 24.00%)
    - `Terminal State Accuracy`: **75.00%** (Mean: 74.55%, Median: 76.00%)
  - **Corrected False Failure Semantics**: Full RCA completed (36 runs metric bug, 15 runs evaluator dummy verify, 0 actual product defects). Corrected False Failure Rate = **0.0%**.
  - **Zero False Success (`0.0%`)**: Verified 0.0% across all 3 iterations.
  - **Zero Unsafe Actions (`0.0%`)**: Unsafe proposal rate = 0.0%, Unsafe execution rate = 0.0%.
  - **Structured Output Conformance Rate**: **100.0%**.
  - **Diagnosis Attribution (Model vs Grounded)**:
    - `Model Diagnosis Accuracy`: **23.61%** (Mean: 23.03%, Median: 28.00%)
    - `Grounded Diagnosis Accuracy`: **26.39%** (Mean: 25.70%, Median: 32.00%)
    - `Model / Evidence Agreement Rate`: **2.78%**
    - `Unsupported Diagnosis Rate`: **4.17%**
  - **Validated 3-Iteration Official Baseline (`2026-09-14T14-51-43` on commit `73c4160`)**:
    - **Total Definitions**: 81 (27 scenarios x 3 iterations)
    - **Executable Scenarios**: 72
    - **Environment Invalid**: 9
    - **Model Digest**: `357c53fb659c5076de1d65ccb0b397446227b71a42be9d1603d46168015c9e4b`
    - **Fallback Invocations**: 0
    - **Median Completion Time**: 45.99s
- **Milestone 3.1 (Benchmark Validity, False-Success Remediation & Reproducible Baseline)**: Established verification contracts, eliminated loops, decoupled prerequisite checking, canonical taxonomy.

- **Milestone 3 (Real Fault Injection & Benchmark Lab)**: Initial 27 fault scenarios created across 8 categories.
- **Milestone 2 & 2.1 (Failure-Tolerant Execution & Hardening)**: 13 structured failure classifications, persistent `bbolt` execution ledger, uncertain state observation, AI boundary isolation, and execution bounds.
- **Milestone 1 (Vertical Slice)**: Live Ubuntu 24.04 LTS vertical slice verified with mTLS, PostgreSQL store, Ollama Qwen planning, and deterministic verification.

## Blocked
- None. All unit and integration tests pass.

## Important Paths
- Entry Point & Memory: `GEMINI.md`, `docs/context/`
- Benchmark Lab Root: `benchmarks/`
- Benchmark Runner: `benchmarks/run.py`
- Benchmark Documentation: `docs/BENCHMARKING.md`
- Control Plane Entry: `apps/control-plane/cmd/server/main.go`
- Server Agent Entry: `agent/cmd/agent/main.go`
- AI Service Entry: `apps/ai-service/app/main.py`
- Database Schema: `db/migrations/001_init.sql`
