package knowledge

import (
	"campusclaw/internal/config"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatStreamFrames(t *testing.T) {
	input := "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\r\n\r\n" + "data: {\"choices\":\n" + "data: [{\"delta\":{\"content\":\"中文\"}}]}\n\n" + "data: {\"choices\":[{\"delta\":{\"content\":\"增量\"}}]}\n\ndata: [DONE]\n\n"
	var parts []string
	err := readChatStream(strings.NewReader(input), func(s string) error { parts = append(parts, s); return nil })
	if err != nil || strings.Join(parts, "") != "中文增量" || len(parts) != 2 {
		t.Fatal(parts, err)
	}
	for _, bad := range []string{"", "data: {}\n\ndata: [DONE]\n\n", "data: broken\n\n", "data: {\"error\":{}}\n\n", "data: " + strings.Repeat("x", 70000) + "\n\n", strings.TrimSuffix(input, "data: [DONE]\n\n")} {
		if err = readChatStream(strings.NewReader(bad), func(string) error { return nil }); err == nil {
			t.Fatal("invalid stream accepted")
		}
	}
}
func TestStreamGatewayRequestAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["stream"] != true || r.Header.Get("Authorization") != "Bearer test" {
			t.Error("request contract")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	svc := New(nil, config.Config{GatewayBaseURL: server.URL, GatewayAPIKey: "test", ChatModel: "mock"})
	if err := svc.StreamAnswer(context.Background(), []HistoryTurn{{Role: "system", Content: "test"}}, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.StreamAnswer(ctx, nil, func(string) error { return nil }); err == nil {
		t.Fatal("cancel ignored")
	}
}
func TestEmptyMaterialScopeDoesNotUseDependencies(t *testing.T) {
	svc := &Service{}
	result, err := svc.SearchWithinMaterials(context.Background(), 1, "query", "hybrid", 4, nil)
	if err != nil || len(result.Hits) != 0 {
		t.Fatal(result, err)
	}
}
