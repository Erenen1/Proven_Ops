#!/bin/bash
docker rm -f opspilot-test-port 2>/dev/null || true
docker run -d -p 8080:80 --name opspilot-test-port nginx 2>/dev/null || true
