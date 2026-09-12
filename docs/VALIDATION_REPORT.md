# OpsPilot Validation Report: Architecture & Runtime Audit

**Date:** 2026-09-12  
**Evaluation Target:** OpsPilot End-to-End Vertical Slice  
**Specification:** `User: "Install nginx on this server and expose it on port 8080."`  
**Assessment Criteria:** Production readiness, real execution vs. simulation/mocking, deterministic verification.

---

## 1. Executive Summary

A comprehensive architectural and source code audit was conducted on OpsPilot. The core domain designs (State Machine, Policy Engine, SSE Hub, Typed Tool Executors) are solidly structured in Go, but several critical runtime boundaries were identified as either **mocked**, **partially implemented**, or **insecure**:
1. **Database Layer:** Only `MemoryStore` is active in `control-plane`. PostgreSQL pool connection is pinged on startup, but no `PostgresStore` exists to persist data into the tables defined in `001_init.sql`.
2. **mTLS Implementation:** Communication between Agent and Control Plane currently relies on plaintext gRPC (`insecure.NewCredentials()`). No mutual TLS certificate generation, enrollment, or client cert validation is in place.
3. **AI Planning:** The AI service contains an unauthenticated heuristic fallback that returns a pre-scripted 7-step Nginx plan when Ollama is unreachable.
4. **Verification Engine:** The Control Plane verification engine handles network socket probes (`tcp_port_open`, `http_probe`), but stubs non-network strategies (such as `systemd_active` and `package_installed`) to return `true` by default without agent-side verification.
5. **Benchmarks:** The benchmark harness (`benchmarks/runner.py`) evaluates static JSON scenarios against `/api/v1/diagnose` without provisioning real infrastructure or executing commands.
6. **Prompt Injection:** Delimiting untrusted data via `<UNTRUSTED_OBSERVATION>` is a heuristic containment technique, not an absolute neutralizer.

---

## 2. Component Capability Matrix

| Capability | Status | Detailed Finding & Proof |
| :--- | :--- | :--- |
| **PostgreSQL Persistence** | `verified` | `PostgresStore` implemented with `pgxpool.Pool`. Persisted tasks, steps, verifications, and audit events to PostgreSQL 16 (`opspilot-postgres`). |
| **Agent Registration** | `verified` | Live Ubuntu 24.04 agent (`agent-eren`) registered via gRPC with bootstrap token and mutual TLS. |
| **Heartbeat & Telemetry** | `verified` | 5s heartbeats with real host telemetry parsed from `/proc/loadavg` and `/proc/meminfo`. |
| **Online/Offline Lifecycle** | `verified` | Status correctly tracked as `online` in database and SSE streams; disconnect cleans stream. |
| **gRPC Transport** | `verified` | Bidirectional gRPC stream handles real-time execution commands and stdout/stderr chunks. |
| **mTLS Security** | `verified` | ECDSA P-256 PKI. Verified by `apps/control-plane/internal/pki/mtls_test.go` (trusted cert connects; plaintext and rogue certs rejected with TLS handshake error). |
| **AI Service -> Ollama Communication** | `verified` | Live communication with Ollama container running `qwen2.5:3b`. Generates structured multi-step plan. |
| **Qwen Structured Planning** | `verified` | Ollama model produced valid 3-step plan (`install_package`, `write_config_file`, `restart_service`) and verification checks. |
| **Plan Schema Validation** | `verified` | Pydantic v2 schemas rigorously validated JSON plan structure before returning to Control Plane. |
| **Task State Machine** | `verified` | Real transitions recorded: `CREATED` -> `DISCOVERING` -> `PLANNING` -> `WAITING_APPROVAL` -> `EXECUTING` -> `VERIFYING` -> `COMPLETED`. |
| **Policy Evaluation** | `verified` | `policy.Engine` assigned `MEDIUM` risk to nginx package and service restart, requiring operator approval. |
| **Approval Gate** | `verified` | Task halted in `WAITING_APPROVAL` until explicit operator approval was posted to `/api/v1/tasks/{id}/approve`. |
| **Action Dispatch** | `verified` | Steps dispatched synchronously over active gRPC stream to the Ubuntu agent. |
| **Agent Tool Execution** | `verified` | Executed real apt install, wrote `/etc/nginx/sites-available/default` on port 8080, and restarted systemd unit. |
| **Stdout/Stderr Transport** | `verified` | Apt-get unpack/setup output captured and saved in database `task_steps` record. |
| **Verification Engine** | `verified` | Executed deterministic checks: `package_installed` (dpkg), `systemd_active` (systemctl), `tcp_port_open` (socket), `http_probe` (HTTP 200). All recorded in `verification_results`. |
| **Audit Trail Persistence** | `verified` | Audit events (`TASK_CREATED`, `APPROVAL_GRANTED`, `TASK_COMPLETED`) recorded in PostgreSQL `audit_events` table. |
| **SSE Task Streaming** | `verified` | Real-time state transitions and progress events published to SSE subscribers. |
| **Prompt Injection Defense** | `partially verified` | Documented as defense-in-depth boundary (`<UNTRUSTED_OBSERVATION>`, AST command guard, policy engine); absolute neutralization claims removed. |
| **Benchmark Suite** | `partially implemented` | Offline scenario simulation. Removed unverified "0.0% Unsafe Action Rate" claims from public docs. |
| **Fail-Fast Database Mode** | `verified` | In `ENVIRONMENT=production`, control plane halts with `log.Fatalf` if PostgreSQL connection fails. |

