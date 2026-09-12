# OpsPilot Product & Business Logic Specification

## 1. Product Mission & Purpose

OpsPilot transforms natural-language infrastructure intentions (e.g. *"Install nginx on server X and expose it on port 8080"*) into planned, policy-controlled, verifiable operations on managed Linux hosts.

The primary product thesis: **Infrastructure administration requires strict safety boundaries, human sign-off on risky mutations, and deterministic state verification.**

---

## 2. User Roles & Permissions (RBAC)

| Role | Permissions | Responsibilities |
| :--- | :--- | :--- |
| **Admin** | Full system access | Manage policies, enroll agents, manage users, approve all tasks, execute runbooks |
| **Operator** | Task execution & approvals | Create tasks, review and approve low/medium operations, trigger runbooks |
| **Viewer** | Read-only access | View fleet health, inspect live task timelines, view audit logs |

---

## 3. Core User Workflows

### 3.1 Natural Language Task Lifecycle
1. **Intent Submission**: Operator inputs natural-language prompt via Dashboard or REST API and selects target server(s).
2. **Discovery**: Control plane polls the target agent for distribution, version, active services, and open ports.
3. **AI Planning**: AI Service returns a structured JSON plan with suggested typed actions and verification strategies.
4. **Policy & Risk Evaluation**: Control plane calculates risk level (`READ_ONLY`, `LOW`, `MEDIUM`, `HIGH`, `FORBIDDEN`) based on the static policy registry.
5. **Approval Gate**: If any step is `MEDIUM` or `HIGH`, the task pauses in `WAITING_APPROVAL` until an operator approves or rejects Plan Version N.
6. **Execution**: Control plane dispatches steps sequentially to the target agent via outbound gRPC streaming.
7. **Deterministic Verification**: Socket dials and HTTP probes independently confirm expected state (e.g. port open, HTTP 200).
8. **Completion**: Task transitions to `COMPLETED` and an immutable audit log entry is written.

### 3.2 Reusable Runbooks
- **Philosophy**: Unknown/New task → AI reasoning; Known/Validated task → Deterministic runbook.
- When an operator completes and verifies a task, they can click **"Save as Runbook"**.
- Subsequent executions run directly through the deterministic step sequence without invoking the LLM, eliminating token cost and latency.

---

## 4. Key Business & Validation Rules

1. **Model Self-Report Discard**: The LLM's suggested risk level is purely advisory. The Control Plane Policy Engine always computes and enforces the true risk level.
2. **Plan Invalidation**: If an execution plan is revised or replanned, previous approvals become invalid (`plan_version` increments).
3. **No Unverified Completion**: Exit code 0 is insufficient. Operations modifying services or ports must pass independent deterministic verification before transitioning to `COMPLETED`.
4. **Credential Redaction**: Passwords, tokens, private keys, and authorization headers in stdout/stderr must be redacted before recording in `audit_events`.
