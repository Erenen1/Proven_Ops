# OpsPilot — Failure Recovery & Deterministic Execution Architecture

OpsPilot operates on a core engineering principle:
> **"Observe → Plan → Validate → Approve → Execute → Verify → Recover if necessary → Verify recovery"**
>
> The LLM is an untrusted, probabilistic planner. Infrastructure state, policy boundaries, and failure recovery MUST BE deterministic, auditable, and mathematically bound.

---

## 1. Structured Failure Taxonomy

Raw free-text error management is prohibited. All operational failures are classified into the `StructuredFailure` model:

```json
{
  "type": "RESOURCE_CONFLICT",
  "action": "restart_service",
  "target": "nginx",
  "message": "Resource conflict detected during 'restart_service': port or socket already in use",
  "retryable": false,
  "requires_replan": true,
  "rollback_required": false,
  "user_action_required": true,
  "details": {
    "exit_code": 1,
    "error": "RESOURCE_CONFLICT"
  }
}
```

### Classification Hierarchy

| Failure Type | Description | Retryable? | Replan? | Rollback? | Operator Action? |
| :--- | :--- | :---: | :---: | :---: | :---: |
| `RESOURCE_CONFLICT` | Port/socket already bound (e.g. `Address already in use`, `bind() failed`). | No | Yes | No | **Yes** (Intent preserved) |
| `VALIDATION_FAILED` | Configuration syntax checks (`nginx -t`) or file schema verification failed. | No | Yes | **Yes** | No |
| `VERIFICATION_FAILED` | Command exited 0, but independent verifier (`systemd_active`, `tcp_port_open`, `http_probe`) failed. | No | Yes | **Yes** | No |
| `AGENT_DISCONNECTED` | Transport stream severed mid-execution (`connection reset`, `stream EOF`). | No (Grace) | No | No | No (Auto-observe on reconnect) |
| `DEPENDENCY_FAILURE` | Transient network timeouts, DNS failure, apt mirror timeouts. | **Yes** | No | No | No |
| `PERMISSION_DENIED` | Insufficient capabilities (`are you root?`, `operation not permitted`). | No | Yes | No | Yes |
| `INVALID_MODEL_OUTPUT` | Ollama timeout, malformed JSON, schema validation failure from AI Service. | No | No | No | Yes (Executor isolated) |
| `TIMEOUT` | Action exceeded execution deadline. | No | Yes | No | No |
| `POLICY_DENIED` | Action denied by control plane policy engine (`RiskForbidden`). | No | No | No | Yes |
| `UNSUPPORTED_OPERATION`| Unknown tool action requested. | No | No | No | Yes |
| `CANCELLED` | Task terminated by operator. | No | No | No | No |
| `EXECUTION_FAILED` | Generic unhandled command non-zero exit code. | No | Yes | No | No |

---

## 2. Retry Policy & Anti-Side-Effect Guard

OpsPilot strictly forbids blind retries of side-effect operations.
- **Allowed for Retry**: Read-only actions (`check_package`, `get_service_status`, `read_file`) and transient network errors (`DEPENDENCY_FAILURE`, network timeouts).
- **Prohibited from Retry**: Mutating operations (`install_package`, `write_config_file`, `restart_service`, `start_service`, `execute_command`).

```go
func (p *RetryPolicy) IsRetryable(sf *models.StructuredFailure, action string, currentAttempt int) bool {
    if currentAttempt >= p.MaxAttempts { return false }
    if sf.Type == models.FailureDependencyFailure && isReadOnlyAction(action) { return true }
    return false
}
```
All retry attempts emit `RETRY_ATTEMPTED` audit events with exponential backoff delay.

---

## 3. Controlled & Bounded Replanning

When a non-retryable failure occurs, the task state machine transitions:
`EXECUTING` → `OBSERVING` → `REPLANNING`

### Replan Guardrails
1. **Bounded Iterations**: Maximum replans per task is bounded by `task.MaxReplans` (default: 3). If exceeded, the task terminates immediately with status `FAILED` and audit event `REPLAN_LIMIT_REACHED`.
2. **Strict Intent Integrity**: The AI Replanner is constrained by prompt boundaries and schema validation. If a port conflict occurs (e.g. port 8080 occupied), the agent **must not** silently alter the intent to port 8081 or terminate unknown processes. It must generate diagnostic observation steps or halt for operator decision.
3. **Plan Versioning (`plan_version`)**: Every replanning cycle increments `task.PlanVersion` (e.g., v1 → v2).
4. **Approval Invalidation**: Approvals granted for Plan v1 are automatically invalidated. If Plan v2 introduces modifying actions or `MEDIUM`/`HIGH` risk, the task enters `WAITING_APPROVAL`, emitting `APPROVAL_INVALIDATED` to the audit trail.

---

## 4. Transactional Configuration & Rollback Compensation

Config file changes follow a transactional lifecycle:
`Backup` → `Write` → `Validate (nginx -t)` → `Revert on Failure` → `Independent Verification` → `ROLLED_BACK`

1. **Pre-flight Backup**: Before writing to any config file, `FileTool` backs up the existing content to `/var/lib/opspilot/backups/{task_id}/...`.
2. **In-tool Atomic Validation**: `FileTool` invokes validation commands (`nginx -t`). If validation fails, it immediately restores the backup, deletes temporary state, and reports validation failure.
3. **Control Plane Compensation**:
   - Transition to `ROLLING_BACK`.
   - Dispatch `rollback_config` tool actions for all modified paths.
   - Restart affected services (`systemctl restart nginx`).
   - Run independent verification (`systemd_active nginx`).
   - Transition to `ROLLED_BACK` (or `ROLLBACK_FAILED` if verification fails).

---

## 5. Multi-Layer Idempotency

OpsPilot implements idempotency across three architectural tiers:

### Tier 1: API Level (`Idempotency-Key`)
- Client passes `Idempotency-Key: <unique-uuid>` on `POST /api/v1/tasks`.
- PostgreSQL enforces a unique constraint on `tasks(idempotency_key)`.
- If an identical key is sent, the API returns HTTP 200 with the existing task and logs an `IDEMPOTENT_NO_OP` (`duplicate_request_prevented`) audit event. No secondary infrastructure task is scheduled.

### Tier 2: Agent Step Execution Cache
- Every dispatched step receives an execution ID: `<task_id>/<step_id>/<attempt>`.
- The agent executor maintains an execution cache (`sync.Map`). If network retransmits the same step execution request, the agent returns the cached result without repeating the underlying side effect.

### Tier 3: Tool-Level Idempotency Pre-Checks
- `install_package`: Checks `dpkg -s <package>`. If installed, returns `ALREADY_SATISFIED` (`idempotent: true`).
- `write_config_file`: Computes SHA256 of target file. If identical to proposed content, returns `ALREADY_SATISFIED` without writing to disk.
- `start_service`: Checks `systemctl is-active`. If active, returns `ALREADY_SATISFIED`.

---

## 6. Agent Disconnection & Graceful Recovery

When the agent's outbound mTLS gRPC connection is severed mid-step:
1. The Control Plane server immediately notifies any blocked step dispatch channels.
2. The task transitions to `WAITING_FOR_AGENT` ("Agent disconnected during execution; entering grace period") and records an `AGENT_DISCONNECTED` audit event.
3. The Control Plane holds execution for a 15-second grace period without dispatching duplicate modifying commands.
4. When the agent reconnects, it re-registers over mTLS, publishes `AGENT_RECONNECTED`, transitions the task to `OBSERVING`, re-probes system state, and resumes safely.
5. If the grace period expires, the task fails deterministically with `agent_disconnect_timeout`.
