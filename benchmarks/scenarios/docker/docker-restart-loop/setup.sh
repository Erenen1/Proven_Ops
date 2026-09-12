#!/bin/bash
docker rm -f opspilot-test-restart 2>/dev/null || true
docker run -d --restart=always --name opspilot-test-restart alpine /bin/sh -c 'echo "PANIC: Out of memory"; exit 1' || true
sleep 1
