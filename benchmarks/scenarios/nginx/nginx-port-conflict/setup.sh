#!/bin/bash
set -euo pipefail

# Stop nginx first so we can bind 8080 with a rogue process
systemctl stop nginx 2>/dev/null || true
pkill -9 -f "python3 -m http.server 8080" 2>/dev/null || true
sleep 1

# Start rogue process on 8080
nohup python3 -m http.server 8080 >/dev/null 2>&1 &
sleep 1
ss -tulpn | grep 8080
