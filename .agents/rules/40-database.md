---
description: Database and schema conventions for PostgreSQL
globs: ["db/**", "apps/control-plane/internal/database/**"]
---

# Database & Schema Conventions

## 1. Engine & Connectivity
- **Engine**: PostgreSQL 16.
- **Go Driver**: `pgx/v5` connection pool (`*pgxpool.Pool`).
- **Resilient Fallback**: The Control Plane implements `database.Store` with both PostgreSQL and an in-memory repository to guarantee zero-crash execution during local development and testing.

## 2. Schema Conventions
- **Identifiers**: Use UUIDs for public entities (`tasks`, `approvals`, `runbooks`, `users`) via `uuid_generate_v4()`. Agent IDs use deterministic prefixes (`agent-<hostname>`).
- **Append-Only Immutable Logs**: Tables such as `audit_events`, `task_transitions`, `task_events`, and `agent_heartbeats` are strictly append-only. Never run `UPDATE` or `DELETE` on these tables.
- **Foreign Keys**: Enforce referential integrity with appropriate cascading rules (`ON DELETE CASCADE` for ephemeral children, `ON DELETE SET NULL` for user associations).

## 3. Migration Policy
- Migrations reside in `db/migrations/` using numeric prefixes (e.g. `001_init.sql`, `002_*.sql`).
- All migrations must be idempotent where possible (`CREATE TABLE IF NOT EXISTS`, `ON CONFLICT DO NOTHING`).
- Destructive schema changes (`DROP TABLE`, `DROP COLUMN`) are strictly forbidden without backwards-compatible data migration plans.
