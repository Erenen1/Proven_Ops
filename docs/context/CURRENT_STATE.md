# Current Project State

Last Updated: 2026-09-12

## Current Focus
Milestone 3 (Real Fault Injection & Benchmark Lab) completed, proven, and benchmarked on live Ubuntu 24.04 LTS under WSL2 across 27 real-infrastructure fault scenarios with an independent host evaluator.

## Completed
- **Milestone 3 (Real Fault Injection & Benchmark Lab) Verified on Ubuntu 24.04 LTS under WSL2**:
  - **27 Real Linux Fault Scenarios**: Spanning 8 categories (Nginx, Systemd, Docker, Filesystem, Permissions, Network, Agent, AI Failure). All old offline JSON mocks removed.
  - **Independent Evaluator**: External verification decoupled from agent self-reporting (`systemctl is-active`, `ss -tulpn`, `curl`, `dpkg`, file content state).
  - **Benchmark Automation**: Single-command execution via `python3 benchmarks/run.py` supporting `--model`, `--iterations`, `--seed`, and comparative analysis via `benchmarks/compare.py`.
  - **Safe Sandbox Execution**: Guaranteed cleanup and reset in `finally` blocks; zero risk to host root filesystems.
  - **Empirical Measurement on Real Host (Run `2026-09-12T22-12-42` against Qwen 2.5 3B)**:
    - **Total Scenarios**: 27
    - **Passed**: 20
    - **Failed**: 7
    - **Task Success Rate**: **74.07%**
    - **Diagnosis Accuracy**: **18.52%** (Strict schema enforcement on LLM output)
    - **Recovery Rate**: **51.85%**
    - **Unsafe Action Rate**: **0.0%** (Hard security constraint satisfied)
    - **False Success Rate**: **7.41%** (Discrepancy detected where agent reported completion but independent host verification failed)
    - **Human Intervention Rate**: **40.74%**
    - **Rollback Success Rate**: **100.0%**
    - **Median Duration**: **37.98s**
    - **Median Tool Calls**: **1.0**
- **Milestone 2 & 2.1 (Failure-Tolerant Execution & Hardening)**: 13 structured failure classifications, persistent `bbolt` execution ledger, uncertain state observation, AI boundary isolation, and execution bounds.
- **Milestone 1 (Vertical Slice)**: Live Ubuntu 24.04 LTS vertical slice verified with mTLS, PostgreSQL store, Ollama Qwen planning, and deterministic verification.
- **Regression Testing**: Original vertical slice ("Install nginx on this server and expose it on port 8080.") verified healthy with HTTP 200 pass.

## In Progress
- Finalizing Milestone 3 documentation and session handoff.

## Next
1. Expand model comparison runs across Qwen 2.5 7B / 14B models to measure reliability scaling.
2. Build UI Benchmark Explorer in React Dashboard to visualize scenario runs, traces, and metrics.

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
