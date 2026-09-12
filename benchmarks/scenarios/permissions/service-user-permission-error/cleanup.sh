#!/bin/bash
systemctl stop opspilot-user-perm.service 2>/dev/null || true
rm -f /etc/systemd/system/opspilot-user-perm.service
systemctl daemon-reload
