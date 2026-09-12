#!/bin/bash
mkdir -p /var/log/opspilot-bench
dd if=/dev/zero of=/var/log/opspilot-bench/runaway.log bs=1M count=10 2>/dev/null || true
