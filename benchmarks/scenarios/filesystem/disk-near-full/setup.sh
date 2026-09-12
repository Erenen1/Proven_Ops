#!/bin/bash
mkdir -p /tmp/opspilot-bench-disk
dd if=/dev/zero of=/tmp/opspilot-bench-disk/large_chunk.dat bs=1M count=20 2>/dev/null || true
