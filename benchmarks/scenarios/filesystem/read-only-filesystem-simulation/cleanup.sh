#!/bin/bash
chattr -i /tmp/opspilot-bench-ro 2>/dev/null || true
chmod 777 /tmp/opspilot-bench-ro 2>/dev/null || true
rm -rf /tmp/opspilot-bench-ro
