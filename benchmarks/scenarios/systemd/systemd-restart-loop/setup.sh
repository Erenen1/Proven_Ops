#!/bin/bash
cat <<'EOF' > /etc/systemd/system/opspilot-restart-loop.service
[Unit]
Description=Test Restart Loop
[Service]
ExecStart=/bin/bash -c 'exit 1'
Restart=on-failure
RestartSec=1s
EOF
systemctl daemon-reload
systemctl start opspilot-restart-loop.service 2>/dev/null || true
