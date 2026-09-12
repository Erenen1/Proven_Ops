# OpsPilot Architecture Specification

## 1. System Philosophy & Separation of Concerns

OpsPilot bridges high-level intent and low-level Linux systems administration without sacrificing control, safety, or verification.

```
       [ Human Operator ]
               |
               v
     +-------------------+
     |  React Dashboard  |
     +---------+---------+
               | HTTPS (REST / SSE)
               v
     +-----------------------------------------------------------+
     |                    CONTROL PLANE                          |
     |                                                           |
     |   +-------------+   +----------------+   +------------+   |
     |   | Auth / RBAC |   |  Task Engine   |   |   Policy   |   |
     |   |             |   | (State Machine)|   |   Engine   |   |
     |   +-------------+   +-------+--------+   +-----+------+   |
     |                             |                  |          |
     |                     +-------v------------------v------+   |
     |                     |           Orchestrator          |   |
     |                     +-------+------------------+------+   |
     |                             |                  |          |
     |   +-------------------------v-+   +------------v------+   |
     |   |    Verification Engine    |   |    Audit Trail    |   |
     |   +---------------------------+   +-------------------+   |
     +-----------------+-----------------------------------------+
                       |                        |
                       | HTTP (JSON)            | gRPC (mTLS)
                       v                        v
             +-------------------+    +-------------------+
             |    AI SERVICE     |    |   SERVER AGENT    |
             |  (Ollama/Qwen)    |    |  (Ubuntu Host)    |
             | - Planner         |    | - Capabilities    |
             | - Tool Selector   |    | - Typed Tools     |
             | - Diagnostician   |    | - Command Guard   |
             +-------------------+    +-------------------+
```

### Core Tenets
1. **The LLM is an Untrusted Planner**: It generates candidate plans and structured arguments based on system context. It never holds execution credentials or shell access.
2. **The Control Plane is the Authoritative Gate**: All LLM recommendations pass through schema checks, policy filters, capability verification, and user approvals before execution.
3. **The Server Agent is a Restricted Executor**: Runs as a non-privileged user where possible, uses specific typed actions (`systemd`, `apt`, `network`, `docker`), and falls back to strictly guarded commands only when necessary.
4. **Deterministic Verification is Truth**: A command exiting with code 0 is insufficient. Operations are independently verified using state queries (e.g. `systemctl is-active`, TCP socket connection, HTTP response codes).

---

## 2. Component Specifications

### 2.1 Control Plane
- **Technology**: Go 1.23+, PostgreSQL 16 (`pgx/v5`).
- **Endpoints**:
  - `REST API`: Task creation, fleet management, approvals, runbook execution.
  - `SSE (Server-Sent Events)`: Real-time task execution progress, output streaming, logs.
  - `gRPC Server`: Listens on port 9090 for agent connections, heartbeat, step dispatching, and streaming output.
- **State Machine**:
  - `CREATED`: Initial task registered.
  - `DISCOVERING`: Probing target host for distribution, installed tools, ports.
  - `PLANNING`: AI service synthesizing steps from intent and discovery.
  - `WAITING_APPROVAL`: Medium/High risk steps requiring operator sign-off.
  - `EXECUTING`: Dispatched to target agent via gRPC.
  - `OBSERVING`: Evaluating step stdout/stderr.
  - `VERIFYING`: Independent health, port, or status probe.
  - `REPLANNING`: If unexpected state observed, AI re-evaluates.
  - `COMPLETED`: All steps executed and independently verified.
  - `PARTIAL_SUCCESS`: Multi-server task with partial node success.
  - `FAILED`: Step failure or verification failure that cannot be remediated.
  - `CANCELLED`: Operator aborted.
  - `TIMEOUT`: Execution deadline exceeded.

### 2.2 Server Agent
- **Technology**: Go 1.23+, single binary distribution.
- **Connection Model**: Outbound gRPC connection over mTLS to Control Plane. No inbound ports required on managed nodes.
- **System Footprint**: Low memory (<30MB RSS), non-root execution with tightly constrained `sudo` permissions via `/etc/sudoers.d/opspilot`.
- **Typed Tool Catalog**:
  - `get_os_info`, `get_system_info`, `get_cpu_usage`, `get_memory_usage`, `get_disk_usage`, `get_load_average`
  - `list_processes`, `get_process_details`
  - `get_service_status`, `start_service`, `stop_service`, `restart_service`, `enable_service`
  - `get_service_logs`, `get_journal_logs`
  - `check_package`, `install_package`
  - `get_open_ports`, `check_port`, `http_probe`, `dns_lookup`
  - `read_file`, `write_config_file` (with automatic backup & rollback)
  - `docker_info`, `docker_ps`, `docker_logs`, `docker_inspect`
  - `execute_command` (fallback guarded by AST parsing, argument inspection, and timeout limits)

### 2.3 AI Service
- **Technology**: Python 3.11+, FastAPI, Pydantic v2, HTTPX.
- **Model Support**: Ollama (default `qwen2.5:3b` / `qwen2.5:7b`), extensible to any OpenAI-compatible provider.
- **Structured Contracts**: Every request and response conforms to strict Pydantic schemas. Freeform markdown or unparsed text is rejected by the Control Plane.
- **Prompt Isolation**: System instructions, user goals, host facts, and untrusted observations (raw logs/output) are demarcated into distinct boundary tags to prevent prompt injection.

### 2.4 Dashboard
- **Technology**: React 18, Vite, TypeScript, Tailwind CSS.
- **Core Views**:
  - Fleet Overview: Real-time agent status, resource stats, OS versions.
  - Task Timeline: Step-by-step visual execution flow with live status indicators.
  - Approval Gate: Highlighting risk levels (READ_ONLY, LOW, MEDIUM, HIGH) and requiring explicit operator action.
  - Runbooks: Parameterized execution templates saved from validated tasks.
  - Audit Trail: Comprehensive immutable log of every operator action and agent execution.

---

## 3. Communication Protocols

### 3.1 Agent <-> Control Plane
- Protocol: gRPC over TLS (mTLS in production / development enrollment mode).
- Streaming: Bidirectional or long-poll streaming for step dispatch and streaming chunked output (stdout/stderr).

### 3.2 Control Plane <-> AI Service
- Protocol: HTTP/JSON over internal network.
- Payloads: Typed schemas for `PlanRequest`, `PlanResponse`, `ReplanRequest`, and `DiagnosticRequest`.

### 3.3 Dashboard <-> Control Plane
- Protocol: REST (JSON) + Server-Sent Events (SSE) for zero-latency task lifecycle updates.
