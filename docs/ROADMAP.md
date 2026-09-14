# OpsPilot Development Roadmap

This document outlines the evolutionary milestones of the OpsPilot platform from initial concept to a hardened production-grade autonomous operations framework.

---

## Milestone 1 — Verified Vertical Slice (Completed)
**Goal:** Deliver an end-to-end operational loop from natural language intent to deterministically verified Linux host execution.
- [x] Outbound gRPC architecture with mTLS between Server Agent and Control Plane.
- [x] Single-binary Go agent with typed system tools (`package`, `file`, `systemd`, `network`).
- [x] AI Planning Service with structured Pydantic schemas powered by local Ollama (`qwen2.5:3b`).
- [x] Control Plane state machine (`DISCOVERING` -> `PLANNING` -> `WAITING_APPROVAL` -> `EXECUTING` -> `VERIFYING` -> `COMPLETED`).
- [x] Deterministic multi-probe verification (`systemctl is-active`, TCP socket probe, HTTP 200).
- [x] Live end-to-end execution verified on Ubuntu 24.04 LTS under WSL2 (`Install nginx and expose on port 8080`).

---

## Milestone 2 & 2.1 — Failure-Tolerant Execution & Hardening (Completed)
**Goal:** Transform the agent from a happy-path planner into a failure-tolerant distributed operator.
- [x] Structured failure classification across 13 failure types.
- [x] Persistent embedded execution ledger via `bbolt` (`/var/lib/opspilot/execution_ledger.db`) guaranteeing zero duplicate mutations across agent restarts.
- [x] Uncertain execution recovery: deterministic state observation (`dpkg -s`, `systemctl is-active`, SHA256) on agent restart during in-flight mutations.
- [x] AI failure boundary isolation: schema rejection for malformed JSON, unsupported tools, and destructive command patterns.
- [x] Configurable execution bounds (`MAX_REPLANS = 3`, `MAX_TOOL_CALLS = 20`, `MAX_TOTAL_TASK_DURATION = 10m`).
- [x] Transactional configuration backups and automated rollback with verified service health checks.
- [x] Transport disconnect recovery with grace period (`WAITING_FOR_AGENT` -> auto-resume).
- [x] Comprehensive audit trail events.

---

## Milestone 3 — Real Fault Injection & Benchmark Lab (Completed)
**Goal:** Measure agent reliability, safety, and diagnostic accuracy against real Linux faults on actual Ubuntu environments.
- [x] 27 real Linux fault scenarios implemented across 8 categories (Nginx, Systemd, Docker, Filesystem, Permissions, Network, Agent, AI Failure).
- [x] Independent host evaluator decoupling verification from agent self-reporting.
- [x] Initial benchmark execution (27 scenarios, 74.07% initial task success, 0.0% unsafe action rate, 7.41% false success detected).
- [x] Deterministic cleanup and reset in guaranteed `finally` blocks.
- [x] Multi-iteration (`--iterations`), model parameterization (`--model`), random seed (`--seed`), and run comparison (`compare.py`).

---

## Milestone 3.1 — Benchmark Validity, False-Success Remediation & Reproducible Baseline (Completed)
**Goal:** Eliminate false-success platform bugs, enforce deterministic verification contracts, separate environment-invalid prerequisite failures, calibrate diagnosis accuracy with canonical taxonomy, and establish a reproducible 3-iteration baseline.
- [x] **Zero False Success Guarantee**: Enforced deterministic verification contracts where mutating operations cannot transition to `COMPLETED` on command exit codes or AI assertions alone (`False Success Rate = 0.0%`).
- [x] **Zero Unsafe Action Execution**: Guaranteed AST inspection and policy guardrails (`Unsafe Action Execution Rate = 0.0%`).
- [x] **Environment Decoupling**: Prerequisite validator classifies absent subsystems (Docker daemon) as `ENVIRONMENT_INVALID`, excluding them from executable success rate denominators.
- [x] **Canonical Root Cause Taxonomy**: Decoupled `STRUCTURED_OUTPUT_CONFORMANCE_RATE` (100.0%) from `DIAGNOSIS_ACCURACY` evaluated across 16-type structured taxonomy.
- [x] **Loop Elimination**: Resolved replanning loops and uncontrolled timeouts (`systemd-missing-unit`, `nginx-port-conflict`, `nginx-service-stopped`) via plan fingerprinting and non-progress detection (`Timeout Rate = 0.0%`).
- [x] **Reproducible Multi-Run Aggregation**: Executed 3-iteration baseline with seed, measuring mean, median, min, max, and flakiness tracking.
- [x] **Audit-Accurate Tool Execution Metrics**: autoritative tracking of actual dispatched agent tool executions (median 7.0 calls).

---

## Milestone 3.2 — AI Provider Provenance, Outcome Semantics & Grounded Diagnosis (Completed)
**Goal:** Guarantee complete auditable AI invocation provenance, enforce official benchmark rules forbidding heuristic fallback, decouple scenario behavioral pass rates from physical goal achievement, correct false failure measurement semantics, and separate raw model diagnosis from evidence-grounded root cause resolution.
- [x] **AI Provider Provenance & Model Digest**: Every plan, replan, and diagnosis records immutable provenance (`invocation_id`, `provider`, `model`, `model_digest`, `latency_ms`, `fallback_used`) persisted in PostgreSQL `ai_invocations` table and structured audit logs.
- [x] **Official Benchmark Mode (`--official`)**: Mandates `ENABLE_HEURISTIC_FALLBACK=false` via `/api/v1/config`, disallows synthetic providers from operational metrics, and marks any run with fallback as `TAINTED`.
- [x] **Decoupled Outcome Semantics**: Separated `SCENARIO_PASS_RATE` (behavioral correctness), `GOAL_ACHIEVEMENT_RATE` (physical host state realization), and `TERMINAL_STATE_ACCURACY`. Safe operator deferrals (`WAITING_APPROVAL`) and safe policy refusals (`FAILED`) are recognized as valid outcomes.
- [x] **Corrected False Failure Semantics**: Full RCA of M3.1's 77.27% False Failure rate (36 runs metric bug, 15 runs evaluator dummy verify bug, 0 runs actual product defect). Redefined metric to trigger only when desired state was physically achieved on host and scenario required completion, but OpsPilot terminated in failure.
- [x] **Evidence-Grounded vs. Raw Model Diagnosis**: Preserved unadulterated `raw_model_diagnosis` while deterministically extracting `evidence_root_cause` from execution traces (systemctl, socket binding, HTTP response codes). Added metrics for `MODEL_DIAGNOSIS_ACCURACY`, `GROUNDED_DIAGNOSIS_ACCURACY`, `MODEL_EVIDENCE_AGREEMENT_RATE`, and `UNSUPPORTED_DIAGNOSIS_RATE`.

---

## Milestone 4 — Production Multi-Host Fleet & Advanced Runbooks (Planned)

**Goal:** Scale platform from single-node operations to distributed multi-host fleets and enterprise runbook execution.
- [ ] Multi-agent concurrent execution with host topology tags and canary rollout strategies.
- [ ] Visual runbook canvas in React Dashboard with parameter templating.
- [ ] OpenTelemetry distributed tracing across Control Plane, Agent gRPC streams, and AI planning.
- [ ] Production Helm charts and multi-architecture systemd deb/rpm packaging.

