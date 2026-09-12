#!/bin/bash
docker logs opspilot-test-crash 2>&1 | grep "FATAL: Config corrupted"
