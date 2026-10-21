// CampusClaw's stateless, read-only MCP teaching service.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const version = "2025-06-18"

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema any             `json:"inputSchema"`
	Annotations map[string]bool `json:"annotations"`
}

func tools() []tool {
	str := func(description string, max int) any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max, "description": description}
	}
	return []tool{
		{"formula_hint", "学生卡在公式、去括号、移项或通分时，给检查步骤与下一步提示，不给整题最终数值。query 为学生当前问题与步骤。", map[string]any{"type": "object", "properties": map[string]any{"query": str("学生的问题与已写步骤", 500)}, "required": []string{"query"}, "additionalProperties": false}, annotations()},
		{"unit_convert", "给长度、质量或时间的单位关系与换算方向提示，不计算题目最终数值。from_unit/to_unit 使用 m、cm、mm、km、g、kg、s、min、h。", map[string]any{"type": "object", "properties": map[string]any{"query": str("学生的问题", 500), "from_unit": str("原单位", 8), "to_unit": str("目标单位", 8)}, "required": []string{"query", "from_unit", "to_unit"}, "additionalProperties": false}, annotations()},
	}
}
func annotations() map[string]bool {
	return map[string]bool{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
}
func guidance(name string, args map[string]any) (string, error) {
	allowed := map[string]int{"query": 500}
	if name == "unit_convert" {
		allowed["from_unit"] = 8
		allowed["to_unit"] = 8
	} else if name != "formula_hint" {
		return "", errors.New("工具不存在")
	}
	if len(args) != len(allowed) {
		return "", errors.New("参数字段不符合工具声明")
	}
	for k, max := range allowed {
		v, ok := args[k].(string)
		if !ok || strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > max {
			return "", errors.New("参数须为有效且长度合适的文本")
		}
	}
	q := args["query"].(string)
	if name == "formula_hint" {
		switch {
		case strings.Contains(q, "括号"):
			return "去括号时，先检查括号前的系数是否乘到括号内每一项；遇到负号，逐项核对符号，再自行合并同类项。", nil
		case strings.Contains(q, "通分"):
			return "先找分母的最小公倍数，核对分子与分母是否同乘相同因数，再自行进行下一步。", nil
		case strings.Contains(q, "移项"):
			return "移项前先确认等号两边做了同一种运算，检查移过等号的项是否正确变号，再自行整理。", nil
		default:
			return "先列出已知量与未知量，核对公式适用条件和量纲，代入前检查每一步的符号与单位。先尝试下一步，再检查是否一致。", nil
		}
	}
	groups := map[string]string{"m": "length", "cm": "length", "mm": "length", "km": "length", "g": "mass", "kg": "mass", "s": "time", "min": "time", "h": "time"}
	from, to := args["from_unit"].(string), args["to_unit"].(string)
	if groups[from] == "" || groups[from] != groups[to] {
		return "", errors.New("请选择同一量纲的受支持单位")
	}
	return "先确认 " + from + " 与 " + to + " 属于同一量纲，写出两种单位的进率。大单位换小单位用乘法，小单位换大单位用除法；相同单位无需换算。请自行代入题目数值并检查数量级。", nil
}
func handler(token string, origins []string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "需要有效的 MCP Bearer 令牌", 401)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			ok := false
			for _, v := range origins {
				if origin == v {
					ok = true
				}
			}
			if !ok {
				http.Error(w, "Origin 不允许", 403)
				return
			}
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "不提供服务器推送流", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		reply := func(result any, code int, msg string) {
			out := map[string]any{"jsonrpc": "2.0", "id": req.ID}
			if len(req.ID) == 0 {
				out["id"] = nil
			}
			if code != 0 {
				out["error"] = map[string]any{"code": code, "message": msg}
			} else {
				out["result"] = result
			}
			json.NewEncoder(w).Encode(out)
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil || !utf8.Valid(raw) || json.Unmarshal(raw, &req) != nil {
			reply(nil, -32700, "请求须为有效 UTF-8 JSON")
			return
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			reply(nil, -32600, "无效 JSON-RPC 请求")
			return
		}
		if len(req.ID) > 0 {
			var id any
			json.Unmarshal(req.ID, &id)
			switch id.(type) {
			case string, float64:
			default:
				reply(nil, -32600, "请求 ID 须为字符串或数字")
				return
			}
		}
		if req.Method != "initialize" && r.Header.Get("MCP-Protocol-Version") != version {
			reply(nil, -32600, "不支持的协议版本")
			return
		}
		if req.Method == "notifications/initialized" && len(req.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		if len(req.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string         `json:"protocolVersion"`
				Capabilities    map[string]any `json:"capabilities"`
				ClientInfo      map[string]any `json:"clientInfo"`
			}
			if json.Unmarshal(req.Params, &p) != nil || p.ProtocolVersion == "" || p.Capabilities == nil || p.ClientInfo == nil {
				reply(nil, -32602, "需要 protocolVersion、capabilities 和 clientInfo")
				return
			}
			reply(map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "campusclaw-hints", "version": "1.0.0"}}, 0, "")
		case "tools/list":
			var params map[string]any
			if len(req.Params) > 0 && (json.Unmarshal(req.Params, &params) != nil || params == nil) {
				reply(nil, -32602, "tools/list 参数须为对象")
				return
			}
			reply(map[string]any{"tools": tools()}, 0, "")
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(req.Params, &p) != nil || p.Arguments == nil {
				reply(nil, -32602, "需要工具名称和参数对象")
				return
			}
			if p.Name != "formula_hint" && p.Name != "unit_convert" {
				reply(nil, -32602, "工具不存在")
				return
			}
			text, e := guidance(p.Name, p.Arguments)
			if e != nil {
				text = e.Error()
			}
			reply(map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": e != nil}, 0, "")
		default:
			reply(nil, -32601, "方法不存在")
		}
	})
	return mux
}
func main() {
	token := strings.TrimSpace(os.Getenv("MCP_AUTH_TOKEN"))
	if token == "" {
		log.Fatal("MCP_AUTH_TOKEN is required")
	}
	addr := os.Getenv("MCP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	server := &http.Server{Addr: addr, Handler: handler(token, strings.Fields(os.Getenv("MCP_ALLOWED_ORIGINS"))), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal("MCP server could not start")
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdown)
}
