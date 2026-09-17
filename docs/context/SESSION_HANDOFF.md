# Session Handoff — ProvenOps

Last Updated: 2026-09-17

## Status
COMPLETE (PRODUCTION-READY FOR DEFINED SCOPE & RELEASE-HARDENED)

## Session Goal
Complete final production hardening for public release, rename OpsPilot to **ProvenOps**, close all production readiness gaps, synchronize documentation with ground-truth code, author a professional GitHub README, run fresh multi-node Docker reproduction tests, and execute a safe commit + push to the remote branch.

## What Was Done
1. **Repository Audit & Secret Scan**:
   - Audited git branch (`feat/ai-provenance`), tracked files, and remote `origin`.
   - Verified no credentials, private keys, or `.env` files are tracked (only `.env.example`).
   - Hardened `.gitignore`.

2. **Canonical Renaming (OpsPilot → ProvenOps)**:
   - Updated public naming, API metadata, and service descriptions to **ProvenOps**.
   - Preserved backward compatibility: environment variables evaluate `PROVENOPS_* > OPSPILOT_* > generic`.
   - Metric prefix updated to `provenops_*` with backward-compatible `opspilot_*` aliases.
   - Filesystem paths use `/var/lib/provenops` and `/etc/provenops/certs` with transparent backward-compatible symlinks to `/var/lib/opspilot` and `/etc/opspilot/certs`.
   - Docker container names updated to `provenops-*` with network aliases preserving existing service resolution.
   - Internal Go module path `opspilot/...` preserved to avoid unnecessary import churn.

3. **Docker & Multi-Node Lab Hardening**:
   - `agent/Dockerfile`: Builds `/bin/provenops-agent` and creates symlinks for `/usr/local/bin/opspilot-agent`, `/var/lib/provenops`, `/var/log/provenops`.
   - `apps/ai-service/Dockerfile`: Multi-stage hardening with non-root user `appuser`.
   - `apps/control-plane/Dockerfile`: Added cert path symlinks and minimal runtime.
   - `docker-compose.yml`: Fully updated with `name: provenops`, dual-stack environment variables, and network aliases.

4. **Makefile & Developer Workflows**:
   - Standard targets: `make build`, `make test`, `make test-unit`, `make test-race`, `make lab-up`, `make lab-down`, `make validate-production`, `make clean`.
   - `.github/workflows/ci.yml`: Added full Go race tests, Python tests, and Dashboard typecheck/build.

5. **Empirical Production Validation (15/15 Claims Verified)**:
   - Live execution on fresh 5-node cluster confirmed:
     1. `verification_contract` (VERIFIED)
     2. `canary_blast_radius_halting` (VERIFIED)
     3. `rolling_deployment_batch_enforcement` (VERIFIED)
     4. `saga_lifo_rollback_compensation` (VERIFIED)
     5. `dag_orchestration_and_cycle_rejection` (VERIFIED)
     6. `retry_engine_and_timeout_enforcement` (VERIFIED)
     7. `remediation_budget_enforcement` (VERIFIED)
     8. `control_plane_restart_active_execution` (VERIFIED)
     9. `agent_kill_ledger_reconciliation` (VERIFIED)
     10. `network_partition_resilience` (VERIFIED)
     11. `revoked_certificate_handshake_rejection` (VERIFIED)
     12. `command_guard_pipeline_blocking` (VERIFIED)
     13. `secret_redaction_in_logs_and_audit` (VERIFIED)
     14. `docker_network_isolation` (VERIFIED)
     15. `go_race_detector_concurrency` (VERIFIED)
   - Stored in `artifacts/validation-report.json` and `artifacts/validation-report.md`.

6. **GitHub README & Documentation**:
   - Rewrote `README.md` to high-grade infrastructure engineering standards with Mermaid architecture, core principles (`EXECUTED != VERIFIED`), capabilities, security model, and quick start.
   - Synchronized `docs/PRODUCTION_READINESS.md`, `docs/THREAT_MODEL.md`, `docs/FAILURE_MODEL.md`, `GEMINI.md`.
