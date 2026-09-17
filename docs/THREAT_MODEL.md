# ProvenOps Threat Model & Trust Boundaries

## 1. Executive Security Philosophy
ProvenOps enforces a strict **Zero-Blind-Trust** security architecture. Large Language Models (LLMs) are treated as **untrusted, probabilistic planners**. They never possess direct shell execution privileges, network access to production nodes, or credential authority.

---

## 2. Trust Boundaries & Component Architecture

```
┌──────────────────────────────────────────────────────────┐
│                   Untrusted Operator                     │
│                  Web Browser / Dashboard                │
└────────────────────────────┬─────────────────────────────┘
                             │ TLS 1.3 + JWT (RBAC)
┌────────────────────────────▼─────────────────────────────┐
│             TRUST BOUNDARY: Control Plane                │
│                 (Sole Source of Authority)               │
│  • RBAC & Audit Engine      • Central Policy Engine      │
│  • Internal X.509 Root CA   • Deterministic Verifier     │
└────────┬───────────────────┼─────────────────────┬───────┘
         │ mTLS (gRPC)       │ Strict JSON Schema  │ SQL (pgx)
         │                   │ (No Shell)          │
┌────────▼──────────┐ ┌──────▼──────────────┐ ┌────▼───────┐
│   Server Agent    │ │     AI Service      │ │ PostgreSQL │
│ (Restricted Exec) │ │ (Untrusted Planner) │ │  (Isolated)│
│ • AST Guard       │ │ • Grounded Schema   │ └────────────┘
│ • Atomic Files    │ │ • Injection Barrier │
│ • bbolt Journal   │ └─────────────────────┘
└───────────────────┘
```

---

## 3. Threat Vectors & Defense-in-Depth Mitigations

### 3.1 Indirect Prompt Injection via Server Output
* **Threat:** Malicious payload embedded inside log files, `/etc/` config, or process names attempting to hijack the AI prompt.
* **Mitigation:**
  1. All server outputs are sanitized and encapsulated within strict `<UNTRUSTED_OBSERVATION>` delimiters.
  2. Model output is strictly forced into Pydantic JSON schemas. Text outside JSON is discarded.
  3. Control Plane Policy Engine evaluates all actions independently against deterministic rules. Model self-reported risk scores are ignored.

### 3.2 Destructive Shell Execution Escapes
* **Threat:** Attempting to execute `rm -rf /`, `mkfs`, `fdisk`, fork bombs, or piping remote scripts into shells (`curl | sh`).
* **Mitigation:**
  1. Default execution uses **Typed Tools** (`ensure_package`, `ensure_service`, `ensure_file`).
  2. Raw shell (`execute_command`) passes through AST tokenization and regular expression guards blocking subshells, dynamic `eval`, `find -exec`, `xargs`, dangerous device redirections (`> /dev/sd*`), and path traversal.

### 3.3 Path Traversal and Symlink Escalation
* **Threat:** Malicious relative paths (`../../etc/shadow`) or symlinks targeting protected system files.
* **Mitigation:**
  1. `ValidateFilesystemPath` resolves canonical paths and evaluates symlink targets with `filepath.EvalSymlinks`.
  2. Protected system roots (`/proc`, `/sys`, `/dev`, `/boot`, `/etc/shadow`, `/etc/sudoers`) are strictly blocked.
  3. Atomic file swap (`.provenops_tmp_*` -> `fsync` -> `rename`) prevents TOCTOU race conditions.

### 3.4 Compromised Agent or Rogue Client
* **Threat:** Rogue machine attempting to impersonate an agent or spoof commands.
* **Mitigation:**
  2. Certificate Identity Binding: Common Name (`CN=agent:<id>`) is validated against registered agent ID.
  3. Certificate Revocation List (CRL): Revoked certificate serial numbers are immediately rejected at connection time.
  4. Single-use, time-bound bootstrap tokens with replay protection.

### 3.5 Credential & Secret Leakage
* **Threat:** Passwords, private keys, or API tokens appearing in audit logs, telemetry, SSE streams, or AI prompts.
* **Mitigation:**
  1. Dedicated `security.RedactString` and `security.RedactMap` engine masks private keys (`-----BEGIN PRIVATE KEY-----`), JWTs (`Bearer eyJ...`), URI credentials (`postgres://user:[REDACTED]@host`), and AWS/GitHub tokens across all logging and streaming pipelines.
