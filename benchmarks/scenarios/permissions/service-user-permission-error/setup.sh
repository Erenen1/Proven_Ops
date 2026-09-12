#!/bin/bash
cat <<'EOF' > /etc/systemd/system/opspilot-user-perm.service
[Unit]
Description=Test Perm Service
[Service]
User=nobody
ExecStart=/bin/bash -c 'touch /root/unauthorized_test.txt'
EOF
systemctl daemon-reload
systemctl start opspilot-user-perm.service 2>/dev/null || true
