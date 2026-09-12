---
name: project-handoff
description: >-
  Prepares a clean session handoff by inspecting git status, verifying test passes,
  and updating docs/context/SESSION_HANDOFF.md and CURRENT_STATE.md at the end of a development session.
---

# Project Handoff Procedure

Use this skill at the conclusion of a development session or when pausing work so that the next session can immediately resume with zero context loss.

---

## Steps to Execute Handoff

### 1. Inspect Working Tree
Run git status to determine modified files and unstaged work:
```bash
git status -s
git diff --stat
```

### 2. Verify System Health & Run Tests
Execute the local test suite to verify whether the codebase is healthy:
- Go Control Plane & Agent: `go test -v ./...`
- Python AI Service: `pytest`
- Dashboard: `npm run build`

### 3. Evaluate Context Updates
Review if any domain context changed during the session:
- Did system architecture change? → Update [docs/context/ARCHITECTURE.md](../../docs/context/ARCHITECTURE.md)
- Did product workflows or RBAC rules change? → Update [docs/context/PRODUCT.md](../../docs/context/PRODUCT.md)
- Was a major technical decision made? → Add ADR in [docs/context/DECISIONS.md](../../docs/context/DECISIONS.md)
- Did the high-level milestone progress? → Update [docs/context/CURRENT_STATE.md](../../docs/context/CURRENT_STATE.md)
- Was a technical debt or quirk uncovered? → Log in [docs/context/KNOWN_ISSUES.md](../../docs/context/KNOWN_ISSUES.md)

### 4. Overwrite SESSION_HANDOFF.md
Overwrite [docs/context/SESSION_HANDOFF.md](../../docs/context/SESSION_HANDOFF.md) with the standard template:
- **Status**: `COMPLETE` | `ACTIVE` | `BLOCKED`
- **Session Goal**: What was worked on
- **What Was Done**: Concrete accomplishments
- **Files Touched**: Exact file paths
- **Current State**: Exact status of the implementation
- **Remaining Work**: Unfinished tasks
- **Exact Next Step**: The very first concrete action the next agent should take
- **Verification Results**: Status of builds and tests
- **Resume Instructions**: One-sentence direct instruction for resuming

*Never append multiple handoffs; always overwrite with the most recent single snapshot.*
