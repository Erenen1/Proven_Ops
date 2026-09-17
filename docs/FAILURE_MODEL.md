# ProvenOps Failure & Recovery Model

## 1. Failure Philosophy
In distributed infrastructure operations, failures are inevitable: network partitions occur, services encounter port collisions, agents crash, and host machines restart. ProvenOps guarantees that no failure leaves the infrastructure in an undefined, unverified, or corrupted state.

---

## 2. Standard 7-Class Error Taxonomy

Every execution failure is categorized into one of 7 deterministic error classes:

| Error Class | Definition | Retry Policy | Example |
| :--- | :--- | :--- | :--- |
| **`TRANSIENT`** | Temporary resource contention or lock | Exponential backoff with jitter (max 3 attempts) | `/var/lib/dpkg/lock held`, temporary socket exhaustion |
| **`CONNECTIVITY`** | Network or gRPC stream disruption | Retry with jitter up to max attempts | `connection refused`, DNS resolution timeout |
| **`TIMEOUT`** | Operation exceeded allocated deadline | Read-only probes retried; mutating actions fail-fast | `apt update` socket timeout, HTTP probe timeout |
| **`POLICY`** | Action denied by policy engine or AST guard | **NEVER** retried (Fail-Fast) | `rm -rf /` blocked, unauthorized package install |
| **`PERMANENT`** | Invalid configuration or missing binary | **NEVER** retried (Fail-Fast) | Command not found, invalid syntax in config |
| **`VERIFICATION`** | Postcondition check failed | Triggers Saga Rollback or Remediation | Port not listening, HTTP returned 502 instead of 200 |
| **`UNKNOWN`** | Unrecognized error condition | Safe probe retried once; mutating actions fail-fast | Unclassified OS error |

---

## 3. Agent Disconnect & Crash Recovery

```
[ Agent Executing ] ➔ [ Process Killed / Power Loss ] 
       │
       ▼
[ Control Plane detects stream disconnect ] ➔ State marked: EXECUTION_UNKNOWN (NO blind fail or retry!)
       │
       ▼
[ Agent Restarts & Reconnects ]
       │
  1. Reads local bbolt ledger (/var/lib/provenops/execution_ledger.db)
  2. Any in-flight RUNNING records converted to UNKNOWN
  3. Control Plane initiates reconciliation handshake:
     - If step succeeded before crash: cached result returned (Idempotent)
     - If step was mutating and state is uncertain: flagged UNCERTAIN_EXECUTION requiring precheck
```

---

## 4. Saga LIFO Compensation & Rollback Engine

When a task fails deterministic postcondition verification, previously succeeded mutating steps are compensated in reverse (Last-In, First-Out) order:

```
Step 1: ensure_package(nginx) ➔ [SUCCEEDED]
Step 2: ensure_file(/etc/nginx/nginx.conf) ➔ [SUCCEEDED] (Backup snapshot taken)
Step 3: ensure_service(nginx, running) ➔ [VERIFICATION FAILED! (Port 80 conflict)]
══════════════════════════════════════════════════════════════════════════════
AUTOMATED SAGA LIFO ROLLBACK INITIATED:
  1. Revert Step 2: Restore /etc/nginx/nginx.conf from pre-flight backup snapshot
  2. Revert Step 1: ensure_package(nginx, absent)
  3. Verify Rollback: Confirm restored configuration validity
Rollback Outcome: ROLLBACK_SUCCEEDED
```

### Reversibility Levels:
* **`FULL`**: Guaranteed lossless automatic reversion (e.g., `ensure_file` via backup snapshot, `ensure_service` via state toggle).
* **`PARTIAL`**: Best-effort system cleanup (e.g., `ensure_package` removes target package, but system dependencies may remain).
* **`NONE`**: Irreversible operations (e.g., raw `execute_command`). High-risk irreversible actions trigger `MANUAL_INTERVENTION_REQUIRED`.

---

## 5. Bounded Closed-Loop Remediation

When enabled, verification failures can invoke the AI diagnostic service:
* **Remediation Budget Limits:**
  * `max_attempts`: 2
  * `max_risk`: `MEDIUM`
  * `max_runtime_sec`: 300
* **Safety Invariant:** Remediating plans must pass the exact same Schema Validation, Policy Engine, and Verification contracts. When budget is exhausted, state escalates to `MANUAL_INTERVENTION_REQUIRED`.
