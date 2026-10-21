#!/usr/bin/env python3
"""Deterministic OpenAI-compatible gateway for local integration tests only."""

import json
import os
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


COUNTS = {"embeddings": 0, "chat": 0, "last_messages": []}
LOCK = threading.Lock()
DIMENSION = int(os.environ.get("MOCK_EMBEDDING_DIMENSION", "8"))


def embedding(text):
    vector = [0.0] * DIMENSION
    if any(word in text for word in ("文本细读", "阅读分析", "课堂观察")):
        position = 0
    elif any(word in text for word in ("方程", "数学建模", "错因分类", "去括号", "移项")):
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
                COUNTS["last_messages"] = payload.get("messages", [])
            if payload.get("tools") and not payload.get("stream"):
                messages = payload.get("messages", [])
                question = next((m.get("content", "") for m in reversed(messages) if m.get("role") == "user"), "")
                completed = sum(m.get("role") == "tool" for m in messages)
                declarations = payload["tools"]
                by_tool = {"formula_hint": None, "unit_convert": None}
                for tool in declarations:
                    for name in by_tool:
                        if name in tool["function"]["description"]:
                            by_tool[name] = tool["function"]["name"]
                def call(name, index=0):
                    args = {"query": "请检查去括号与单位步骤"}
                    if name == "unit_convert": args.update(from_unit="m", to_unit="kg" if "MCP工具失败" in question else "cm")
                    if "MCP参数错误" in question: args = {"query": 3}
                    return {"id": f"mock_call_{completed}_{index}", "type": "function", "function": {"name": "unapproved_tool" if "MCP未知" in question else by_tool.get(name) or declarations[0]["function"]["name"], "arguments": json.dumps(args, ensure_ascii=False)}}
                calls = []
                if "MCP不调用" not in question:
                    if "MCP循环" in question or completed == 0: calls = [call("unit_convert" if "MCP单位" in question or "MCP工具失败" in question else "formula_hint")]
                    if "MCP批量" in question and completed == 0: calls = [call("formula_hint", i) for i in range(8)]
                    if "MCP多调用" in question and completed == 0: calls = [call("formula_hint", 0), call("unit_convert", 1)]
                    if "MCP双步骤" in question and completed == 1: calls = [call("unit_convert")]
                return self.send_json(200, {"choices": [{"message": {"role": "assistant", "content": None if calls else "可以回答了", **({"tool_calls": calls} if calls else {})}, "finish_reason": "tool_calls" if calls else "stop"}]})
            if not payload.get("stream"):
                return self.send_json(200, {"choices": [{"message": {"content": "根据本班资料，相关内容可在所引切片中核对。[1]"}}]})
            messages = payload.get("messages", [])
            system = messages[0].get("content", "") if messages else ""
            question = next((m.get("content", "") for m in reversed(messages) if m.get("role") == "user"), "")
            if "mock-before" in question:
                return self.send_json(503, {"error": "test gateway unavailable"})
            tool_results = [m.get("content", "") for m in messages if m.get("role") == "tool"]
            if tool_results:
                text = "工具提示已核对：请逐项检查去括号的系数与符号，再核对单位进率和换算方向，自行完成下一步。"
                if any("error" in t or "denied" in t for t in tool_results): text = "本次部分工具未能完成。你仍可先检查括号前系数是否乘到每一项，并自行核对单位。"
            elif "本轮必须先判断" in system:
                text = "你在去括号这一步漏乘了一项。先检查括号内每一项是否都乘到了，再自己继续计算。[1]"
            elif "没有资料依据" in system:
                text = "可以从题目给出的条件开始分析，列出已知量和要解决的问题，再逐步验证你的想法。[1]"
            else:
                text = "直接回答：2(x+3)=10，去括号后 2x+6=10，所以 x=2。[1]"
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.end_headers()
            try:
                for index in range(0, len(text), 5):
                    frame = {"choices": [{"delta": {"content": text[index:index + 5]}}]}
                    data = ("data: " + json.dumps(frame, ensure_ascii=False) + "\n\n").encode()
                    self.wfile.write(data)
                    self.wfile.flush()
                    if "mock-error" in question and index >= 5:
                        return
                    time.sleep(0.15 if "mock-slow" in question else 0.025)
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()
            except (BrokenPipeError, ConnectionResetError):
                pass
            return
        self.send_json(404, {"error": "not found"})


if __name__ == "__main__":
    port = int(os.environ.get("MOCK_GATEWAY_PORT", "18765"))
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()
