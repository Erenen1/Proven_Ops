#!/bin/bash
curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:9095/ | grep 500
