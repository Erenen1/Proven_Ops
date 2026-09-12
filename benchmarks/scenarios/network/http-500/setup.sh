#!/bin/bash
pkill -9 -f "bench_http_500" 2>/dev/null || true
nohup python3 -c '
from http.server import HTTPServer, BaseHTTPRequestHandler
class H(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(500)
        self.end_headers()
        self.wfile.write(b"Internal Server Error")
HTTPServer(("0.0.0.0", 9095), H).serve_forever()
' >/dev/null 2>&1 &
sleep 1
