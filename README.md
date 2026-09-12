# OpsPilot

### Policy-Controlled AI Infrastructure Operations Platform

> **"OpsPilot converts natural-language infrastructure tasks into planned, policy-controlled, auditable and verified server operations."**

OpsPilot is an enterprise-grade platform that bridges high-level operational intent with low-level Linux systems administration without sacrificing safety, control, or verification.

It is explicitly **not** a simple “LLM → SSH → Shell command” script. The LLM functions exclusively as an **untrusted probabilistic planner**, while the Go Control Plane acts as the authoritative security gate, and the Go Server Agent operates as a constrained executor with deterministic verification.

---

## Architecture Overview

```
       [ Human Operator ]
               |
               v
     +-------------------+
     |  React Dashboard  |
     +---------+---------+
               | HTTPS (REST / SSE)
               v
+-------------------------------------------------------------+
|                      CONTROL PLANE (Go)                     |
|                                                             |
|   +---------------+   +------------------+   +----------+   |
|   |  Auth & RBAC  |   |   Task Engine    |   |  Policy  |   |
|   | (JWT / Roles) |   |  (State Machine) |   |  Engine  |   |
|   +---------------+   +--------+---------+   +----+-----+   |
|                                |                  |         |
|                        +-------v------------------v-----+   |
|                        |          Orchestrator          |   |
|                        +-------+------------------+-----+   |
|                                |                  |         |
|   +----------------------------v-+   +------------v-----+   |
|   |     Verification Engine      |   |   Audit Trail    |   |
|   | (Socket / HTTP / Systemd)    |   | (Immutable Log)  |   |
|   +------------------------------+   +------------------+   |
+------------------+-----------------------------+------------+
                   |                             |
                   | HTTP (JSON Schema)          | gRPC (mTLS Outbound)
                   v                             v
         +-------------------+         +-------------------+
         |    AI SERVICE     |         |   SERVER AGENT    |
         |  (Python/FastAPI) |         |  (Ubuntu 24.04)   |
         | - Strict Schemas  |         | - Capabilities    |
         | - Prompt Defense  |         | - Typed Tools     |
         | - Qwen / Ollama   |         | - Command Guard   |
         +-------------------+         +-------------------+
```

---

## Core Security Principle

> **"LLM output is never trusted as executable authority."**

