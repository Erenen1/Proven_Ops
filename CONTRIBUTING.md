# Contributing to ProvenOps

Thank you for your interest in contributing to **ProvenOps**! 

ProvenOps is an open-source, policy-controlled AI infrastructure operations platform. Because this platform interacts directly with server administration, security and architectural integrity are our highest priorities.

We follow a lightweight **GitHub Flow / Trunk-Based Development** approach. This document outlines our branch strategy, commit conventions, pull request workflows, and security requirements.

---

## 1. Branch Strategy

The `main` branch is our production-ready, always-deployable source of truth.

- **Never commit directly to `main`**. All work happens on short-lived feature or bugfix branches and merges to `main` via Pull Requests.
- **Do not create long-lived `develop` branches**. Keep branches small, focused, and short-lived.

### Branch Naming Conventions
Branch names must be **lowercase**, **kebab-case**, concise, and descriptive of the work being done.

Format:
```
<type>/<short-description>
```

| Type | Purpose | Example |
| :--- | :--- | :--- |
| `feat/` | New feature or capability | `feat/agent-registration`, `feat/task-state-machine` |
| `fix/` | Bug fix | `fix/agent-heartbeat-timeout`, `fix/port-parsing` |
| `refactor/` | Code refactoring without behavior change | `refactor/policy-engine-rules` |
| `docs/` | Documentation improvements | `docs/security-model-update` |
| `test/` | Adding or updating tests | `test/task-state-machine` |
| `chore/` | Tooling, dependencies, or maintenance | `chore/upgrade-pgx` |
| `ci/` | CI/CD pipeline changes | `ci/add-github-actions-cache` |
| `security/` | Security hardening or vulnerability remediation | `security/command-validation-guard` |

> ❌ **Avoid vague branch names**: `test`, `new`, `changes`, `my-branch`, `feature1`, `final`, `final-final`.

---

## 2. Commit Message Standard

We strictly follow the [Conventional Commits](https://www.conventionalcommits.org/) standard.

Format:
```
<type>(optional-scope): <description>
```

### Supported Types
- `feat`: A new feature
- `fix`: A bug fix
- `refactor`: Code restructuring without changing behavior
- `docs`: Documentation only
- `test`: Adding or modifying tests
- `chore`: Routine maintenance, updating dependencies
- `ci`: CI/CD configuration
- `build`: Changes affecting build system or external dependencies
- `perf`: Code changes that improve performance
- `security`: Security patches, vulnerability mitigations, hardening

### Good Examples
```
feat(agent): implement outbound gRPC heartbeat mechanism
feat(control-plane): implement explicit task state machine
security(executor): reject destructive shell commands
refactor(policy): simplify risk tier evaluation logic
fix(agent): handle automatic reconnection after connection loss
test(tasks): add invalid state transition test suite
docs: document agent bootstrap enrollment flow
ci: add Go multi-architecture build verification
```

### Commit Granularity
- **Make commits atomic**: Each commit should represent a single logical unit of change.
- **Avoid omnibus commits**: Do not combine authentication, agent heartbeats, database schema, and dashboard UI into one giant commit (`feat: everything`). Split them logically.
- ❌ **Do not use vague commit messages**: `update`, `changes`, `fix stuff`, `done`, `working`, `wip`, `test123`.

---

## 3. Pull Request Workflow

```
main
  ↓
git checkout -b <type>/<description>
  ↓
Local Development & Testing
  ↓
git push origin <type>/<description>
  ↓
Open Pull Request (using PR Template)
  ↓
Automated CI Checks Run
  ↓
Code Review
  ↓
Squash and Merge to main
  ↓
Delete Feature Branch
```

### Squash and Merge
We use **Squash and Merge** as the default merge strategy.
- During development on your branch, you may create multiple iterative commits.
- When merging to `main`, squash all branch commits into a single clean Conventional Commit:
  ```
  feat(agent): implement secure outbound enrollment (#42)
  ```
This ensures our `git log --oneline` on `main` remains clean, linear, and easy to audit months later.

---

## 4. Main Branch Protection (Recommended Settings)

For repository maintainers, configure GitHub repository branch protection for `main`:
1. **Require a pull request before merging**: Prevent direct pushes to `main`.
2. **Require status checks to pass before merging**: Ensure GitHub Actions (`test-go`, `test-python`, `test-frontend`) succeed.
3. **Require branches to be up to date before merging**: Avoid merge skew.
4. **Do not allow bypassing the above settings**.
5. **Block force pushes** and **block branch deletions** on `main`.

*Note: For early single-maintainer phases, mandatory review approvals can remain optional, but require at least 1 approval + `CODEOWNERS` as soon as external contributors participate.*

---

## 5. Security & Secret Hygiene

Because OpsPilot interacts directly with operating system commands and infrastructure, **repository security is non-negotiable**.

### Absolute Rules
- **NEVER commit sensitive credentials**:
  - Passwords (database, admin, etc.)
  - JWT secrets or signing keys
  - TLS private keys, certificates, or `.pem`/`.p12` bundles
  - Agent bootstrap tokens
  - API keys or LLM provider tokens
  - SSH private keys
  - Local `.env` files
- Always use `.env.example` containing dummy template values.
- Verify files with `git status` and `git diff --staged` before committing.

### What to do if a secret is accidentally committed
> ⚠️ **Removing a secret in a subsequent commit DOES NOT delete it from Git history.**

If a credential is accidentally committed:
1. **Immediately revoke / rotate the secret** in your infrastructure. Consider it compromised.
2. Purge the secret from your local Git history using `git filter-repo` or BFG Repo-Cleaner before pushing.
3. If already pushed to remote, notify the repository maintainers immediately.

---

## 6. Keeping Documentation in Sync

ProvenOps maintains architectural living memory documents in `docs/context/`:
- [docs/context/ARCHITECTURE.md](docs/context/ARCHITECTURE.md)
- [docs/context/PRODUCT.md](docs/context/PRODUCT.md)
- [docs/context/DECISIONS.md](docs/context/DECISIONS.md)
- [docs/context/CURRENT_STATE.md](docs/context/CURRENT_STATE.md)
- [docs/context/SESSION_HANDOFF.md](docs/context/SESSION_HANDOFF.md)
- [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md)

If your PR alters service boundaries, contracts, security assumptions, or design decisions, **you must update the relevant documentation in the same PR**. Do not leave outdated documentation behind.

---

## 7. Release & Semantic Versioning

OpsPilot follows [Semantic Versioning (SemVer)](https://semver.org/):
```
MAJOR.MINOR.PATCH (e.g. v0.1.0, v0.2.0, v1.0.0)
```

- During pre-v1 development, we use `v0.x.x` releases:
  - `v0.1.0`: Initial core vertical slice (Control Plane, Agent, AI Service, Dashboard, verified Nginx on 8080).
  - `v0.2.0`: Reusable Runbook engine and execution templates.
  - `v0.3.0`: Multi-server fleet distribution.
  - `v1.0.0`: Production-ready, public stable release.
- Version tags are applied at release milestones, never per commit.

---

## 8. Development Commands Checklist

Before submitting a Pull Request, run the local verification suite:

```bash
# 1. Test Control Plane
cd apps/control-plane && go test -v -race ./...

# 2. Test Server Agent
cd agent && go test -v -race ./...

# 3. Test AI Service
cd apps/ai-service && pytest

# 4. Build Frontend Dashboard
cd apps/dashboard && npm run build
```
All commands must exit with code `0`.
