#!/bin/bash
set -euo pipefail
# Verify journal log contains expected error message
journalctl -u opspilot-test-crash.service -n 20 --no-pager | grep -i "Database configuration missing"
