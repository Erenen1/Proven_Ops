#!/bin/bash
set -euo pipefail

pkill -9 -f "python3 -m http.server 8080" 2>/dev/null || true
systemctl start nginx 2>/dev/null || true
