"""Tiny OpenAI-compatible server used by widget_test.zsh. Not a real model."""
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        user = body["messages"][-1]["content"]
        if "wipe" in user:
            reply = {"command": "rm -rf ./build", "explanation": "Deletes build.", "risk": "safe"}
        elif "impossible" in user:
            reply = {"command": "", "explanation": "A shell can't make coffee.", "risk": "safe"}
        else:
            reply = {"command": "echo NLSH_$((40+2))", "explanation": "Prints a marker.", "risk": "safe"}
        out = json.dumps({"choices": [{"message": {"content": json.dumps(reply)}}]}).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(out)))
        self.end_headers()
        self.wfile.write(out)
    def log_message(self, *a): pass

HTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
