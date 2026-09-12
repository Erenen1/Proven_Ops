---
description: Always-on project memory lifecycle and context synchronization protocols
always_on: true
---

# Context Memory Protocol

Chat history is ephemeral. The repository itself carries the permanent memory of the project.

---

## 1. Session Start Protocol (Progressive Disclosure)

When starting any new session, do **not** read the entire codebase. Follow this 3-tier loading sequence:

### Tier 1 — Global Baseline (Every Session)
1. Read root [GEMINI.md](file:///c:/Users/ern/Documents/Ops_Pilot/GEMINI.md) (Architecture overview & critical constraints).
2. Read [docs/context/CURRENT_STATE.md](file:///c:/Users/ern/Documents/Ops_Pilot/docs/context/CURRENT_STATE.md) (Current development focus & recent state).
3. Read [docs/context/SESSION_HANDOFF.md](file:///c:/Users/ern/Documents/Ops_Pilot/docs/context/SESSION_HANDOFF.md) (Last session's notes and exact next step).

### Tier 2 — Task-Specific Deep Dive (As Needed)
- Architecture or integration work → Read [docs/context/ARCHITECTURE.md](file:///c:/Users/ern/Documents/Ops_Pilot/docs/context/ARCHITECTURE.md) & [docs/context/DECISIONS.md](file:///c:/Users/ern/Documents/Ops_Pilot/docs/context/DECISIONS.md).
- User workflow, RBAC, or business logic → Read [docs/context/PRODUCT.md](file:///c:/Users/ern/Documents/Ops_Pilot/docs/context/PRODUCT.md).
- Debugging, performance, or known quirks → Read [docs/context/KNOWN_ISSUES.md](file:///c:/Users/ern/Documents/Ops_Pilot/docs/context/KNOWN_ISSUES.md).

### Tier 3 — Source Code Inspection
Only after reading the relevant memory documents, inspect the specific source files needed for the task.

---

## 2. Context Synchronization Protocol

Whenever code or system configuration changes, evaluate and update the corresponding context file:

| Event / Trigger | Target Document | Action Required |
| :--- | :--- | :--- |
| **System component, gRPC proto, or boundary changed** | `docs/context/ARCHITECTURE.md` | Update topology, component table, or data flow |
| **User role, permission, or workflow altered** | `docs/context/PRODUCT.md` | Update business rules, state rules, or validation logic |
| **Architectural decision or irreversible choice made** | `docs/context/DECISIONS.md` | Record a new ADR entry |
| **Milestone achieved or focus shifted** | `docs/context/CURRENT_STATE.md` | Update Focus, Completed, and Next sections |
| **New bug, tech debt, or risk uncovered** | `docs/context/KNOWN_ISSUES.md` | Log root cause, symptoms, or workarounds |
| **Session ending or handing off work** | `docs/context/SESSION_HANDOFF.md` | Overwrite with the latest state and exact next step |

---

## 3. Context Hygiene Rules

- **High Signal, Low Token**: Keep files compact. Avoid pasting large generated code blocks or multi-page changelogs into context docs.
- **Never Duplicate**: Reference files rather than copying identical descriptions into multiple documents.
- **Overwrite Handoffs**: `SESSION_HANDOFF.md` is a snapshot of the *latest* handoff, not an append-only log. Overwrite it.
- **Verify Stale Data**: Never trust a context doc blindly if the source code contradicts it. Validate with runtime/tests and fix the doc.
