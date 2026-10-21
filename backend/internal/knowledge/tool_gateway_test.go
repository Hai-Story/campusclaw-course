package knowledge

import (
	"campusclaw/internal/config"
	"campusclaw/internal/mcp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolGatewayContractsAndFinalStream(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer model-test-only" {
			t.Error("wrong model credential")
		}
		var input map[string]any
		json.NewDecoder(r.Body).Decode(&input)
		if input["stream"] == true {
			if _, ok := input["tools"]; ok {
				t.Error("tools must be disabled during final generation")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"先检查\"}}]}\n\n")
			w.(http.Flusher).Flush()
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"每一项\"}}]}\n\ndata: [DONE]\n\n")
			return
		}
		if input["tool_choice"] != "auto" || len(input["tools"].([]any)) != 1 {
			t.Error("missing function declarations")
		}
		raw, _ := json.Marshal(input)
		if strings.Contains(string(raw), "mcp-secret") || strings.Contains(string(raw), "mcp-demo:8090") {
			t.Error("MCP secrets entered model request")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"a","type":"function","function":{"name":"t1","arguments":"{\"query\":\"问题\"}"}}]}}]}`)
	}))
	defer server.Close()
	s := &Service{gateway: newGateway(config.Config{GatewayBaseURL: server.URL, GatewayAPIKey: "model-test-only", ChatModel: "fixture"})}
	tool := mcp.Declaration{Type: "function", Function: mcp.FunctionSchema{Name: "t1", Description: "给提示", Parameters: json.RawMessage(`{"type":"object"}`)}}
	messages := []mcp.ChatMessage{{Role: "user", Content: "问题"}}
	choice, e := s.DecideTools(context.Background(), messages, []mcp.Declaration{tool})
	if e != nil || len(choice.ToolCalls) != 1 {
		t.Fatal(choice, e)
	}
	messages = append(messages, choice, mcp.ChatMessage{Role: "tool", ToolCallID: "a", Content: "检查每一项"})
	chunks := []string{}
	e = s.StreamTools(context.Background(), messages, func(text string) error { chunks = append(chunks, text); return nil })
	if e != nil || len(chunks) != 2 || strings.Join(chunks, "") != "先检查每一项" || requests != 2 {
		t.Fatal(chunks, e, requests)
	}
}
func TestToolGatewayRejectsIncompleteOrMalformedDecision(t *testing.T) {
	for _, body := range []string{`{}`, `{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`, `{"choices":[{"finish_reason":"tool_calls","message":{"content":null}}]}`, `{"choices":[{"finish_reason":"stop","message":{"tool_calls":"bad"}}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		s := &Service{gateway: newGateway(config.Config{GatewayBaseURL: server.URL})}
		if _, e := s.DecideTools(context.Background(), nil, nil); e == nil {
			t.Fatal(body)
		}
		server.Close()
	}
}
