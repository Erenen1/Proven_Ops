#!/bin/bash
set -euo pipefail
dpkg -s nginx >/dev/null && systemctl is-active nginx >/dev/null
