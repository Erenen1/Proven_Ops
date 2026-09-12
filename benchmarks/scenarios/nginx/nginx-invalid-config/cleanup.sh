#!/bin/bash
set -euo pipefail
# Restore clean default site config if needed
nginx -t || (rm -f /etc/nginx/sites-enabled/bench-invalid.conf && systemctl restart nginx)