1. **Untrusted Planner**: The model never holds shell execution credentials. It only recommends structured typed actions.
2. **Untrusted Data Isolation**: All server logs, stdout, and file contents are tagged as `<UNTRUSTED_OBSERVATION>` as a defense-in-depth boundary against indirect prompt injection (paired with structured JSON schema validation and control plane policy enforcement).
3. **Hardened Policy Boundary**: Actions have static risk levels (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`). The LLM's own self-reported risk is ignored.
4. **Deterministic Verification**: Tasks are never marked as `COMPLETED` based on an LLM statement or exit code 0 alone; independent TCP socket, systemd status, and HTTP probes must verify state change.
5. **Defense-in-Depth Command Guard**: Raw command fallback (`execute_command`) is subjected to tokenization and AST inspection to block destructive calls (`rm -rf /`, `mkfs`, `fdisk`, `dd`, `shutdown`, fork bombs).
6. **Least-Privilege Agent**: Runs as a non-root `opspilot` service account with strictly scoped `sudo` privileges configured in `/etc/sudoers.d/opspilot`.

---

## Operational Lifecycle

```
Natural Language Intent
       ↓
Environment Discovery (Host OS, capabilities, ports)
       ↓
AI Planning (Generates typed step plan)
       ↓
Policy & Risk Evaluation (Enforces rules: Read-Only vs Approval)
       ↓
Operator Approval Gate (Plan v1, step risk breakdown)
       ↓
Execution (Dispatched via outbound gRPC to agent)
       ↓
Observe Results (Stdout/stderr streaming)
       ↓
Re-plan if necessary (Automatic failure diagnosis)
       ↓
Deterministic Verification (systemctl is-active, port probe, HTTP 200)
       ↓
Audit Trail (Immutable append-only record)
       ↓
COMPLETED / VERIFIED
```

---

## Supported MVP Scenarios

Tested and verified against real Ubuntu 22.04 and 24.04 hosts:
1. **Install Nginx and verify custom port (8080)**: package install, configuration update, service restart, TCP port check, and HTTP probe.
2. **Install Docker and verify**: package installation, systemd daemon activation, test socket check.
3. **Restart failed systemd services**: diagnosis and controlled restart.
4. **Diagnose service failures**: extract `journalctl -u <service>` logs, identify root causes.
5. **Detect port contention**: identify processes binding target ports via `ss -tlpn`.
6. **System telemetry**: inspect CPU, memory, disk usage, and load averages.
7. **Locate disk usage sources**: diagnose runaway log files and disk exhaustion.
8. **Docker container crash analysis**: inspect exit codes, OOMKilled events, and container logs.
9. **Safe package management**: install apt packages with non-interactive locks and rollback safeguards.
10. **Configuration modification with backups**: automatic `.bak.<timestamp>` file creation before modification.
11. **Multi-server task orchestration**: dispatch tasks across fleet nodes with aggregated status.

---

## Quickstart (Local Development)

### Prerequisites
- Docker & Docker Compose
- Go 1.23+ (optional for local binary build)
- Python 3.11+ (optional for local AI service testing)
- Node.js 20+ (optional for dashboard dev server)

### 1. Clone & Configure
```bash
git clone https://github.com/yourusername/OpsPilot.git
cd OpsPilot
cp .env.example .env
```

### 2. Start Central Stack via Docker Compose
```bash
docker compose up --build -d
```
Services started:
- **Dashboard**: `http://localhost:3000`
- **Control Plane REST/SSE**: `http://localhost:8080`
- **Control Plane gRPC**: `localhost:9090`
- **AI Service**: `http://localhost:8000`
- **PostgreSQL**: `localhost:5432`

### 3. Enroll a Server Agent (Ubuntu 22.04 / 24.04)
On your managed Ubuntu host:
```bash
# Run the automated installer
curl -sSL http://<CONTROL_PLANE_IP>:8080/scripts/install-agent.sh | sudo bash -s -- \
  --server <CONTROL_PLANE_IP>:9090 \
  --token opspilot-default-bootstrap-token-2026
```

---

## Benchmark Evaluation
 
 OpsPilot includes an offline JSON scenario evaluation harness in `benchmarks/` to measure:
 - **Diagnosis Accuracy**: Correctly identifying fault root causes from logs
 - **Plan Generation Quality**: Structuring typed remediation steps
 - **Static Command Safety**: Rejection of forbidden command patterns
 
 > **Note:** This harness evaluates AI diagnostic accuracy and token safety offline against mock log scenarios; it is a simulation harness and does not replace live-host fault injection testing.
 
 Run the benchmark simulation:
 ```bash
 python benchmarks/runner.py
 ```

---

## Reusable Runbooks

Once an operational pattern is solved and verified through AI reasoning, operators can click **"Save as Runbook"**:
```
Unknown / New Task  → AI Reasoning & Structured Planning
Known / Validated Task → Deterministic Reusable Runbook (Zero LLM overhead)
```

---

## Tech Stack
- **Control Plane**: Go 1.23, PostgreSQL 16, pgx/v5, gRPC, Chi router, JWT, SSE.
- **Server Agent**: Go 1.23, single binary, non-root runner, gRPC outbound mTLS.
- **AI Service**: Python 3.11, FastAPI, Pydantic v2, Ollama / Qwen model provider.
- **Dashboard**: React 18, TypeScript, Vite, Tailwind CSS, Lucide Icons.
- **Infrastructure**: Docker Compose, Systemd.

---

## Contributing

We welcome contributions from the community! OpsPilot follows a lightweight GitHub Flow and Conventional Commits standard.

Please read our [Contributing Guidelines](CONTRIBUTING.md) for branch naming standards, commit message formats, PR workflows, and security hygiene rules before submitting a Pull Request.

---

## License

OpsPilot is released under the [MIT License](LICENSE).

