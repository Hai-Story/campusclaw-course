package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtocol(t *testing.T) {
	h := handler("test-only-token", []string{"http://allowed"})
	request := func(method, body, token, origin string) (int, map[string]any) {
		r := httptest.NewRequest(method, "/mcp", strings.NewReader(body))
		r.Header.Set("Authorization", token)
		r.Header.Set("MCP-Protocol-Version", version)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	for _, token := range []string{"", "Bearer wrong"} {
		code, _ := request("POST", `{}`, token, "")
		if code != 401 {
			t.Fatal(code)
		}
	}
	code, _ := request("POST", `{}`, "Bearer test-only-token", "http://evil")
	if code != 403 {
		t.Fatal(code)
	}
	code, _ = request("GET", "", "Bearer test-only-token", "")
	if code != 405 {
		t.Fatal(code)
	}
	code, res := request("POST", `{"jsonrpc":"2.0","id":"abc","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, "Bearer test-only-token", "")
	if code != 200 || res["id"] != "abc" || res["result"] == nil {
		t.Fatal(res)
	}
	code, _ = request("POST", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "Bearer test-only-token", "")
	if code != 202 {
		t.Fatal(code)
	}
	_, res = request("POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "Bearer test-only-token", "")
	if len(res["result"].(map[string]any)["tools"].([]any)) != 2 {
		t.Fatal(res)
	}
	for _, body := range []string{`{`, `{"jsonrpc":"2.0","id":1,"method":"unknown"}`, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`} {
		_, res = request("POST", body, "Bearer test-only-token", "")
		if res["error"] == nil {
			t.Fatal(res)
		}
	}
	for _, name := range []string{"formula_hint", "unit_convert"} {
		args := map[string]any{"query": "去括号与单位检查"}
		if name == "unit_convert" {
			args["from_unit"] = "m"
			args["to_unit"] = "cm"
		}
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		_, res = request("POST", string(raw), "Bearer test-only-token", "")
		if res["result"].(map[string]any)["isError"] != false {
			t.Fatal(res)
		}
	}
}
func TestGuidanceValidation(t *testing.T) {
	for _, a := range []map[string]any{{}, {"query": 123}, {"query": "x", "extra": "x"}, {"query": strings.Repeat("中", 501)}} {
		if _, e := guidance("formula_hint", a); e == nil {
			t.Fatal(a)
		}
	}
	if _, e := guidance("unit_convert", map[string]any{"query": "x", "from_unit": "m", "to_unit": "kg"}); e == nil {
		t.Fatal("dimension mismatch")
	}
}
