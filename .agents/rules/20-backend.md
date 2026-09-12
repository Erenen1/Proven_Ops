---
description: Backend engineering standards for Go Control Plane, Go Server Agent, and Python AI Service
globs: ["apps/control-plane/**", "agent/**", "apps/ai-service/**", "proto/**"]
---

# Backend Engineering Conventions

## 1. Go Control Plane (`apps/control-plane/`)
- **Routing & HTTP**: Use lightweight `chi/v5` router. Handle graceful shutdown with `context.Context` and OS signal notifications.
- **Database**: Use `pgx/v5` connection pools. Avoid heavy ORMs; keep queries explicit, typed, and transaction-safe for state machine mutations.
- **State Machine Authority**: State transitions MUST go through `statemachine.Machine.Transition()`. Never modify `task.Status` directly in API handlers.
- **Real-Time Events**: Broadcast lifecycle changes through `events.Hub`. All events must serialize to valid JSON.

## 2. Go Server Agent (`agent/`)
- **Single Binary Footprint**: Keep dependencies lean (<30MB memory RSS). Ensure non-root compatibility with scoped `sudo` permissions.
- **gRPC Outbound Client**: The agent initiates outbound connections to the Control Plane. Never listen on inbound ports on managed hosts. Implement automatic reconnection with backoff.
- **Typed Tools Priority**: All operations must first attempt typed tools (`systemd`, `package`, `network`, `file`, `docker`).
- **Command Guard Defense**: Any invocation of `execute_command` must pass through `executor.CommandGuard` to block destructive operations (`rm -rf /`, `mkfs`, `fdisk`, `dd`, `shutdown`, `reboot`, fork bombs).
- **Atomic File Backups**: Before modifying configuration files, automatically write a `.bak.<timestamp>` backup.

## 3. Python AI Service (`apps/ai-service/`)
- **Strict Typing**: All request and response bodies must be validated via Pydantic v2 schemas. Never return unparsed freeform markdown text to the Control Plane.
- **Untrusted Observation Boundary**: All host outputs (stdout, stderr, logs) MUST be wrapped in `<UNTRUSTED_OBSERVATION>` tags in prompts to mitigate indirect prompt injection.
- **Model Provider Abstraction**: Implement providers inheriting from `LLMProvider`. Default to Ollama with deterministic heuristic fallbacks for offline or benchmark execution.

## 4. Protobuf Contracts (`proto/`)
- Schema changes in `proto/agent.proto` require recompilation to Go via `scripts/build-proto.ps1`.
- Maintain backwards compatibility in protobuf field numbers.
