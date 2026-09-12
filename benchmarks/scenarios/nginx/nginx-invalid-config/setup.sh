#!/bin/bash
set -euo pipefail
# Ensure nginx is installed and active
systemctl is-active nginx >/dev/null || systemctl start nginx
