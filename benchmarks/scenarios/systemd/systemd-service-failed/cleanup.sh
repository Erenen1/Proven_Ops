#!/bin/bash
set -euo pipefail
systemctl stop opspilot-test-crash.service 2>/dev/null || true
rm -f /etc/systemd/system/opspilot-test-crash.service 2>/dev/null || true
systemctl daemon-reload 2>/dev/null || true
