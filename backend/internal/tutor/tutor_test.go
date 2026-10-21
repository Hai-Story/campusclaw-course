package tutor

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/knowledge"
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const source = "名称：数学解题引导\n适用条件：方程,去括号\n资料：数学\n工具：knowledge_search\n启用：是\n\n只提示检查方向。"

func TestSkillValidation(t *testing.T) {
	valid, err := ParseSkill(source)
	if err != nil || valid.Source != source || !valid.Enabled || len(valid.Tools) != 1 {
		t.Fatalf("%+v %v", valid, err)
	}
	invalid := []string{"", strings.Replace(source, "名称：数学解题引导\n", "", 1), strings.Replace(source, "启用：是", "启用：maybe", 1), strings.Replace(source, "knowledge_search", "shell", 1), strings.Replace(source, "资料：数学", "未知：数学", 1), "名称：重复\n" + source, strings.Replace(source, "\n\n只提示检查方向。", "", 1), strings.Replace(source, "数学解题引导", strings.Repeat("数", 33), 1), strings.Replace(source, "方程,去括号", strings.Repeat("方", 121), 1), strings.Replace(source, "资料：数学", "资料："+strings.Repeat("数", 33), 1), strings.Replace(source, "只提示检查方向。", strings.Repeat("指", 2001), 1), strings.Repeat("字", 8001), string([]byte{0xff})}
	for _, input := range invalid {
		if _, err := ParseSkill(input); err == nil {
			t.Fatalf("accepted invalid source %.80q", input)
		}
	}
	if _, err := ParseSkill("\n# 注释\n" + strings.ReplaceAll(source, "：", ":")); err != nil {
		t.Fatal(err)
	}
}
func TestRouteAndPrompt(t *testing.T) {
	skills := []Skill{{ID: 9, Enabled: true, Trigger: "Equation，方程;；|", Name: "数学解题引导", Description: "guide", Subject: "数学"}, {ID: 2, Enabled: true, Trigger: "EQUATION、方程", Name: "数学解题引导", Description: "guide"}, {ID: 1, Enabled: false, Trigger: "equation,方程"}}
	if s := RouteSkill("equation 方程", skills); s == nil || s.ID != 2 {
		t.Fatalf("%+v", s)
	}
	if RouteSkill("天气", skills) != nil {
		t.Fatal("zero score")
	}
	history := []Message{{Role: "system", Content: "hostile"}, {Role: "user", Content: ""}}
	for i := 0; i < 8; i++ {
		history = append(history, Message{Role: "assistant", Content: strings.Repeat("字", 600)})
	}
	history = append(history, Message{Role: "user", Content: "唯一当前问题"})
	hits := Evidence([]knowledge.Hit{{Title: "数学", Excerpt: strings.Repeat("数", 900)}, {Title: "语文", Excerpt: "unbound subject"}, {Title: "数学", Excerpt: " "}}, &skills[0])
	if len(hits) != 1 || hits[0].Rank != 1 || len([]rune(hits[0].Excerpt)) != 800 {
		t.Fatal(hits)
	}
	msgs := BuildMessages(Agent{}, &skills[0], history, hits)
	if len(msgs) != 7 || strings.Contains(msgs[0].Content, strings.Repeat("数", 20)) || !strings.Contains(msgs[0].Content, "不写最终得数") {
		t.Fatal(msgs)
	}
	if strings.Count(msgs[len(msgs)-1].Content, "唯一当前问题") != 1 || len([]rune(msgs[1].Content)) != 500 {
		t.Fatal("history budget")
	}
	no := BuildMessages(Agent{}, nil, history, nil)
	if !strings.Contains(no[0].Content, "仍按角色") || !strings.Contains(no[0].Content, "请直接回答") {
		t.Fatal(no)
	}
}
func TestExample(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/examples/math-guidance.txt")
	if os.IsNotExist(err) {
		t.Skip("example outside backend-only checkout")
	}
	if err != nil {
		t.Fatal(err)
	}
	skill, err := ParseSkill(string(raw))
	if err != nil || RouteSkill("方程怎么移项", []Skill{skill}) == nil {
		t.Fatal(err)
	}
}
func TestCitationFilter(t *testing.T) {
	for _, c := range []struct {
		count int
		parts []string
		want  string
	}{{0, []string{"中[", "1", "]文[99]尾"}, "中文尾"}, {1, []string{"用[1]", "排[", "2]尾["}, "用[1]排尾"}, {1, []string{"[普通] [", "999999999999999999999999999999999999999999]"}, "[普通] "}} {
		f := citationFilter{count: c.count}
		var b strings.Builder
		for _, p := range c.parts {
			b.WriteString(f.feed(p, false))
		}
		b.WriteString(f.feed("", true))
		if b.String() != c.want {
			t.Fatalf("got %q want %q", b.String(), c.want)
		}
	}
}
func TestAuthorizationBeforeDatabase(t *testing.T) {
	svc := New(nil, nil)
	for _, handler := range []http.HandlerFunc{svc.WriteSkill, svc.DeleteSkill, svc.WriteAgent} {
		for _, logged := range []bool{false, true} {
			r := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
			if logged {
				r = r.WithContext(auth.WithUser(r.Context(), auth.User{Role: "student", ClassID: 1}))
			}
			w := httptest.NewRecorder()
			handler(w, r)
			want := 401
			if logged {
				want = 403
			}
			if w.Code != want {
				t.Fatalf("%d", w.Code)
			}
		}
	}
	for _, input := range []string{"{}", `{"content":"字"}`, `{"content":"` + strings.Repeat("字", 501) + `"}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(input))
		r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: 1, ClassID: 1}))
		w := httptest.NewRecorder()
		svc.SendMessage(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
}
func TestPrivateConversationAndDatabaseErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, nil)
	for _, failure := range []error{sql.ErrNoRows, errors.New("database down")} {
		mock.ExpectQuery("SELECT .* FROM tutor_conversations WHERE id=\\? AND user_id=\\? AND class_id=\\?").WithArgs(uint64(3), uint64(2), uint64(1)).WillReturnError(failure)
		r := httptest.NewRequest("GET", "/api/conversations/3", nil)
		r.SetPathValue("id", "3")
		r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: 2, ClassID: 1}))
		w := httptest.NewRecorder()
		svc.GetConversation(w, r)
		want := 404
		if failure != sql.ErrNoRows {
			want = 503
		}
		if w.Code != want {
			t.Fatal(w.Code)
		}
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestMessageOrdering(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := New(db, nil)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "role", "content", "citations", "skill", "status", "created", "tool_calls"}).AddRow(2, "assistant", "后", "[]", "数学解题引导", "complete", now, nil).AddRow(1, "user", "前", "[]", "", "complete", now, nil)
	mock.ExpectQuery("SELECT .* FROM tutor_messages .* ORDER BY id DESC LIMIT 6").WithArgs(uint64(1)).WillReturnRows(rows)
	items, err := svc.messages(context.Background(), 1, true)
	if err != nil || len(items) != 2 || items[0].Content != "前" {
		t.Fatal(items, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestConversationLock(t *testing.T) {
	svc := New(nil, nil)
	if !svc.acquire(1) || svc.acquire(1) || !svc.acquire(2) {
		t.Fatal("lock isolation")
	}
	svc.release(1)
	if !svc.acquire(1) {
		t.Fatal("release")
	}
}

type fakeKnowledge struct {
	fail bool
	t    *testing.T
}

func (f fakeKnowledge) SearchWithinMaterials(context.Context, uint64, string, string, int, []uint64) (knowledge.SearchResponse, error) {
	return knowledge.SearchResponse{Hits: []knowledge.Hit{}}, nil
}
func (f fakeKnowledge) StreamAnswer(_ context.Context, messages []knowledge.HistoryTurn, emit func(string) error) error {
	if len(messages) != 2 || strings.Count(messages[1].Content, "题目问题") != 1 {
		f.t.Fatal("current question duplicated or lost")
	}
	for _, text := range []string{"答案", "文字[", "1]"} {
		if err := emit(text); err != nil {
			return err
		}
	}
	if f.fail {
		return errors.New("upstream failed")
	}
	return nil
}
func TestDoneRequiresPersistenceAndPartialIsExplicit(t *testing.T) {
	for _, mode := range []string{"complete", "database-failure", "partial"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			s := New(db, fakeKnowledge{fail: mode == "partial", t: t})
			now := time.Now()
			mock.ExpectQuery("SELECT .* FROM tutor_conversations WHERE id=\\? AND user_id=\\? AND class_id=\\?").WithArgs(uint64(9), uint64(2), uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "user", "class", "agent", "title", "created", "updated"}).AddRow(9, 2, 1, 3, "新的提问", now, now))
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id,class_id,name,prompt,enabled FROM tutor_agents").WithArgs(uint64(3), uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "class", "name", "prompt", "enabled"}).AddRow(3, 1, "停用助手", "", false))
			mock.ExpectQuery("SELECT b.skill_id").WithArgs(uint64(3), uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery("SELECT b.material_id").WithArgs(uint64(3), uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery("SELECT s.id.*s.enabled=1").WithArgs(uint64(3), uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "class", "name", "trigger", "subject", "tools", "description", "source", "enabled"}))
			mock.ExpectCommit()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM tutor_messages").WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectExec("INSERT INTO tutor_messages").WithArgs(uint64(9), "题目问题").WillReturnResult(sqlmock.NewResult(100, 1))
			mock.ExpectExec("UPDATE tutor_conversations").WithArgs("题目问题", uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectQuery("SELECT id,role,content").WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "role", "content", "citations", "skill", "status", "created", "tool_calls"}).AddRow(100, "user", "题目问题", "[]", "", "complete", now, nil))
			status := "complete"
			if mode == "partial" {
				status = "incomplete"
			}
			save := mock.ExpectExec("INSERT INTO tutor_messages").WithArgs(uint64(9), "答案文字", "[]", "", status, nil)
			if mode == "database-failure" {
				save.WillReturnError(errors.New("write failed"))
			} else {
				save.WillReturnResult(sqlmock.NewResult(101, 1))
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{"content":"题目问题","agent_id":99,"class_id":99}`))
			r.SetPathValue("id", "9")
			r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: 2, ClassID: 1}))
			w := httptest.NewRecorder()
			s.SendMessage(w, r)
			body := w.Body.String()
			if !strings.HasPrefix(body, "event: meta") || strings.Contains(body, "[1]") {
				t.Fatal(body)
			}
			if mode == "complete" {
				if !strings.Contains(body, "event: done") || strings.Contains(body, "event: error") {
					t.Fatal(body)
				}
			} else {
				if !strings.Contains(body, "event: error") || strings.Contains(body, "event: done") {
					t.Fatal(body)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			if !s.acquire(9) {
				t.Fatal("generation lock leaked")
			}
		})
	}
}

func TestMCPReferences(t *testing.T) {
	mixed := strings.Replace(source, "knowledge_search", "knowledge_search,formula_hint@campus-tools；unit_convert@campus-tools|formula_hint@campus-tools", 1)
	skill, e := ParseSkill(mixed)
	if e != nil || len(skill.Tools) != 3 || skill.Source != mixed {
		t.Fatal(skill, e)
	}
	for _, ref := range []string{"formula_hint", "Formula_hint@campus-tools", "formula-hint@campus-tools", "x@Other", "x@a@b"} {
		if _, e := ParseSkill(strings.Replace(source, "knowledge_search", ref, 1)); e == nil {
			t.Fatal(ref)
		}
	}
}
