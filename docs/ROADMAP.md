# OpsPilot Development Roadmap

## Phase 0: Project Foundations & Architecture [COMPLETED]
- [x] Monorepo directory structure definition
- [x] Context & Living Memory (`PROJECT_CONTEXT.md`)
- [x] Architecture specifications (`docs/ARCHITECTURE.md`)
- [x] Threat model and security policies (`docs/SECURITY.md`)
- [x] Architectural Decision Records (`docs/DECISIONS.md`)
- [x] Roadmap documentation (`docs/ROADMAP.md`)
- [x] PostgreSQL database migration scripts (`db/migrations/001_init.sql`)
- [x] Docker Compose environment (`docker-compose.yml`, `infra/dev/Dockerfile.ubuntu-node`)

## Phase 1: Go Control Plane Core [COMPLETED]
- [x] Go module initialization (`apps/control-plane`)
- [x] Database connection pool and repositories (`pgx/v5` & in-memory resilient store)
- [x] Authentication & RBAC middleware (JWT, Admin/Operator/Viewer roles)
- [x] Agent registry & heartbeat manager
- [x] Task State Machine with transition history and validation
- [x] Server-Sent Events (SSE) live event hub

## Phase 2: Protobuf & Go Server Agent [COMPLETED]
- [x] Protobuf definitions (`proto/agent.proto`)
- [x] Agent binary structure (`agent/cmd/agent`)
- [x] Host capability and hardware discovery engine
- [x] Typed tools implementation (`systemd`, `apt`, `net`, `file`, `docker`)
- [x] Safe `execute_command` fallback engine with AST token filters
- [x] Outbound gRPC connection with reconnection logic and heartbeat

## Phase 3: AI Service (Python FastAPI & Ollama) [COMPLETED]
- [x] FastAPI application setup (`apps/ai-service`)
- [x] Pydantic request/response schemas
- [x] LLM provider abstraction (Ollama default, Qwen models)
- [x] Prompt engineering with boundary isolation for untrusted host outputs
- [x] Structured JSON output generator and schema validator
- [x] Dynamic replanning and failure diagnosis endpoints

## Phase 4: Policy Engine, Risk Evaluation & Approvals [COMPLETED]
- [x] Static tool risk registry (READ_ONLY, LOW, MEDIUM, HIGH, FORBIDDEN)
- [x] Environment and server-scoped policy engine
- [x] Plan approval workflow (versioned approvals, rejection handling)
- [x] Orchestration pipeline connecting State Machine, AI Plan, and Agent Dispatch

## Phase 5: Verification Engine & Audit Trail [COMPLETED]
- [x] Multi-tier verification strategies:
  - Systemd status verification (`systemctl is-active`)
  - TCP port binding check
  - HTTP probe (status code & content check)
  - Package query check (`dpkg -s`)
  - File integrity check (checksum comparison)
- [x] Append-only audit logger with credential redaction

## Phase 6: React Dashboard [COMPLETED]
- [x] Vite + React + TypeScript + Tailwind project (`apps/dashboard`)
- [x] Fleet overview & host status cards
- [x] Live Task Timeline with SSE real-time updates
- [x] Interactive Approval Gate with risk visualizers
- [x] Runbook execution and creation view
- [x] Audit log explorer with filtering

## Phase 7: Runbook System [COMPLETED]
- [x] Convert successful task plan into reusable runbook template
- [x] Deterministic runbook execution (bypassing AI planner)

## Phase 8: Multi-Server Fleet Orchestration [COMPLETED]
- [x] Parallel task dispatch across multiple target agents
- [x] Individual node status tracking (SUCCESS, FAILED, TIMEOUT, OFFLINE)
- [x] Aggregate task state calculation (COMPLETED, PARTIAL_SUCCESS, FAILED)

## Phase 9: Benchmark Lab & Target Scenarios [COMPLETED]
- [x] Controlled failure environments (`benchmarks/`):
  - `bad-nginx-config`
  - `port-conflict`
  - `disk-full`
  - `systemd-failure`
  - `docker-crash`
- [x] Evaluation harness and metrics reporting

## Phase 10: Testing, CI/CD & Production Polish [COMPLETED]
- [x] Ubuntu agent installer script (`scripts/install-agent.sh`)
- [x] Automated unit and integration tests
- [x] GitHub Actions CI workflow (`.github/workflows/ci.yml`)
- [x] Comprehensive production-grade `README.md`
