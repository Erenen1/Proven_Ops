#!/bin/bash
systemctl stop opspilot-restart-loop.service 2>/dev/null || true
rm -f /etc/systemd/system/opspilot-restart-loop.service 2>/dev/null || true
systemctl daemon-reload
