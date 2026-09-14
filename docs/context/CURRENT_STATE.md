# Current Project State

Last Updated: 2026-09-14

## Current Focus
Milestone 3.2 (AI Provider Provenance, Outcome Semantics & Grounded Diagnosis) completed and verified across an official 3-iteration run on live Ubuntu 24.04 LTS under WSL2.

## Completed
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
