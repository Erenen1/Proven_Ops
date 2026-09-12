#!/bin/bash
set -euo pipefail

cat <<'EOF' > /etc/systemd/system/opspilot-test-crash.service
[Unit]
Description=OpsPilot Test Crash Service
After=network.target

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'echo "CRITICAL: Database configuration missing /etc/opspilot-db.conf"; exit 1'

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl start opspilot-test-crash.service 2>/dev/null || true
