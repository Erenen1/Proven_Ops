#!/usr/bin/env bash
set -euo pipefail

# ProvenOps Server Agent Installation Script for Ubuntu 22.04 / 24.04
# Usage: curl -sSL http://control-plane:8080/scripts/install-agent.sh | sudo bash -s -- --token <BOOTSTRAP_TOKEN> --server <CONTROL_PLANE_ADDR>

CONTROL_PLANE_ADDR="${CONTROL_PLANE_ADDR:-localhost:9090}"
BOOTSTRAP_TOKEN="${BOOTSTRAP_TOKEN:-provenops-default-bootstrap-token-2026}"
ENVIRONMENT="${ENVIRONMENT:-production}"
INSTALL_DIR="/opt/provenops"
CONFIG_DIR="/etc/provenops"
BIN_NAME="provenops-agent"

echo "=== [ProvenOps] Installing Server Agent ==="

# Check root privileges
if [ "$EUID" -ne 0 ]; then
  echo "Error: This script must be run as root (or with sudo)." >&2
  exit 1
fi

# Detect OS
if [ -f /etc/os-release ]; then
  . /etc/os-release
  if [ "$ID" != "ubuntu" ]; then
    echo "Warning: Detected $ID $VERSION_ID. ProvenOps officially supports Ubuntu 22.04/24.04."
  fi
fi

# Create dedicated non-root provenops service account
if ! id -u provenops >/dev/null 2>&1; then
  echo "Creating system user 'provenops'..."
  useradd --system --no-create-home --shell /bin/false provenops
fi

# Setup scoped least-privilege sudo policy
echo "Configuring scoped sudoers policy in /etc/sudoers.d/provenops..."
cat << 'EOF' > /etc/sudoers.d/provenops
# ProvenOps least-privilege administrative actions
provenops ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /usr/bin/apt-get, /usr/bin/apt, /usr/bin/dpkg
EOF
chmod 0440 /etc/sudoers.d/provenops

# Create directories and backward-compatible symlinks
mkdir -p "${INSTALL_DIR}"
mkdir -p "${CONFIG_DIR}"
mkdir -p /var/lib/provenops/backups /var/lib/provenops/certs /var/log/provenops
ln -sf "${INSTALL_DIR}" /opt/opspilot 2>/dev/null || true
ln -sf "${CONFIG_DIR}" /etc/opspilot 2>/dev/null || true
ln -sf /var/lib/provenops /var/lib/opspilot 2>/dev/null || true

# Write environment configuration
cat << EOF > "${CONFIG_DIR}/agent.env"
PROVENOPS_CONTROL_PLANE_ADDR=${CONTROL_PLANE_ADDR}
PROVENOPS_BOOTSTRAP_TOKEN=${BOOTSTRAP_TOKEN}
PROVENOPS_ENVIRONMENT=${ENVIRONMENT}
PROVENOPS_HEARTBEAT_INTERVAL_SEC=5
PROVENOPS_WORKING_DIR=/tmp/provenops
# Backward compatibility
AGENT_CONTROL_PLANE_ADDR=${CONTROL_PLANE_ADDR}
AGENT_BOOTSTRAP_TOKEN=${BOOTSTRAP_TOKEN}
EOF
chmod 0600 "${CONFIG_DIR}/agent.env"
chown provenops:provenops "${CONFIG_DIR}/agent.env"

# Copy or download binary
if [ -f "./bin/${BIN_NAME}" ]; then
  cp "./bin/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
elif [ -f "./${BIN_NAME}" ]; then
  cp "./${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
elif [ -f "./bin/opspilot-agent" ]; then
  cp "./bin/opspilot-agent" "${INSTALL_DIR}/${BIN_NAME}"
fi

chmod 0755 "${INSTALL_DIR}/${BIN_NAME}"
ln -sf "${INSTALL_DIR}/${BIN_NAME}" /usr/local/bin/provenops-agent 2>/dev/null || true
ln -sf "${INSTALL_DIR}/${BIN_NAME}" /usr/local/bin/opspilot-agent 2>/dev/null || true
chown -R provenops:provenops "${INSTALL_DIR}" /var/lib/provenops

# Create Systemd Unit
echo "Creating systemd service: /etc/systemd/system/provenops-agent.service..."
cat << EOF > /etc/systemd/system/provenops-agent.service
[Unit]
Description=ProvenOps Infrastructure Server Agent
After=network.target network-online.target systemd-journald.service
Wants=network-online.target

[Service]
Type=simple
User=provenops
Group=provenops
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${CONFIG_DIR}/agent.env
ExecStart=${INSTALL_DIR}/${BIN_NAME}
Restart=always
RestartSec=5s
LimitNOFILE=65536
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable provenops-agent.service

echo "=== [ProvenOps] Agent installed successfully! ==="
echo "Start agent with: sudo systemctl start provenops-agent"
echo "Check status with: sudo systemctl status provenops-agent"
