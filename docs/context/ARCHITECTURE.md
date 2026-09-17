# ProvenOps Technical Architecture

## 1. System Overview

ProvenOps is composed of five loosely coupled components collaborating over typed contracts:

```
[ Dashboard (React 18 / Vite / TS / Tailwind) ]
                     │ HTTPS (REST / SSE)
                     ▼
[ Control Plane (Go 1.23, Chi, pgx/v5, gRPC) ] ───▶ [ PostgreSQL 16 ]
         │                               │
         │ HTTP (JSON Schema)            │ gRPC (Outbound mTLS)
         ▼                               ▼
[ AI Service (FastAPI / Ollama) ]   [ Server Agent (Go 1.23 Daemon) ]
```

---

## 2. Component Specifications

### 2.1 Control Plane (`apps/control-plane/`)
- **Port 8080 (REST / SSE)**: Handles API requests from the Dashboard and pushes real-time execution events via Server-Sent Events (`events.Hub`).
- **Port 9090 (gRPC Server)**: Terminating endpoint for agents. Implements `opspilotv1.AgentService` (`Register`, `SendHeartbeat`, `ConnectStream`).
- **State Machine (`internal/statemachine/`)**:
  `CREATED` → `DISCOVERING` → `PLANNING` → `WAITING_APPROVAL` → `EXECUTING` → `OBSERVING` → `VERIFYING` → `COMPLETED`.
  Guards prevent bypassing approval or verification steps.
- **Policy Engine (`internal/policy/`)**: Assigns static risk levels (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`). Enforces mandatory approval on package installations and service mutations.
- **Deterministic Verifier (`internal/verification/`)**: Runs independent TCP socket dials and HTTP GET status probes.
- **Storage (`internal/database/`)**: Dual-mode store (`*pgxpool.Pool` for PostgreSQL with an in-memory resilient fallback).

### 2.2 Server Agent (`agent/`)
- **Execution Mode**: Single binary daemon (`provenops-agent`), non-root `provenops` system user with tightly scoped sudo (`systemctl`, `apt-get`, `dpkg`).
- **Network Model**: Outbound gRPC connection over mTLS to Control Plane. No listening ports on managed Linux hosts.
- **Discovery (`internal/discovery/`)**: Reports hostname, OS distro/version (Ubuntu 22.04/24.04), hardware specs, and capabilities (`systemd`, `apt`, `docker`, `network`, `journald`).
- **Typed Tools (`internal/tools/`)**: 20+ typed actions for systemd services, apt packages, file management (with atomic `.bak` backups), network probes, and Docker containers.
- **Command Guard (`internal/executor/`)**: Tokenizer and regex denylist blocking destructive commands (`rm -rf /`, `mkfs`, `fdisk`, `dd`, `shutdown`, `reboot`, fork bombs, curl-to-bash).

### 2.3 AI Service (`apps/ai-service/`)
- **Framework**: Python 3.11+ with FastAPI, Pydantic v2.
- **Model Providers**: Abstract `LLMProvider` interfacing with local Ollama (`qwen2.5:3b`, `qwen2.5:7b`) and OpenAI-compatible endpoints, with deterministic fallback heuristics.
- **Security Boundary**: All host outputs, stdout, stderr, and logs are wrapped inside `<UNTRUSTED_OBSERVATION>` tags in prompts to neutralize indirect prompt injection.

### 2.4 Dashboard (`apps/dashboard/`)
- **Stack**: React 18, Vite, TypeScript, Tailwind CSS (`slate-950` technical dark mode).
- **Core Views**: Fleet status, Live Task Timeline (SSE streaming), Interactive Approval Gate, Reusable Runbooks, Audit Trail, and Security Policy explorer.

### 2.5 Database (`db/migrations/001_init.sql`)
- PostgreSQL 16 schema with tables for users, roles, agents, capabilities, tasks, steps, events, transitions, approvals, policies, verifications, runbooks, and audit events.
