#!/bin/bash
systemctl is-failed opspilot-restart-loop.service || systemctl is-active opspilot-restart-loop.service || true
