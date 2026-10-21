package knowledge

import (
	"campusclaw/internal/config"
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundKeywordCandidatesAreFilteredBeforeLimitAndHydration(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := New(db, config.Config{IndexVersion: "test"})
	mock.ExpectQuery("SELECT COALESCE").WithArgs(uint64(1), "test").WillReturnRows(sqlmock.NewRows([]string{"failed", "building"}).AddRow(0, 0))
	mock.ExpectQuery("SELECT id, MATCH.*material_id IN \\(\\?,\\?\\).*ORDER BY score DESC, id ASC LIMIT \\?").WithArgs("方程", uint64(1), "test", "方程", uint64(4), uint64(5), 20).WillReturnRows(sqlmock.NewRows([]string{"id", "score"}).AddRow(9, 1.0))
	mock.ExpectQuery("SELECT kc.id.*m.id IN \\(\\?,\\?\\)").WithArgs(uint64(9), uint64(1), uint64(1), uint64(1), "test", uint64(4), uint64(5)).WillReturnRows(sqlmock.NewRows([]string{"id", "material", "title", "original", "index", "text", "start", "end", "basis"}).AddRow(9, 4, "数学", "math.txt", 1, "方程", 0, 2, "original"))
	result, err := s.SearchWithinMaterials(context.Background(), 1, "方程", "keyword", 4, []uint64{4, 5})
	if err != nil || len(result.Hits) != 1 || result.Hits[0].MaterialID != 4 {
		t.Fatal(result, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestBoundVectorFilterAndThreshold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var query struct {
			Filter struct {
				Must []struct {
					Key   string         `json:"key"`
					Match map[string]any `json:"match"`
				} `json:"must"`
			} `json:"filter"`
			Threshold float64 `json:"score_threshold"`
		}
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Error(err)
		}
		if len(query.Filter.Must) != 3 || query.Filter.Must[0].Match["value"] != float64(1) || query.Filter.Must[2].Key != "material_id" || query.Threshold != 0.35 {
			t.Errorf("invalid tenant/material filter: %+v", query)
		}
		ids := query.Filter.Must[2].Match["any"].([]any)
		if ids[0] != float64(4) {
			t.Error("wrong material scope")
		}
		_, _ = w.Write([]byte(`{"result":[{"id":9,"score":0.9},{"id":10,"score":0.2}]}`))
	}))
	defer server.Close()
	v := newVectorStore(server.URL, 2)
	items, err := v.Search(context.Background(), 1, "test", []float32{1, 0}, 4, []uint64{4})
	if err != nil || len(items) != 1 || items[0].ID != 9 {
		t.Fatal(items, err)
	}
}
