# OpsPilot End-to-End (E2E) Live Verification Guide

This guide describes how to run and verify the complete OpsPilot vertical slice against a real Ubuntu host (Ubuntu 22.04 or 24.04 with systemd).

---

## 1. Prerequisites

1. **Control Plane Host** (Windows / macOS / Linux):
   - Docker & Docker Compose (for PostgreSQL 16 and Ollama)
   - Go 1.23+
   - Python 3.11+
2. **Managed Target Host** (Real Ubuntu VM / WSL2 with systemd):
   - Ubuntu 22.04 LTS or 24.04 LTS
   - Active systemd init (`systemctl is-system-running` returns `running` or `degraded`)
   - `sudo` privileges for the agent runner
   - Network connectivity to the Control Plane gRPC port (default `:9090`)

---

## 2. Step-by-Step E2E Setup

### Step 2.1. Start Central Infrastructure

1. **Start PostgreSQL 16**:
   ```bash
   docker compose up -d postgres
   ```
   Postgres automatically initializes the schema from `db/migrations/001_init.sql`.

2. **Start Ollama & Pull Qwen2.5**:
   ```bash
   docker run -d --name opspilot-ollama -p 11434:11434 -v opspilot_ollama:/root/.ollama ollama/ollama
   docker exec opspilot-ollama ollama pull qwen2.5:3b
   ```

3. **Generate PKI / Mutual TLS (mTLS) Certificates**:
   ```bash
   cd apps/control-plane
   go run cmd/pki-gen/main.go -out ../../infra/certs -host localhost -agent-id agent-ubuntu-01
   ```
   This generates:
   - `infra/certs/ca.crt` & `ca.key` (Root Certificate Authority)
   - `infra/certs/server.crt` & `server.key` (Control Plane Server Certificate)
   - `infra/certs/agent.crt` & `agent.key` (Agent Client Certificate)

4. **Start Control Plane**:
   ```bash
   cd apps/control-plane
   # Run with PostgreSQL persistence and mTLS enabled:
   export DATABASE_URL="postgres://opspilot:opspilot_dev_secret@localhost:5432/opspilot?sslmode=disable"
   export TLS_ENABLED="true"
   export TLS_CA_CERT="../../infra/certs/ca.crt"
   export TLS_SERVER_CERT="../../infra/certs/server.crt"
   export TLS_SERVER_KEY="../../infra/certs/server.key"
   go run cmd/server/main.go
   ```

5. **Start Python AI Planning Service**:
   ```bash
   cd apps/ai-service
   export OLLAMA_BASE_URL="http://localhost:11434"
   export DEFAULT_LLM_MODEL="qwen2.5:3b"
   uvicorn app.main:app --host 0.0.0.0 --port 8000
   ```

---

### Step 2.2. Build & Deploy Agent to Ubuntu Host

1. **Cross-Compile Agent Binary for Linux**:
   ```bash
   cd agent
   GOOS=linux GOARCH=amd64 go build -o ../bin/opspilot-agent cmd/agent/main.go
   ```

2. **Copy Agent Binary & Certificates to Target Ubuntu Host**:
   ```bash
   sudo mkdir -p /etc/opspilot/certs /var/log/opspilot
   sudo cp bin/opspilot-agent /usr/local/bin/opspilot-agent
   sudo cp infra/certs/ca.crt /etc/opspilot/certs/ca.crt
   sudo cp infra/certs/agent.crt /etc/opspilot/certs/agent.crt
   sudo cp infra/certs/agent.key /etc/opspilot/certs/agent.key
   sudo chmod 600 /etc/opspilot/certs/agent.key
   ```

3. **Start the Agent Daemon on Ubuntu**:
   ```bash
   sudo AGENT_CONTROL_PLANE_ADDR="127.0.0.1:9090" \
        AGENT_BOOTSTRAP_TOKEN="opspilot-default-bootstrap-token-2026" \
        AGENT_HOSTNAME="ubuntu-node-01" \
        TLS_ENABLED="true" \
        TLS_CA_CERT="/etc/opspilot/certs/ca.crt" \
        TLS_CLIENT_CERT="/etc/opspilot/certs/agent.crt" \
        TLS_CLIENT_KEY="/etc/opspilot/certs/agent.key" \
        TLS_SERVER_NAME="localhost" \
        /usr/local/bin/opspilot-agent
   ```

---

## 3. Vertical Slice Execution

### Target User Intent:
> *"Install nginx on this server and expose it on port 8080."*

### Execution Flow:

1. **Create Task via REST API**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/tasks \
     -H "Content-Type: application/json" \
     -d '{
       "title": "Install and expose Nginx on 8080",
       "prompt": "Install nginx on this server and expose it on port 8080.",
       "target_agent_ids": ["agent-ubuntu-node-01"]
     }'
   ```

2. **Verify Automatic Transition Sequence**:
   - `CREATED` → `DISCOVERING` (gRPC host collector inspects OS version & capabilities)
   - `PLANNING` (AI Service calls Ollama / Qwen2.5 to generate structured JSON plan)
   - `POLICY_EVALUATION` (Policy engine detects `install_package` / `restart_service` as MEDIUM risk)
   - `WAITING_APPROVAL` (System halts execution and awaits operator approval)

3. **Approve Plan**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/tasks/<TASK_ID>/approve \
     -H "Content-Type: application/json" \
     -d '{"plan_version": 1, "notes": "Approved for testing"}'
   ```

4. **Verify Execution**:
   - `EXECUTING` (Control Plane dispatches commands over gRPC mTLS stream)
   - Real apt package installation: `apt-get install -y nginx`
   - Real configuration update: `/etc/nginx/sites-available/default` with `listen 8080;`
   - Real systemd service restart: `systemctl restart nginx`

5. **Deterministic Verification & Audit**:
   - `VERIFYING` suite executes:
     - `systemctl is-active nginx` → confirms `active (running)`
     - TCP socket probe to `:8080` → confirms port open
     - HTTP probe to `http://<host>:8080` → confirms `HTTP 200 OK`
   - `COMPLETED` state reached
   - Audit trail persisted into PostgreSQL `audit_events` and `verification_results` tables

---

## 4. Verification Check Commands on Ubuntu

```bash
# 1. Check systemd service status
systemctl is-active nginx
# Expected: active

# 2. Check open port
ss -tlpn | grep 8080
# Expected: LISTEN on 0.0.0.0:8080 or :::8080

# 3. Check HTTP endpoint
curl -I http://localhost:8080
# Expected: HTTP/1.1 200 OK
```
