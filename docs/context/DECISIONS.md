# Architecture Decision Records (ADRs)

## ADR-001 — Monorepo Architecture
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: OpsPilot comprises a Go Control Plane, Go Server Agent, Python AI Service, React Dashboard, Protobuf schemas, and a benchmark lab.
- **Decision**: Maintain all components in a single monorepo with root `go.work`.
- **Reason**: Atomic schema updates across protobufs, agent executors, and control plane orchestrators; simplified local development with Docker Compose.
- **Alternatives**: Polyrepo (rejected due to schema synchronization overhead).
- **Consequences**: Fast feedback loop, single CI/CD pipeline, and unified project context.

## ADR-002 — Go Standard Library, Chi, and pgx for Control Plane
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: The Control Plane orchestrates concurrent gRPC streams, REST endpoints, SSE connections, and transactional state machine mutations.
- **Decision**: Use Go 1.23+ with Chi router and `pgx/v5` connection pool.
- **Reason**: High throughput, minimal memory overhead, explicit transaction management, and no heavy ORM magic.
- **Alternatives**: GORM (rejected due to hidden queries and connection pool complexities).
- **Consequences**: Clear, auditable database queries and high resilience.

## ADR-003 — Outbound gRPC with mTLS for Server Agent
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Managing Linux hosts across varied network topologies and firewalls without requiring open inbound ports on target servers.
- **Decision**: The Server Agent initiates an outbound gRPC connection to the Control Plane.
- **Reason**: Traverses NAT and enterprise firewalls without firewall rules on managed hosts; multiplexes heartbeats, telemetry, task dispatch, and log streaming over a single TCP socket.
- **Alternatives**: Control plane SSHing into target hosts (rejected as unscaleable, hard to audit, and high security risk).
- **Consequences**: No open inbound ports on managed servers.

## ADR-004 — Python FastAPI with Strict Pydantic Contracts for AI Service
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: AI models (Ollama/Qwen) produce probabilistic outputs that need structured extraction and validation.
- **Decision**: Implement the AI Service in Python using FastAPI and Pydantic v2.
- **Reason**: Python provides the premier ecosystem for local LLM integration; Pydantic enforces schema validation before responses reach the Control Plane.
- **Alternatives**: Calling Ollama directly from Go (rejected due to prompt templating rigidity and lack of rich LLM validation libraries).
- **Consequences**: Strict JSON schemas guaranteed; malformed model outputs are rejected at the service boundary.

## ADR-005 — Typed Tools as Primary Primitive, Defense-in-Depth for Raw Fallback
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Unrestricted shell execution by an AI agent creates immense security risks.
- **Decision**: Typed tools (`systemd`, `package`, `network`, `file`, `docker`) are the primary primitives. Fallback `execute_command` is strictly guarded by AST token inspection and denylists.
- **Reason**: Deterministic execution, precise risk scoring, and zero-tolerance for destructive commands.
- **Alternatives**: Unrestricted bash shell (rejected as fundamentally unsafe).
- **Consequences**: Safe operations with auditable arguments and automatic config backups.

## ADR-006 — React, Vite & Tailwind CSS for Dashboard
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Operations dashboard needs real-time timeline streaming, approval gates, and fleet status cards without bloat.
- **Decision**: React 18, Vite, TypeScript, and Tailwind CSS with Server-Sent Events (SSE).
- **Reason**: Fast build times, zero-latency streaming updates, and technical dark-mode aesthetics.
- **Alternatives**: Server-side rendered HTML / HTMX (rejected due to complex multi-step timeline interactivity).
- **Consequences**: Modern, responsive user experience for operators.

## ADR-007 — Dual-Mode Database Store (PostgreSQL with In-Memory Resilient Fallback)
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Control Plane tests and local dev should run smoothly even if PostgreSQL is temporarily starting or offline.
- **Decision**: Define `database.Store` interface with both `pgxpool` and thread-safe in-memory implementations.
- **Reason**: Guarantees zero-crash tests and seamless developer onboarding.
- **Consequences**: Fast unit tests, resilient local development.

## ADR-008 — Untrusted Observation Boundary for Prompt Injection Defense
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Malicious payloads or adversarial strings inside server logs could hijack LLM instructions.
- **Decision**: Encapsulate all host logs, stdout, and stderr inside `<UNTRUSTED_OBSERVATION>` tags in AI prompts.
- **Reason**: Instructs the model that observation data is inert diagnostic evidence, never executable instructions.
- **Consequences**: Neutralizes indirect prompt injection attacks.

## ADR-009 — Structured Failure Taxonomy & Anti-Side-Effect Retry Policy
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Unstructured string error handling and blind automatic retries of mutating commands risk compounding infrastructure outages.
- **Decision**: Classify all operational failures into a strict 12-type `StructuredFailure` enum. Prohibit retries of side-effect operations (`install_package`, `write_config_file`, `restart_service`); restrict retries to read-only queries and transient network timeouts with exponential backoff.
- **Reason**: Guarantees deterministic failure classification and prevents dangerous duplicate execution.
- **Consequences**: High operational safety; all retry attempts recorded in audit logs.

## ADR-010 — Bounded Replanning with Strict Intent Integrity and Plan Versioning
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Agentic loops without boundaries can enter infinite execution cycles or silently mutate user intent (e.g. switching requested port 8080 to 8081).
- **Decision**: Impose `MAX_REPLANS = 3`. Require `plan_version` increments on every replan cycle. Mandate that replans introducing modifying or elevated risk actions invalidate prior operator approvals and transition to `WAITING_APPROVAL`. Enforce prompt and schema constraints forbidding silent intent drift.
- **Reason**: Bounded execution avoids infinite loops; operator approval gates protect intent boundaries.
- **Consequences**: Deterministic replanning with strict human-in-the-loop oversight.

## ADR-011 — Transactional Configuration Writes and Verified Rollback Compensation
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Applying an invalid server configuration can bring down active production services.
- **Decision**: Implement pre-flight backup to `/var/lib/opspilot/backups/{task_id}/`, run in-tool atomic validation (`nginx -t`), and auto-revert on syntax failure. For execution-level rollback, transition task state machine to `ROLLING_BACK`, restore configuration, restart affected services, independently verify service health, and transition to `ROLLED_BACK`.
- **Reason**: Restores verified system health even when planned actions fail.
- **Consequences**: Zero downtime on syntax error; verified state restoration.

## ADR-012 — Multi-Tier Idempotency (API Header, Step Execution Cache, Tool Pre-Checks)
- **Status**: Active
- **Date**: 2026-09-12
- **Context**: Network retries at the client or agent communication level can lead to duplicate task creation or redundant side effects.
- **Decision**: Support `Idempotency-Key` HTTP header with PostgreSQL unique constraint; generate `execution_id` (`task_id/step_id/attempt`) for agent step execution cache (`sync.Map`); implement pre-checks in tools (`dpkg -s`, SHA256 content comparison, `systemctl is-active`) returning `ALREADY_SATISFIED`.
- **Reason**: Prevents redundant infrastructure mutations and guarantees idempotent API behavior.
- **Consequences**: Eliminates side-effect duplication and logs `IDEMPOTENT_NO_OP` events.

