package mcp

import (
	"bytes"
	"campusclaw/internal/auth"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func serverRows(base, status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "class_id", "name", "base_url", "token_text", "status", "revision", "protocol_version", "last_error"}).AddRow(1, 2, "campus-tools", base, "test-only", status, 1, ProtocolVersion, "")
}
func expectReserve(m sqlmock.Sqlmock, available bool) {
	m.ExpectBegin()
	m.ExpectExec("INSERT IGNORE INTO mcp_conversation_usage").WithArgs(uint64(4), uint64(2), uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	n := int64(1)
	if !available {
		n = 0
	}
	m.ExpectExec("UPDATE mcp_conversation_usage").WithArgs(uint64(4), 24).WillReturnResult(sqlmock.NewResult(0, n))
	if available {
		m.ExpectCommit()
	} else {
		m.ExpectRollback()
	}
}
func expectAudit(m sqlmock.Sqlmock) {
	m.ExpectExec("INSERT INTO mcp_call_audits").WithArgs(uint64(2), uint64(1), "campus-tools", "formula_hint@campus-tools", uint64(3), uint64(4), uint64(5), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
}
func expectFinish(m sqlmock.Sqlmock, status string) {
	m.ExpectExec("UPDATE mcp_call_audits SET status").WithArgs(status, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
}
func expectCurrent(m sqlmock.Sqlmock, c Candidate, status string) {
	m.ExpectQuery("SELECT COUNT.*FROM tutor_agent_skills").WithArgs(uint64(4), uint64(2), uint64(3), uint64(6), uint64(2)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectQuery("SELECT .* FROM mcp_servers WHERE class_id").WithArgs(uint64(2), "campus-tools").WillReturnRows(serverRows(c.Server.BaseURL, status))
	if status == "connected" {
		m.ExpectQuery("SELECT t.name,t.description").WithArgs(uint64(6), uint64(1), "formula_hint").WillReturnRows(sqlmock.NewRows([]string{"name", "description", "schema", "hash", "approved"}).AddRow(c.Tool.Name, c.Tool.Description, string(c.Tool.InputSchema), c.Tool.Hash, c.Tool.Hash))
	}
}
func candidate(base string) Candidate {
	tool := testTool()
	_ = validateTool(&tool)
	return Candidate{Ref: "formula_hint@campus-tools", Server: Server{ID: 1, ClassID: 2, Name: "campus-tools", BaseURL: base, Token: "test-only", Status: "connected"}, Tool: tool, SkillID: 6, Approved: true}
}

var scope = Scope{ClassID: 2, UserID: 3, ConversationID: 4, QuestionID: 5}

type decisionFixture struct {
	t     *testing.T
	round int
	mode  string
}

func (g *decisionFixture) DecideTools(ctx context.Context, messages []ChatMessage, tools []Declaration) (ChatMessage, error) {
	g.round++
	if len(tools) != 1 || tools[0].Function.Name != "t1" || tools[0].Function.Parameters == nil {
		g.t.Fatal(tools)
	}
	if g.mode == "none" || g.round > 2 {
		return ChatMessage{Role: "assistant", Content: "可以作答"}, nil
	}
	if g.round == 2 {
		last := messages[len(messages)-1]
		if last.Role != "tool" || last.ToolCallID != "model-1" {
			g.t.Fatal("missing tool result", last)
		}
	}
	return ChatMessage{Role: "assistant", ToolCalls: []ModelCall{{ID: fmt.Sprintf("model-%d", g.round), Type: "function", Function: Function{Name: "t1", Arguments: `{"query":"去括号"}`}}}}, nil
}
func (g *decisionFixture) StreamTools(ctx context.Context, m []ChatMessage, f func(string) error) error {
	return errors.New("Run must leave final streaming to the caller")
}
func TestTwoRoundModelSelectedExecution(t *testing.T) {
	var count atomic.Int32
	remote := rpcServer(t, "ok", &count)
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig(remote.URL))
	c := candidate(remote.URL)
	for i := 0; i < 2; i++ {
		expectReserve(m, true)
		expectAudit(m)
		expectCurrent(m, c, "connected")
		expectFinish(m, "ok")
	}
	gateway := &decisionFixture{t: t}
	batches := 0
	messages, calls, e := svc.Run(context.Background(), scope, []Candidate{c}, gateway, []ChatMessage{{Role: "user", Content: "两步学习问题"}}, func(batch []CallInfo) error { batches++; return nil })
	if e != nil || len(calls) != 2 || batches != 2 || count.Load() != 2 || calls[0].Round != 1 || calls[1].Round != 2 {
		t.Fatal(calls, batches, e)
	}
	if messages[len(messages)-1].Role != "system" || calls[0].CallID == calls[1].CallID {
		t.Fatal("missing termination or duplicate host ids")
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestModelMayChooseNoTool(t *testing.T) {
	svc := New(nil, testConfig("http://mcp-demo:8090"))
	_, calls, e := svc.Run(context.Background(), scope, []Candidate{candidate("http://mcp-demo:8090")}, &decisionFixture{t: t, mode: "none"}, nil, func([]CallInfo) error { return nil })
	if e != nil || len(calls) != 0 {
		t.Fatal(calls, e)
	}
}
func TestHostRejectionAndFailClosedAudit(t *testing.T) {
	for _, mode := range []string{"arguments", "quota", "disconnected", "audit-failure"} {
		t.Run(mode, func(t *testing.T) {
			var count atomic.Int32
			remote := rpcServer(t, "ok", &count)
			defer remote.Close()
			db, m, _ := sqlmock.New()
			defer db.Close()
			svc := New(db, testConfig(remote.URL))
			c := candidate(remote.URL)
			expectReserve(m, mode != "quota")
			if mode == "audit-failure" {
				m.ExpectExec("INSERT INTO mcp_call_audits").WillReturnError(errors.New("storage unavailable"))
			} else {
				expectAudit(m)
			}
			raw := `{"query":"问题"}`
			if mode == "arguments" {
				raw = `{"query":1}`
			}
			if mode == "disconnected" {
				expectCurrent(m, c, "disconnected")
			}
			if mode != "audit-failure" {
				expectFinish(m, "denied")
			}
			info, e := svc.Execute(context.Background(), scope, c, raw, 1, true)
			if (mode == "audit-failure") != (e != nil) || info.Status != "denied" || count.Load() != 0 {
				t.Fatal(info, e, count.Load())
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestUnknownReferenceCannotWriteAnotherServersAudit(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig("http://mcp-demo:8090"))
	expectReserve(m, true)
	info, e := svc.Execute(context.Background(), scope, Candidate{Ref: "missing@other-class"}, "{}", 0, true)
	if e != nil || info.Status != "denied" {
		t.Fatal(info, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestApprovalRejectsForeignOrDisconnectedAndKeepsTransaction(t *testing.T) {
	for _, mode := range []string{"foreign", "disconnected"} {
		t.Run(mode, func(t *testing.T) {
			db, m, _ := sqlmock.New()
			defer db.Close()
			svc := New(db, testConfig("http://mcp-demo:8090"))
			m.ExpectBegin()
			q := m.ExpectQuery("SELECT .* FROM mcp_servers WHERE class_id").WithArgs(uint64(2), "campus-tools")
			if mode == "foreign" {
				q.WillReturnError(sql.ErrNoRows)
			} else {
				q.WillReturnRows(serverRows("http://mcp-demo:8090", "disconnected"))
			}
			m.ExpectRollback()
			tx, _ := db.Begin()
			e := svc.Approve(context.Background(), tx, 6, 2, 3, []string{"formula_hint@campus-tools"})
			tx.Rollback()
			if !errors.Is(e, ErrReference) {
				t.Fatal(e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestAtomicConnectionReplacement(t *testing.T) {
	var count atomic.Int32
	remote := rpcServer(t, "ok", &count)
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig(remote.URL))
	for i := 0; i < 2; i++ {
		m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(serverRows(remote.URL, "disconnected"))
		m.ExpectExec("UPDATE mcp_servers SET status='disconnected' WHERE").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(serverRows(remote.URL, "disconnected"))
		m.ExpectBegin()
		m.ExpectQuery("SELECT tool_name,new_json FROM mcp_tool_changes").WillReturnRows(sqlmock.NewRows([]string{"name", "new_json"}))
		m.ExpectExec("DELETE FROM mcp_tools").WithArgs(uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec("INSERT INTO mcp_tool_changes").WillReturnResult(sqlmock.NewResult(1, 1))
		m.ExpectExec("INSERT INTO mcp_tools").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec("UPDATE mcp_servers SET status='connected'").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectCommit()
		v, tools, e := svc.connect(context.Background(), 1, 2)
		if e != nil || v.Status != "connected" || len(tools) != 1 {
			t.Fatal(v, tools, e)
		}
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

type repeatingFixture struct {
	count int
	multi bool
}

func (g *repeatingFixture) DecideTools(ctx context.Context, m []ChatMessage, tools []Declaration) (ChatMessage, error) {
	g.count++
	calls := []ModelCall{{ID: fmt.Sprintf("r%d", g.count), Type: "function", Function: Function{Name: "t1", Arguments: `{"query":"问题"}`}}}
	if g.multi {
		calls = append(calls, ModelCall{ID: fmt.Sprintf("r%d-b", g.count), Type: "function", Function: Function{Name: "t1", Arguments: `{"query":"问题"}`}})
	}
	return ChatMessage{ToolCalls: calls}, nil
}
func (g *repeatingFixture) StreamTools(context.Context, []ChatMessage, func(string) error) error {
	return nil
}
func TestConversationBudgetStopsFurtherModelRounds(t *testing.T) {
	var count atomic.Int32
	remote := rpcServer(t, "ok", &count)
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig(remote.URL))
	c := candidate(remote.URL)
	expectReserve(m, false)
	expectAudit(m)
	expectFinish(m, "denied")
	g := &repeatingFixture{}
	_, calls, e := svc.Run(context.Background(), scope, []Candidate{c}, g, nil, func([]CallInfo) error { return nil })
	if e != nil || len(calls) != 1 || calls[0].Status != "denied" || count.Load() != 0 || g.count != 1 {
		t.Fatal(calls, e, count.Load(), g.count)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestMultiCallBatchHonorsPerQuestionLimit(t *testing.T) {
	var count atomic.Int32
	remote := rpcServer(t, "ok", &count)
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	cfg := testConfig(remote.URL)
	cfg.MaxCalls = 1
	svc := New(db, cfg)
	c := candidate(remote.URL)
	expectReserve(m, true)
	expectAudit(m)
	expectCurrent(m, c, "connected")
	expectFinish(m, "ok")
	expectAudit(m)
	expectFinish(m, "denied")
	_, calls, e := svc.Run(context.Background(), scope, []Candidate{c}, &repeatingFixture{multi: true}, nil, func([]CallInfo) error { return nil })
	if e != nil || len(calls) != 2 || calls[0].Status != "ok" || calls[1].Status != "denied" || count.Load() != 1 {
		t.Fatal(calls, e, count.Load())
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestMetadataHashChangeRevokesApproval(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig("http://mcp-demo:8090"))
	m.ExpectQuery("SELECT .* FROM mcp_servers WHERE class_id").WillReturnRows(serverRows("http://mcp-demo:8090", "connected"))
	tool := testTool()
	m.ExpectQuery("SELECT t.name").WillReturnRows(sqlmock.NewRows([]string{"name", "description", "schema", "hash", "approved"}).AddRow(tool.Name, "新说明", []byte(tool.InputSchema), "new-hash", "old-hash"))
	items, e := svc.Candidates(context.Background(), 2, 6, []string{"formula_hint@campus-tools"})
	if e != nil || len(items) != 1 || items[0].Approved {
		t.Fatal(items, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestDisabledSkillCannotDispatch(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig("http://mcp-demo:8090"))
	c := candidate("http://mcp-demo:8090")
	expectReserve(m, true)
	expectAudit(m)
	m.ExpectQuery("SELECT COUNT.*FROM tutor_agent_skills").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	expectFinish(m, "denied")
	info, e := svc.Execute(context.Background(), scope, c, `{"query":"问题"}`, 1, true)
	if e != nil || info.Status != "denied" {
		t.Fatal(info, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestDisconnectCancelsInflightAndDeniesLaterDispatch(t *testing.T) {
	started := make(chan struct{})
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		calls.Add(1)
		close(started)
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig(remote.URL))
	c := candidate(remote.URL)
	expectReserve(m, true)
	expectAudit(m)
	expectCurrent(m, c, "connected")
	m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WithArgs(uint64(1), uint64(2)).WillReturnRows(serverRows(remote.URL, "connected"))
	m.ExpectBegin()
	m.ExpectExec("DELETE FROM mcp_tools").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("UPDATE mcp_servers SET status='disconnected'").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	expectFinish(m, "error")
	finished := make(chan CallInfo, 1)
	go func() {
		info, e := svc.Execute(context.Background(), scope, c, `{"query":"问题"}`, 1, true)
		if e != nil {
			t.Error(e)
		}
		finished <- info
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request never started")
	}
	if _, e := svc.disconnect(context.Background(), 1, 2); e != nil {
		t.Fatal(e)
	}
	select {
	case info := <-finished:
		if info.Status != "error" {
			t.Fatal(info)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request not canceled")
	}
	expectReserve(m, true)
	expectAudit(m)
	expectCurrent(m, c, "disconnected")
	expectFinish(m, "denied")
	info, e := svc.Execute(context.Background(), scope, c, `{"query":"问题"}`, 2, true)
	if e != nil || info.Status != "denied" || calls.Load() != 1 {
		t.Fatal(info, e, calls.Load())
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestTwoCallsInOneRoundBothRun(t *testing.T) {
	var count atomic.Int32
	remote := rpcServer(t, "ok", &count)
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	cfg := testConfig(remote.URL)
	cfg.MaxRounds = 1
	svc := New(db, cfg)
	c := candidate(remote.URL)
	for i := 0; i < 2; i++ {
		expectReserve(m, true)
		expectAudit(m)
		expectCurrent(m, c, "connected")
		expectFinish(m, "ok")
	}
	_, calls, e := svc.Run(context.Background(), scope, []Candidate{c}, &repeatingFixture{multi: true}, nil, func(batch []CallInfo) error {
		if len(batch) != 2 {
			t.Fatal(batch)
		}
		return nil
	})
	if e != nil || len(calls) != 2 || count.Load() != 2 || calls[0].Round != 1 || calls[1].Round != 1 {
		t.Fatal(calls, e, count.Load())
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestSlowDiscoveryCannotReconnectAfterDisconnect(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		var result any
		switch req.Method {
		case "initialize":
			close(started)
			<-release
			result = map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "notifications/initialized":
			w.WriteHeader(202)
			return
		case "tools/list":
			result = map[string]any{"tools": []Tool{testTool()}}
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer remote.Close()
	db, m, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, testConfig(remote.URL))
	m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(serverRows(remote.URL, "connected"))
	m.ExpectExec("UPDATE mcp_servers SET status='disconnected' WHERE").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(serverRows(remote.URL, "disconnected"))
	m.ExpectBegin()
	m.ExpectExec("DELETE FROM mcp_tools").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("UPDATE mcp_servers SET status='disconnected'").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(sqlmock.NewRows([]string{"id", "class_id", "name", "base_url", "token_text", "status", "revision", "protocol_version", "last_error"}).AddRow(1, 2, "campus-tools", remote.URL, "test-only", "disconnected", 2, ProtocolVersion, ""))
	finished := make(chan error, 1)
	go func() { _, _, e := svc.connect(context.Background(), 1, 2); finished <- e }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("discovery not started")
	}
	if _, e := svc.disconnect(context.Background(), 1, 2); e != nil {
		t.Fatal(e)
	}
	close(release)
	if e := <-finished; !errors.Is(e, errConnectionChanged) {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestUpdatingTokenCanPreserveOrExplicitlyClear(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(fmt.Sprint(clear), func(t *testing.T) {
			db, m, _ := sqlmock.New()
			defer db.Close()
			base := "http://mcp-demo:8090"
			svc := New(db, testConfig(base))
			m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(serverRows(base, "connected"))
			m.ExpectBegin()
			value := "test-only"
			payload := map[string]any{"name": "campus-tools", "base_url": base}
			if clear {
				value = ""
				payload["token"] = ""
			}
			m.ExpectExec("UPDATE mcp_servers SET name").WithArgs("campus-tools", base, value, uint64(1), uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec("DELETE FROM mcp_tools").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			m.ExpectQuery("SELECT .* FROM mcp_servers WHERE id").WillReturnRows(sqlmock.NewRows([]string{"id", "class_id", "name", "base_url", "token_text", "status", "revision", "protocol_version", "last_error"}).AddRow(1, 2, "campus-tools", base, value, "disconnected", 2, ProtocolVersion, ""))
			raw, _ := json.Marshal(payload)
			r := httptest.NewRequest("PUT", "/", bytes.NewReader(raw))
			r.SetPathValue("id", "1")
			r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: 3, ClassID: 2, Role: "teacher"}))
			w := httptest.NewRecorder()
			svc.WriteServer(w, r)
			if w.Code != 200 || strings.Contains(w.Body.String(), "test-only") {
				t.Fatal(w.Code, w.Body.String())
			}
			var v Server
			json.Unmarshal(w.Body.Bytes(), &v)
			if v.HasToken == clear {
				t.Fatal(v)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
