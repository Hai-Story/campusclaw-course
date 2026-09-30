#!/usr/bin/env python3
"""Deterministic OpenAI-compatible gateway for local integration tests only."""

import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


COUNTS = {"embeddings": 0, "chat": 0}
LOCK = threading.Lock()
DIMENSION = int(os.environ.get("MOCK_EMBEDDING_DIMENSION", "8"))


def embedding(text):
    vector = [0.0] * DIMENSION
    if any(word in text for word in ("文本细读", "阅读分析", "课堂观察")):
        position = 0
    elif any(word in text for word in ("方程建模", "数学建模", "错因分类")):
        position = 1
    elif any(word in text for word in ("自动验收", "验收材料")):
        position = 2
    elif any(word in text for word in ("天气", "比分", "明日气温")):
        position = 6
    else:
        position = 7
    vector[position] = 1.0
    return vector


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def send_json(self, status, payload):
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/stats":
            with LOCK:
                return self.send_json(200, dict(COUNTS))
        self.send_json(404, {"error": "not found"})

    def do_POST(self):
        try:
            size = int(self.headers.get("Content-Length", "0"))
            if size <= 0 or size > 65536:
                return self.send_json(400, {"error": "invalid length"})
            payload = json.loads(self.rfile.read(size))
        except (ValueError, json.JSONDecodeError):
            return self.send_json(400, {"error": "invalid JSON"})
        if self.path == "/v1/embeddings":
            with LOCK:
                COUNTS["embeddings"] += 1
            return self.send_json(200, {"data": [{"embedding": embedding(str(payload.get("input", "")))}]})
        if self.path == "/v1/chat/completions":
            with LOCK:
                COUNTS["chat"] += 1
            return self.send_json(200, {"choices": [{"message": {"content": "根据本班资料，相关内容可在所引切片中核对。[1]"}}]})
        self.send_json(404, {"error": "not found"})


if __name__ == "__main__":
    port = int(os.environ.get("MOCK_GATEWAY_PORT", "18765"))
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()
