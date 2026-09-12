#!/bin/bash
docker rm -f opspilot-test-crash 2>/dev/null || true
docker run -d --name opspilot-test-crash alpine /bin/sh -c 'echo "FATAL: Config corrupted in /app/conf.json"; exit 1' || true
sleep 1
