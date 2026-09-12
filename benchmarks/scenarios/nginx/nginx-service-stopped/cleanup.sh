#!/bin/bash
set -euo pipefail
systemctl is-active nginx >/dev/null || systemctl start nginx