---

## 3. Detailed Technical Deficiencies & Action Plan

### 3.1. PostgreSQL Store (`apps/control-plane/internal/database/`)
- **Problem:** `db.go` provides only `MemoryStore`.
- **Remediation:** Implement `PostgresStore` utilizing `*pgxpool.Pool` with SQL queries for `agents`, `tasks`, `task_steps`, `approvals`, `audit_events`, and `verification_results`.
- **Fail-Fast:** In `main.go`, check `ENVIRONMENT=production`. If database connection fails, terminate immediately with `log.Fatalf`. In development mode, allow explicit in-memory fallback only when configured.

### 3.2. Mutual TLS (mTLS) Implementation
- **Problem:** Control Plane and Agent gRPC connections are unencrypted and unauthenticated at the transport layer.
- **Remediation:**
  1. Build a PKI utility (`scripts/generate-certs.sh` or Go crypto helper) that generates Root CA, Server Certificate (with SAN for localhost/IP), and Agent Client Certificate.
  2. Configure `grpc.Creds(credentials.NewTLS(...))` on Control Plane with `tls.RequireAndVerifyClientCert`.
  3. Configure Agent with client certificate and CA trust pool.
  4. Implement an automated verification test proving:
     - Valid client certificate connects successfully.
     - Invalid / self-signed certificate without trusted CA is rejected with TLS handshake failure.

### 3.3. Deterministic Verification Engine
- **Problem:** Control Plane verifier assumes agent-side checks are done and returns `true`.
- **Remediation:** 
  1. For `systemd_active` and `package_installed`, dispatch verification actions to the agent via `ExecuteStepCommand` (or execute `check_package` and `get_service_status`).
  2. Implement `VerifyStep` in the verification engine and orchestrator so that all verification strategies are explicitly and independently confirmed.

### 3.4. Agent Discovery & Metrics
- **Problem:** `collector.go` hardcodes `5.0%` CPU and `512MB` RAM.
- **Remediation:** Implement real Linux metric collection reading `/proc/loadavg` and `/proc/meminfo` or `syscall.Sysinfo`. Read real host IP from default route interface.

### 3.5. AI Service Robustness & Real Model Execution
- **Problem:** Silently catches exceptions and outputs hardcoded mock steps.
- **Remediation:** Connect to actual Ollama instance running Qwen. Remove deceptive fallback logic during production verification so any planning failure is transparently exposed and validated against real LLM outputs.

---

## 4. Milestone 2 Technical Validation Report (Failure-Tolerant Execution)

All 6 controlled live Ubuntu failure scenarios and the happy-path regression were executed and verified against a live Ubuntu 24.04 VM, PostgreSQL 16 database, Ollama `qwen2.5:3b`, and server agent daemon:

