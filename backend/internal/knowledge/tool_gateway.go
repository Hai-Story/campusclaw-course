package knowledge

import (
	"bytes"
	"campusclaw/internal/mcp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Service) DecideTools(ctx context.Context, messages []mcp.ChatMessage, tools []mcp.Declaration) (mcp.ChatMessage, error) {
	var result struct {
		Choices []struct {
			Message      mcp.ChatMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
		} `json:"choices"`
	}
	e := s.gateway.post(ctx, "/chat/completions", map[string]any{"model": s.gateway.chat, "messages": messages, "tools": tools, "tool_choice": "auto", "stream": false, "temperature": 0.1, "max_tokens": 1000}, &result)
	if e != nil {
		return mcp.ChatMessage{}, errors.New("模型工具决策失败")
	}
	if len(result.Choices) != 1 {
		return mcp.ChatMessage{}, errors.New("模型工具决策结构无效")
	}
	v := result.Choices[0]
	if v.FinishReason != "stop" && v.FinishReason != "tool_calls" {
		return mcp.ChatMessage{}, errors.New("模型工具决策未完成")
	}
	if v.FinishReason == "tool_calls" && len(v.Message.ToolCalls) == 0 {
		return mcp.ChatMessage{}, errors.New("模型缺少工具请求")
	}
	return v.Message, nil
}
func (s *Service) StreamTools(ctx context.Context, messages []mcp.ChatMessage, emit func(string) error) error {
	raw, e := json.Marshal(map[string]any{"model": s.gateway.chat, "messages": messages, "stream": true, "temperature": 0.1, "max_tokens": 1000})
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", s.gateway.baseURL+"/chat/completions", bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+s.gateway.key)
	req.Header.Set("Content-Type", "application/json")
	client := *s.gateway.client
	client.Timeout = 0
	res, e := client.Do(req)
	if e != nil {
		return errors.New("模型流不可用")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return errors.New("模型未返回 SSE")
	}
	return readChatStream(res.Body, emit)
}
