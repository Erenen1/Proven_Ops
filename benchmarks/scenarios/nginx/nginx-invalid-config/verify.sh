#!/bin/bash
set -euo pipefail
# Verify configuration is valid and service is still running
nginx -t && systemctl is-active nginx
