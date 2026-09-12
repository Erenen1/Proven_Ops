# Architecture Decision Records (ADRs)

## ADR-001: Monorepo Architecture
- **Status**: Accepted
- **Context**: OpsPilot consists of a Go Control Plane, Go Server Agent, Python AI Service, React Dashboard, Protobuf definitions, and benchmark lab scenarios.
- **Decision**: Organize the project as a clean monorepo.
- **Consequences**: Easy atomic updates across protobuf schemas, agent implementations, and control plane handlers. Single source repository simplifies local dev stack with Docker Compose.

## ADR-002: Go Standard Library & pgx for Control Plane
- **Status**: Accepted
- **Context**: The Control Plane manages concurrent gRPC streams, REST endpoints, SSE connections, and transactional state machine mutations.
- **Decision**: Use Go with `pgx/v5` for PostgreSQL connectivity and lightweight Chi HTTP routing. Avoid heavyweight ORMs (like GORM) to ensure explicit query control, transaction safety, and minimal memory overhead.

## ADR-003: Outbound gRPC with mTLS for Server Agent
- **Status**: Accepted
- **Context**: Managing agents across diverse network topologies, NATs, and firewalls is difficult if the Control Plane must connect inbound to the agent.
- **Decision**: The Server Agent initiates an outbound gRPC connection to the Control Plane. Mutual TLS (mTLS) authenticates both ends.
- **Consequences**: No firewall openings required on the managed Linux nodes. Full multiplexing of heartbeats, task dispatch, and log streaming over a single TCP connection.

## ADR-004: Python FastAPI with Strict Pydantic Contracts for AI Service
- **Status**: Accepted
- **Context**: AI models (Ollama/Qwen) need prompt management, formatting, and structured output extraction. Python provides the richest ecosystem for local LLM orchestration and Pydantic validation.
- **Decision**: Implement the AI Service as a Python FastAPI microservice. Enforce JSON schema validation on every LLM completion. Reject malformed responses before they reach the Control Plane.

## ADR-005: Typed Tools as Primary Primitive, Defense-in-Depth for Raw Fallback
- **Status**: Accepted
- **Context**: Direct execution of unparsed bash strings creates immense security and verification challenges.
- **Decision**: Implement explicit typed actions (`systemd`, `apt`, `file`, `net`, `docker`) as primary primitives with deterministic verification strategies. Allow `execute_command` solely as a guarded fallback with token inspection and execution constraints.

## ADR-006: React, Vite & Tailwind CSS for Dashboard
- **Status**: Accepted
- **Context**: Modern operations dashboards need responsive status updates, interactive approval gates, and clear task timelines without excessive UI bloat.
- **Decision**: Use Vite + React + TypeScript + Tailwind CSS. Real-time updates delivered via Server-Sent Events (SSE).
