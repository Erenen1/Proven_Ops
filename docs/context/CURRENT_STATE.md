# Current Project State

Last Updated: 2026-09-12

## Current Focus
Validating and hardening OpsPilot against a real Ubuntu host environment. Full vertical slice validation completed and verified end-to-end.

## Completed
- **Live Ubuntu Vertical Slice Verified**: End-to-end flow passed on Ubuntu 24.04 with live Ollama `qwen2.5:3b`, PostgreSQL 16, and real Server Agent daemon:
  - Task creation (`POST /api/v1/tasks`)
  - Agent host discovery (`DISCOVERING`)
  - AI Plan generation via Ollama (`PLANNING`)
  - Policy & Risk evaluation -> `WAITING_APPROVAL`
  - Operator approval (`POST /api/v1/tasks/{id}/approve`)
  - Real execution (`EXECUTING`): `apt-get install -y nginx`, configure `/etc/nginx/sites-available/default` for port 8080, `systemctl restart nginx`
  - Deterministic verification (`VERIFYING`): `package_installed` (dpkg), `systemd_active` (systemd), `tcp_port_open` (socket probe on 8080), `http_probe` (HTTP 200)
  - `COMPLETED` state and PostgreSQL `audit_events` persistence.
- **Production PostgreSQL Store**: Implemented `PostgresStore` (`apps/control-plane/internal/database/postgres.go`) using `pgxpool.Pool` with fail-fast in production mode.
- **True Mutual TLS (mTLS)**: Implemented ECDSA P-256 PKI, client/server certificate generation, strict `RequireAndVerifyClientCert` enforcement, and rejection test (`apps/control-plane/internal/pki/mtls_test.go`).
- **Deterministic Verification Engine**: Implemented `executeVerification` in orchestrator with actual agent tools and network probes.
- **Agent Real Metrics**: Fixed dummy metrics in `collector.go` to parse `/proc/loadavg` and `/proc/meminfo`.
- **Documentation & Reality Hardening**: `VALIDATION_REPORT.md` and `E2E_TEST.md` created; downgraded prompt injection claims to defense-in-depth boundary; clarified offline nature of benchmark suite.

## In Progress
- Completing session handoff and reporting findings.

## Next
1. Add automated VM integration tests to GitHub Actions pipeline.
2. Build UI test harness for Dashboard verification against live backend.

## Blocked
- None. All components build and pass unit tests.

## Important Paths
- Entry Point & Memory: `GEMINI.md`, `docs/context/`
- Workspace Rules: `.agents/rules/`
- Control Plane Entry: `apps/control-plane/cmd/server/main.go`
- Server Agent Entry: `agent/cmd/agent/main.go`
- AI Service Entry: `apps/ai-service/app/main.py`
- Dashboard Entry: `apps/dashboard/src/App.tsx`
- Database Schema: `db/migrations/001_init.sql`
- Agent Installer: `scripts/install-agent.sh`
