# Current Project State — ProvenOps

Last Updated: 2026-09-17

## Current Focus
Public release hardening and canonical renaming to **ProvenOps**. All 15 production readiness areas empirically tested and verified against the live 5-node Docker lab (`provenops-node-01` to `provenops-node-05`), PostgreSQL 16, Control Plane, AI Service, and Dashboard.

## Completed
- **Public Product Rename & Hardening (OpsPilot → ProvenOps)**:
  - Canonical product name: **ProvenOps**.
  - Metric namespace: `provenops_*` with backward-compatible `opspilot_*` aliases.
  - Environment configuration: `PROVENOPS_*` primary with fallback to `OPSPILOT_*`.
  - Docker Compose services and container names updated to `provenops-*` (`provenops-postgres`, `provenops-control-plane`, `provenops-ai-service`, `provenops-dashboard`, `provenops-node-01`..`node-05`).
  - Internal Root CA Common Name updated to `ProvenOps Internal Root CA`.
  - Binary outputs updated to `provenops-agent` (with `/usr/local/bin/opspilot-agent` symlink).
  - Storage paths updated to `/var/lib/provenops` (with `/var/lib/opspilot` symlink) and `/etc/provenops/certs` (with `/etc/opspilot/certs` symlink).
  - Dashboard title, brand headers, and package name updated to `provenops-dashboard`.
  - Comprehensive GitHub `README.md` authored.

- **Empirical Production Validation Suite (`scripts/validate_production.py`)**:
  - Live multi-node verification executed on 5 real Linux agent nodes with 100% empirical evidence (15/15 tests VERIFIED, 0 failed):
    1. Independent Verification Contract (`EXECUTED != VERIFIED`)
    2. Canary Rollout Blast Radius Halting (node-02..node-05 100% untouched)
    3. Rolling Deployment Sequential Batch Enforcement
    4. Saga LIFO Compensation on Real Node (SHA256 hash verified)
    5. DAG Orchestration & Cycle Rejection
    6. Retry Engine & Process Timeout Enforcement (<3s)
    7. Remediation Budget Exhaustion Loop Halting
    8. Control Plane Restart Active Execution Recovery
    9. Agent Kill & bbolt Ledger Reconciliation (no duplicate execution)
    10. Network Partition Disconnect & Auto-Reconnection
    11. Revoked Certificate Rejection via Live mTLS Handshake (bad certificate alert)
    12. CommandGuard Pipeline Blocking (AST regex & binary guard)
    13. Secret Redaction in Actual Database & Audit Records
    14. Docker Network Isolation (agents blocked from Postgres & AI Service)
    15. Concurrency Race Detector (`go test -race` clean across all core packages)
  - Generated reports: `artifacts/validation-report.json` and `artifacts/validation-report.md`.
  - Final Verdict: `PRODUCTION READY FOR DEFINED SCOPE`.

- **Developer Tooling & CI Workflows**:
  - `Makefile` standard targets (`build`, `test`, `test-unit`, `test-race`, `lab-up`, `lab-down`, `validate-production`, `clean`).
  - GitHub Actions CI workflow in `.github/workflows/ci.yml`.

## Important Paths
- Entry Point & Memory: `GEMINI.md`, `docs/context/`
- Architecture & State Machine: `docs/context/ARCHITECTURE.md`
- Threat Model: `docs/THREAT_MODEL.md`
- Failure Model: `docs/FAILURE_MODEL.md`
- Production Readiness: `docs/PRODUCTION_READINESS.md`
- Control Plane Entry: `apps/control-plane/cmd/server/main.go`
- Server Agent Entry: `agent/cmd/agent/main.go`
- AI Service Entry: `apps/ai-service/app/main.py`
- Database Migrations: `db/migrations/001_init.sql` through `006_production_hardening.sql`
- Validation Suite: `scripts/validate_production.py`
