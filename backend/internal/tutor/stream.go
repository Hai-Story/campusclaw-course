package tutor

import (
	"campusclaw/internal/httpx"
	"campusclaw/internal/mcp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// @openapi {"method":"post","path":"/api/conversations/{id}/messages","id":"post_api_conversations_by_id_messages","summary":"连续提问并流式回答","input":"MessageInput","output":"","success":200,"codes":[400,401,404,409,503],"produces":"text/event-stream","description":"content 为 2–500 字。智能体固定，配置每轮重读。首帧 meta 带 skill/citations；工具调用逐轮补发 meta.tool_calls（ref/status/text/round/call_id），先于正文，无调用省略；delta 的 text 是新增正文；done 的 message_id 表示完整回答已落库；error 的 error 表示失败，部分正文以 incomplete 保存。"}
func (s *Service) SendMessage(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Content = strings.TrimSpace(input.Content)
	n := utf8.RuneCountInString(input.Content)
	if n < 2 || n > 500 {
		httpx.Error(w, 400, "问题须为 2–500 字")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 110*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	c, err := s.conversation(r.Context(), objectID(r), u)
	if err != nil {
		fail(w, err)
		return
	}
	if !s.acquire(c.ID) {
		httpx.Error(w, 409, "此会话正在生成回答，请稍后再试")
		return
	}
	defer s.release(c.ID)
	agent, skills, err := s.config(r.Context(), c.AgentID, u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	skill := RouteSkill(input.Content, skills)
	if _, ok = w.(http.Flusher); !ok {
		httpx.Error(w, 503, "流式响应不可用")
		return
	}
	if err = s.saveQuestion(r.Context(), c, input.Content); err != nil {
		fail(w, err)
		return
	}
	result, err := s.knowledge.SearchWithinMaterials(r.Context(), u.ClassID, input.Content, "hybrid", 4, agent.MaterialIDs)
	if err != nil {
		httpx.Error(w, 503, "检索服务暂时不可用")
		return
	}
	hits := Evidence(result.Hits, skill)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	var emitMu sync.Mutex
	emit := func(event string, value any) error {
		emitMu.Lock()
		defer emitMu.Unlock()
		data, e := json.Marshal(value)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); e != nil {
			return e
		}
		return http.NewResponseController(w).Flush()
	}
	var selected any
	if skill != nil {
		selected = map[string]any{"id": skill.ID, "name": skill.Name}
	}
	if err = emit("meta", map[string]any{"status": "ok", "skill": selected, "citations": hits}); err != nil {
		return
	}
	history, err := s.messages(r.Context(), c.ID, true)
	if err != nil {
		_ = emit("error", map[string]string{"error": "历史读取失败，请稍后重试"})
		return
	}
	filter := citationFilter{count: len(hits)}
	var answer strings.Builder
	delta := func(text string) error {
		if text == "" {
			return nil
		}
		if answer.Len()+len(text) > 64<<10 {
			return errors.New("answer too long")
		}
		answer.WriteString(text)
		return emit("delta", map[string]string{"text": text})
	}
	var calls []mcp.CallInfo
	turns := BuildMessages(agent, skill, history, hits)
	streamDelta := func(text string) error { return delta(filter.feed(text, false)) }
	candidates := []mcp.Candidate{}
	if s.MCP != nil && skill != nil {
		candidates, err = s.MCP.Candidates(r.Context(), u.ClassID, skill.ID, skill.Tools)
	}
	if err == nil && len(candidates) > 0 {
		gateway, ok := s.knowledge.(mcp.Gateway)
		if !ok {
			err = errors.New("工具网关不可用")
		} else {
			toolMessages := []mcp.ChatMessage{}
			for _, turn := range turns {
				toolMessages = append(toolMessages, mcp.ChatMessage{Role: turn.Role, Content: turn.Content})
			}
			questionID := uint64(0)
			if len(history) > 0 {
				questionID = history[len(history)-1].ID
			}
			stop := make(chan struct{})
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-stop:
						return
					case <-r.Context().Done():
						return
					case <-ticker.C:
						emitMu.Lock()
						_, _ = fmt.Fprint(w, ": tools pending\n\n")
						_ = http.NewResponseController(w).Flush()
						emitMu.Unlock()
					}
				}
			}()
			toolMessages, calls, err = s.MCP.Run(r.Context(), mcp.Scope{ClassID: u.ClassID, UserID: u.ID, ConversationID: c.ID, QuestionID: questionID}, candidates, gateway, toolMessages, func(batch []mcp.CallInfo) error {
				return emit("meta", map[string]any{"status": "ok", "skill": selected, "citations": hits, "tool_calls": batch})
			})
			close(stop)
			<-finished
			if err == nil {
				err = gateway.StreamTools(r.Context(), toolMessages, streamDelta)
			}
		}
	} else if err == nil {
		err = s.knowledge.StreamAnswer(r.Context(), turns, streamDelta)
	}
	if err == nil {
		err = delta(filter.feed("", true))
	}
	if err == nil && strings.TrimSpace(answer.String()) == "" {
		err = errors.New("empty answer")
	}
	status := "complete"
	if err != nil || r.Context().Err() != nil {
		status = "incomplete"
	}
	name := ""
	if skill != nil {
		name = skill.Name
	}
	var messageID uint64
	var saveErr error
	if answer.Len() > 0 {
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Second)
		messageID, saveErr = s.saveAnswer(saveCtx, c.ID, answer.String(), name, status, hits, calls)
		cancel()
	}
	if err != nil || saveErr != nil || status == "incomplete" {
		_ = emit("error", map[string]string{"error": "回答未完成，请稍后重试"})
		return
	}
	_ = emit("done", map[string]any{"message_id": messageID})
}
