# OpsPilot — Project Context & Engineering Overview

OpsPilot is a policy-controlled, AI-assisted infrastructure operations platform that translates natural-language sysadmin intent into planned, policy-filtered, auditable, and deterministically verified operations on Linux hosts (Ubuntu 22.04 / 24.04 LTS).

---

## 1. High-Level Architecture & Tech Stack

| Component | Stack | Responsibilities |
| :--- | :--- | :--- |
| **Control Plane** | Go 1.23, Chi, pgx/v5, gRPC | Source of authority: RBAC, State Machine, Policy Engine, Orchestration, SSE Hub, Audit Trail |
| **Server Agent** | Go 1.23 (single binary) | Restricted executor: Outbound mTLS gRPC, host discovery, typed tools, command guard, bbolt ledger |
| **AI Service** | Python 3.11, FastAPI, Pydantic | Untrusted planner: Ollama (Qwen2.5 3B) / OpenAI provider, structured JSON schemas |
| **Dashboard** | React 18, Vite, TS, Tailwind | Technical UI: Fleet view, real-time SSE timeline, approval gates, runbooks, audit explorer |
| **Database** | PostgreSQL 16 | Relational storage for tasks, steps, agent telemetry, approvals, policies, runbooks, audit events |
| **Benchmark Lab**| Python 3, Pytest, Bash | Real Linux fault injection engine (27 scenarios), independent host evaluator, metrics calculator |

---

## 2. Core Security & Reliability Guardrails

1. **LLM Shell Isolation**: The LLM is an untrusted probabilistic planner. It **never** receives direct shell or SSH execution credentials.
2. **Untrusted Data Isolation**: All server outputs, logs, and files are wrapped in `<UNTRUSTED_OBSERVATION>` tags as a defense-in-depth boundary against indirect prompt injection.
3. **Control Plane Authority**: Risk levels (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`) are enforced by the Control Plane policy registry; model self-reported risks are discarded.
4. **Deterministic Verification**: Task success requires independent verification checks (`systemctl is-active`, TCP socket probe, HTTP status code), never exit code 0 or model claims alone.
5. **Defense-in-Depth Guard**: Fallback `execute_command` uses AST tokenization to block destructive operations (`rm -rf /`, `mkfs`, `fdisk`, `dd`, `shutdown`, fork bombs).
6. **Persistent Execution Ledger**: Local embedded `bbolt` KV store ensures strict idempotency and zero duplicate mutations across agent process crashes.
7. **Independent Benchmark Evaluator**: Benchmark results are graded by independent host inspection scripts, enforcing a **0.0% False Success Rate** and **0.0% Unsafe Action Rate**.

---

## 3. Current Project Status

- **Milestone 1 (Verified Vertical Slice)**: COMPLETED. Live Ubuntu 24.04 LTS under WSL2 execution verified (`Install nginx and expose on port 8080`).
- **Milestone 2 & 2.1 (Failure Recovery & Hardening)**: COMPLETED. 13 structured failure types, persistent `bbolt` ledger, uncertain execution observation, config rollbacks, and task bounds.
- **Milestone 3 (Real Fault Injection & Benchmark Lab)**: ACTIVE. 27 real Linux fault scenarios across 8 categories created and currently executing against live Ubuntu environment.
