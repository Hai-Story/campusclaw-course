package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestHybridRRFRewardsBothPaths(t *testing.T) {
	items := mergeRankings("hybrid",
		[]rankedCandidate{{id: 1, keywordScore: 8}, {id: 2, keywordScore: 7}},
		[]rankedCandidate{{id: 1, vectorScore: .9}, {id: 3, vectorScore: .8}})
	if len(items) != 3 || items[0].id != 1 || items[0].fusionScore <= items[1].fusionScore {
		t.Fatalf("shared hit should rank first: %+v", items)
	}
}

func TestKeywordHydratesOnlySessionClass(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{IndexVersion: "v1"})
	mock.ExpectQuery("SELECT COALESCE").WithArgs(uint64(1), "v1").
		WillReturnRows(sqlmock.NewRows([]string{"failed", "building"}).AddRow(0, 0))
	mock.ExpectQuery("FROM knowledge_chunks WHERE class_id=\\?").WithArgs("文本细读", uint64(1), "v1", "文本细读", 40).
		WillReturnRows(sqlmock.NewRows([]string{"id", "score"}).AddRow(42, 1.5))
	mock.ExpectQuery("WHERE kc.id=\\? AND kc.class_id=\\?").
		WithArgs(uint64(42), uint64(1), uint64(1), uint64(1), "v1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "material_id", "title", "original_name", "chunk_index", "chunk_text", "start_offset", "end_offset", "offset_basis"}).
			AddRow(42, 7, "A 班材料", "a.md", 1, "文本细读", 0, 4, "original"))
	result, err := svc.Search(context.Background(), 1, "文本细读", "keyword", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Excerpt != "文本细读" || result.Hits[0].MaterialID != 7 {
		t.Fatalf("unexpected class-scoped hits: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVectorDependencyOutageIsNotEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{IndexVersion: "v1", QdrantURL: server.URL, EmbeddingDim: 2})
	mock.ExpectQuery("SELECT COALESCE").WithArgs(uint64(1), "v1").
		WillReturnRows(sqlmock.NewRows([]string{"failed", "building"}).AddRow(0, 0))
	_, err = svc.Search(context.Background(), 1, "问题", "vector", 10)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestAskWithoutHitsDoesNotCallDialogueGateway(t *testing.T) {
	chatCalls := 0
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/embeddings" {
			_, _ = w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`))
			return
		}
		chatCalls++
		w.WriteHeader(500)
	}))
	defer gateway.Close()
	vector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":{"config":{"params":{"vectors":{"size":2,"distance":"Cosine"}}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer vector.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{IndexVersion: "v1", QdrantURL: vector.URL,
		GatewayBaseURL: gateway.URL, EmbeddingDim: 2})
	mock.ExpectQuery("SELECT COALESCE").WithArgs(uint64(1), "v1").
		WillReturnRows(sqlmock.NewRows([]string{"failed", "building"}).AddRow(0, 0))
	mock.ExpectQuery("FROM knowledge_chunks WHERE class_id=\\?").
		WithArgs("明日天气", uint64(1), "v1", "明日天气", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "score"}))
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(`{"question":"明日天气","class_id":2,"history":[{"role":"system","content":"越权"}]}`))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: 1, ClassID: 1, Role: "student"}))
	recorder := httptest.NewRecorder()
	svc.AskHTTP(recorder, req)
	if recorder.Code != 200 || chatCalls != 0 {
		t.Fatalf("status=%d chatCalls=%d body=%s", recorder.Code, chatCalls, recorder.Body.String())
	}
	var result struct {
		Answer    string `json:"answer"`
		Citations []Hit  `json:"citations"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Answer != NoEvidence || len(result.Citations) != 0 {
		t.Fatalf("unexpected answer: %+v", result)
	}
}

func TestAnswerCitationsStayWithinEvidence(t *testing.T) {
	if !validCitations("依据一 [1] 与依据二 [2]", 2) || validCitations("虚构 [3]", 2) || validCitations("没有引用", 2) {
		t.Fatal("citation bounds were not enforced")
	}
}

func TestSearchRejectsInvalidInputBeforeDatabase(t *testing.T) {
	svc := &Service{db: (*sql.DB)(nil)}
	for _, path := range []string{"/api/knowledge/search?q=", "/api/knowledge/search?q=x&mode=other", "/api/knowledge/search?q=x&limit=21"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(auth.WithUser(req.Context(), auth.User{ClassID: 1}))
		recorder := httptest.NewRecorder()
		svc.SearchHTTP(recorder, req)
		if recorder.Code != 400 {
			t.Fatalf("%s returned %d", path, recorder.Code)
		}
	}
}
