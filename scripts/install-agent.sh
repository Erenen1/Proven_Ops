#!/usr/bin/env bash
set -euo pipefail

# OpsPilot Server Agent Installation Script for Ubuntu 22.04 / 24.04
# Usage: curl -sSL http://control-plane:8080/install.sh | sudo bash -s -- --token <BOOTSTRAP_TOKEN> --server <CONTROL_PLANE_ADDR>

CONTROL_PLANE_ADDR="${CONTROL_PLANE_ADDR:-localhost:9090}"
BOOTSTRAP_TOKEN="${BOOTSTRAP_TOKEN:-opspilot-default-bootstrap-token-2026}"
ENVIRONMENT="${ENVIRONMENT:-production}"
INSTALL_DIR="/opt/opspilot"
CONFIG_DIR="/etc/opspilot"
BIN_NAME="opspilot-agent"

echo "=== [OpsPilot] Installing Server Agent ==="

# Check root privileges
if [ "$EUID" -ne 0 ]; then
  echo "Error: This script must be run as root (or with sudo)." >&2
  exit 1
fi

# Detect OS
if [ -f /etc/os-release ]; then
  . /etc/os-release
  if [ "$ID" != "ubuntu" ]; then
    echo "Warning: Detected $ID $VERSION_ID. OpsPilot officially supports Ubuntu 22.04/24.04."
  fi
fi

# Create dedicated non-root opspilot service account
if ! id -u opspilot >/dev/null 2>&1; then
  echo "Creating system user 'opspilot'..."
  useradd --system --no-create-home --shell /bin/false opspilot
fi

# Setup scoped least-privilege sudo policy
echo "Configuring scoped sudoers policy in /etc/sudoers.d/opspilot..."
cat << 'EOF' > /etc/sudoers.d/opspilot
# OpsPilot least-privilege administrative actions
opspilot ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /usr/bin/apt-get, /usr/bin/apt, /usr/bin/dpkg
EOF
chmod 0440 /etc/sudoers.d/opspilot

# Create directories
mkdir -p "${INSTALL_DIR}"
mkdir -p "${CONFIG_DIR}"

# Write environment configuration
cat << EOF > "${CONFIG_DIR}/agent.env"
AGENT_CONTROL_PLANE_ADDR=${CONTROL_PLANE_ADDR}
AGENT_BOOTSTRAP_TOKEN=${BOOTSTRAP_TOKEN}
AGENT_ENVIRONMENT=${ENVIRONMENT}
AGENT_HEARTBEAT_INTERVAL_SEC=5
AGENT_WORKING_DIR=/tmp/opspilot
EOF
chmod 0600 "${CONFIG_DIR}/agent.env"
chown opspilot:opspilot "${CONFIG_DIR}/agent.env"

# Copy or download binary
if [ -f "./bin/${BIN_NAME}" ]; then
  cp "./bin/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
elif [ -f "./${BIN_NAME}" ]; then
  cp "./${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
fi

chmod 0755 "${INSTALL_DIR}/${BIN_NAME}"
chown -R opspilot:opspilot "${INSTALL_DIR}"

# Create Systemd Unit
echo "Creating systemd service: /etc/systemd/system/opspilot-agent.service..."
cat << EOF > /etc/systemd/system/opspilot-agent.service
[Unit]
Description=OpsPilot Infrastructure Server Agent
After=network.target network-online.target systemd-journald.service
Wants=network-online.target

[Service]
Type=simple
User=opspilot
Group=opspilot
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
systemctl enable opspilot-agent.service

echo "=== [OpsPilot] Agent installed successfully! ==="
echo "Start agent with: sudo systemctl start opspilot-agent"
echo "Check status with: sudo systemctl status opspilot-agent"
