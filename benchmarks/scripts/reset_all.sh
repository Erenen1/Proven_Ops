#!/bin/bash
set -euo pipefail

echo "[OpsPilot Reset] Restoring baseline environment..."

# 1. Kill any stray benchmark processes
pkill -9 -f "python3 -m http.server 808" || true
pkill -9 -f "python3 -m http.server 909" || true
pkill -9 -f "python3 -m http.server 800" || true
pkill -9 -f "opspilot_dummy_service" || true

# 2. Cleanup Docker test containers
if command -v docker >/dev/null 2>&1; then
    docker rm -f opspilot-test-crash opspilot-test-restart opspilot-test-port opspilot-test-unhealthy 2>/dev/null || true
fi

# 3. Cleanup test files & mounts
rm -rf /var/lib/opspilot/backups/* 2>/dev/null || true
rm -rf /tmp/opspilot-bench-* 2>/dev/null || true
rm -f /var/lib/opspilot/hardening_marker.txt 2>/dev/null || true
rm -f /var/lib/opspilot/bench_marker.txt 2>/dev/null || true
rm -f /etc/nginx/sites-enabled/bench-invalid.conf 2>/dev/null || true

# 4. Stop & disable any benchmark systemd test units
systemctl stop opspilot-test-crash.service 2>/dev/null || true
systemctl disable opspilot-test-crash.service 2>/dev/null || true
rm -f /etc/systemd/system/opspilot-test-crash.service 2>/dev/null || true
systemctl daemon-reload 2>/dev/null || true

# 5. Restore default Nginx service to healthy active state on port 8080
if [ -f /etc/nginx/sites-available/default ]; then
    nginx -t 2>/dev/null && systemctl restart nginx 2>/dev/null || true
fi

echo "[OpsPilot Reset] Environment restored cleanly."
