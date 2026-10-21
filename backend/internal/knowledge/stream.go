package knowledge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) StreamAnswer(ctx context.Context, messages []HistoryTurn, emit func(string) error) error {
	ctx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	body, err := json.Marshal(map[string]any{"model": s.gateway.chat, "messages": messages, "stream": true, "temperature": 0.1, "max_tokens": 1000})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", s.gateway.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.gateway.key)
	client := *s.gateway.client
	client.Timeout = 110 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("stream gateway HTTP %d", resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return errors.New("gateway did not return SSE")
	}
	return readChatStream(resp.Body, emit)
}
func readChatStream(reader io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(io.LimitReader(reader, 1<<20))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	data := []string{}
	size := 0
	hasText := false
	done := false
	frame := func() error {
		if len(data) == 0 {
			return nil
		}
		payload := strings.Join(data, "\n")
		data = nil
		size = 0
		if payload == "[DONE]" {
			if !hasText {
				return errors.New("empty model answer")
			}
			done = true
			return nil
		}
		if !utf8.ValidString(payload) {
			return errors.New("invalid stream UTF-8")
		}
		var part struct {
			Error   json.RawMessage `json:"error"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &part); err != nil {
			return err
		}
		if len(part.Error) > 0 && string(part.Error) != "null" {
			return errors.New("gateway stream error")
		}
		if len(part.Choices) > 0 && part.Choices[0].Delta.Content != "" {
			hasText = true
			return emit(part.Choices[0].Delta.Content)
		}
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := frame(); err != nil {
				return err
			}
			if done {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			size += len(value)
			if size > 64<<10 {
				return errors.New("stream frame too large")
			}
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("model stream ended without DONE")
}
