# ProvenOps — AI Project Entry Point

ProvenOps is a policy-controlled AI infrastructure operations platform that converts natural-language sysadmin intent into planned, policy-filtered, auditable, and deterministically verified operations on Linux hosts (Ubuntu 22.04/24.04).

---

## 1. High-Level Architecture & Tech Stack

| Component | Stack | Responsibilities |
| :--- | :--- | :--- |
| **Control Plane** | Go 1.23, Chi, pgx/v5, gRPC | Source of authority: RBAC, State Machine, Policy Engine, Orchestration, SSE Hub, Audit Trail |
| **Server Agent** | Go 1.23 (single binary) | Restricted executor: Outbound mTLS gRPC, host discovery, typed tools, command guard, non-root runner |
| **AI Service** | Python 3.11, FastAPI, Pydantic | Untrusted planner: Ollama (Qwen2.5/3) & OpenAI provider, structured JSON schemas, injection defense |
| **Dashboard** | React 18, Vite, TS, Tailwind | Technical UI: Fleet view, real-time SSE timeline, approval gates, runbooks, audit explorer |
| **Database** | PostgreSQL 16 | Relational storage for tasks, steps, agent telemetry, approvals, policies, runbooks, audit events |

---

## 2. Critical Security & Engineering Guardrails

1. **LLM Shell Isolation**: The LLM is an untrusted probabilistic planner. It **never** receives direct shell or SSH execution privileges.
2. **Untrusted Data Isolation**: All server outputs, logs, and files are wrapped in `<UNTRUSTED_OBSERVATION>` tags as a defense-in-depth boundary against indirect prompt injection (paired with strict JSON schema validation and control-plane policy enforcement).
3. **Control Plane Authority**: Risk levels (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`) are enforced by the Control Plane policy registry; model self-reported risks are discarded.
4. **Deterministic Verification**: Task success requires independent verification checks (`systemctl is-active`, TCP socket probe, HTTP status code), never exit code 0 or model claims alone.
5. **Defense-in-Depth Guard**: Fallback `execute_command` uses AST tokenization to block destructive operations (`rm -rf /`, `mkfs`, `fdisk`, `dd`, `shutdown`, fork bombs).

---

## 3. Source of Truth Hierarchy

When resolving ambiguities or conflicts:
1. **Source Code & Runtime Implementation** (ground truth)
2. **Database Schemas & Configuration** (`db/migrations/`, `.env.example`, `go.work`)
3. **Architecture & Product Documentation** (`docs/context/ARCHITECTURE.md`, `PRODUCT.md`)
4. **Current Project State** (`docs/context/CURRENT_STATE.md`)
5. **Session Handoff** (`docs/context/SESSION_HANDOFF.md`)
6. **Chat History** (ephemeral context)

*Rule: If documentation contradicts the code, inspect the actual implementation, verify with tests, and update the stale context file.*

---

## 4. Project Memory Map & Progressive Context Loading

Do **not** read the entire codebase at session start. Follow progressive disclosure:
- **Every Session Start**: Read `GEMINI.md` → `docs/context/CURRENT_STATE.md` → `docs/context/SESSION_HANDOFF.md`.
- **Architecture Tasks**: Consult `docs/context/ARCHITECTURE.md` and `docs/context/DECISIONS.md`.
- **Business Logic & Workflows**: Consult `docs/context/PRODUCT.md`.
- **Debugging & Faults**: Consult `docs/context/KNOWN_ISSUES.md`.
- **Specialized Rules**: Loaded from `.agents/rules/` (`00-core.md`, `10-context-memory.md`, `20-backend.md`, `30-frontend.md`, `40-database.md`).
- **End of Session**: Run `/project-handoff` skill or update `docs/context/SESSION_HANDOFF.md`.