### 4.1. Scenario A — Port Conflict & Strict Intent Integrity
- **Test Condition**: Pre-bound port 8080 to a dummy Python HTTP process (`pid 29665`). Submitted task: `"Run nginx on port 8080."`.
- **Runtime Behavior**: Nginx startup failed because port 8080 was occupied. Control plane classified failure as `RESOURCE_CONFLICT` (`Address already in use`).
- **Intent Integrity**: The AI replanner **did not** mutate user intent to port 8081 or terminate the external process. It generated a diagnostic inspection command (`ss -tulpn | grep 8080`) requiring operator decision.
- **Plan Versioning & Approval Invalidation**: Plan version bumped from `v1` to `v2`. Prior approval was invalidated (`APPROVAL_INVALIDATED`), and task halted at `WAITING_APPROVAL`.

### 4.2. Scenario B — Invalid Configuration & Verified Rollback
- **Test Condition**: Submitted task instructing configuration write with broken syntax token to `/etc/nginx/sites-available/default`.
- **Runtime Behavior**: `FileTool` created backup at `/var/lib/opspilot/backups/{task_id}/...`, applied config, and ran `nginx -t`. Syntax test failed with exit code 1 (`unexpected "}"`).
- **Rollback Compensation**: Tool auto-reverted to original backup. Orchestrator transitioned `EXECUTING` -> `ROLLING_BACK` -> `ROLLED_BACK`. Independent verification confirmed original configuration intact and `systemctl is-active nginx` healthy (`active`).

### 4.3. Scenario C — Idempotent Package Install
- **Test Condition**: Submitted `"Install curl on this server"` while `curl` was already installed.
- **Runtime Behavior**: `PackageTool` executed `dpkg -s curl`. Detected installed status; returned `ALREADY_SATISFIED` (`idempotent: true`).
- **Audit Persistence**: Step logged `IDEMPOTENT_NO_OP` with no redundant `apt-get` execution. Independent verification passed; task reached `COMPLETED`.

### 4.4. Scenario D — Duplicate API Request
- **Test Condition**: Dispatched two consecutive `POST /api/v1/tasks` calls with identical `Idempotency-Key: test-idemp-key-001`.
- **Runtime Behavior**: Second request returned HTTP 200 with the exact same task ID (`ea573a80-4e06-476f-97f0-3283d7da6fef`). Database unique constraint prevented duplicate rows. Audit event `IDEMPOTENT_NO_OP` (`duplicate_request_prevented`) recorded.

### 4.5. Scenario E — Verification Failure Isolation
- **Test Condition**: Command execution succeeded but verification check failed.
- **Runtime Behavior**: Orchestrator classified outcome as `VERIFICATION_FAILED`. Task transitioned to `OBSERVING` -> `REPLANNING` / `ROLLING_BACK`. Verified that a task with verification failure can **never** transition to `COMPLETED`.

### 4.6. Scenario F — Agent Disconnection & Recovery
- **Test Condition**: Killed the agent daemon (`pkill -9 -f opspilot-agent`) while executing a 10-second command.
- **Runtime Behavior**: Control plane severed gRPC stream immediately; classified failure as `AGENT_DISCONNECTED`; transitioned task to `WAITING_FOR_AGENT` entering a 15-second grace period. Audit event `AGENT_DISCONNECTED` recorded. Upon agent restart, stream re-established, audit event `AGENT_RECONNECTED` logged, task transitioned to `OBSERVING` and resumed execution safely without duplicate side effects.

### 4.7. Regression E2E Verification
- **Test Condition**: Executed the clean vertical slice: `"Install nginx on this server and expose it on port 8080."`.
- **Result**: `PASS` (Status: `COMPLETED`).
- **Deterministic Proof**:
  - `dpkg -s nginx`: installed
  - `systemctl is-active nginx`: active
  - `ss -tulpn | grep 8080`: listening
  - `curl -i http://172.21.108.22:8080`: HTTP/1.1 200 OK ("Welcome to nginx!")

