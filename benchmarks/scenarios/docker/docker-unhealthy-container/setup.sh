#!/bin/bash
docker rm -f opspilot-test-unhealthy 2>/dev/null || true
docker run -d --name opspilot-test-unhealthy --health-cmd="exit 1" --health-interval=1s --health-retries=1 alpine sleep 60 || true
sleep 3
