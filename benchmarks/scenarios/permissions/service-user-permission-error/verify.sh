#!/bin/bash
journalctl -u opspilot-user-perm.service -n 10 --no-pager | grep -i "permission denied"
