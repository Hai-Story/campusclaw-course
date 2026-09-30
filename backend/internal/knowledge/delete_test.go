package knowledge

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"github.com/DATA-DOG/go-sqlmock"
)

func deletionRequest(id, role string) *http.Request {
	r := httptest.NewRequest(http.MethodDelete, "/api/materials/"+id, nil)
	r.SetPathValue("id", id)
	return r.WithContext(auth.WithUser(r.Context(), auth.User{ID: 1, ClassID: 1, Role: role}))
}

func TestStudentCannotDeleteMaterial(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{})
	w := httptest.NewRecorder()
	svc.DeleteMaterialHTTP(w, deletionRequest("7", "student"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCrossClassAndUnknownDeletionHaveSameResponse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{})
	responses := make([]string, 0, 2)
	for _, id := range []string{"7", "999"} {
		materialID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT e.id,m.stored_name,m.source").WithArgs(materialID, uint64(1)).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()
		w := httptest.NewRecorder()
		svc.DeleteMaterialHTTP(w, deletionRequest(id, "teacher"))
		if w.Code != http.StatusNotFound {
			t.Fatalf("id=%s status=%d", id, w.Code)
		}
		responses = append(responses, w.Body.String())
	}
	if responses[0] != responses[1] {
		t.Fatalf("different 404 responses: %q %q", responses[0], responses[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUploadRemovesFileAndVectors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stored.md")
	if err := os.WriteFile(path, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	vectorDeletes := 0
	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/campusclaw_chunks/points/delete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		vectorDeletes++
		w.WriteHeader(http.StatusOK)
	}))
	defer qdrant.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{UploadDir: dir, QdrantURL: qdrant.URL})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT e.id,m.stored_name,m.source").WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_name", "source"}).AddRow(11, "stored.md", "upload"))
	mock.ExpectQuery("SELECT status FROM knowledge_index_jobs").WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("ready"))
	mock.ExpectExec("DELETE FROM materials").WithArgs(uint64(7), uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	w := httptest.NewRecorder()
	svc.DeleteMaterialHTTP(w, deletionRequest("7", "teacher"))
	if w.Code != http.StatusNoContent || vectorDeletes != 1 {
		t.Fatalf("status=%d vectors=%d body=%s", w.Code, vectorDeletes, w.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stored file remains: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSeedWritesTombstone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer qdrant.Close()
	svc := New(db, config.Config{UploadDir: t.TempDir(), QdrantURL: qdrant.URL})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT e.id,m.stored_name,m.source").WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_name", "source"}).AddRow(11, "seed-class-a.md", "seed"))
	mock.ExpectQuery("SELECT status FROM knowledge_index_jobs").WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("ready"))
	mock.ExpectExec("INSERT IGNORE INTO deleted_seed_materials").WithArgs("seed-class-a.md").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM materials").WithArgs(uint64(7), uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	w := httptest.NewRecorder()
	svc.DeleteMaterialHTTP(w, deletionRequest("7", "teacher"))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWaitsForProcessingIndexJob(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stored.md")
	if err := os.WriteFile(path, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{UploadDir: dir})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT e.id,m.stored_name,m.source").WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_name", "source"}).AddRow(11, "stored.md", "upload"))
	mock.ExpectQuery("SELECT status FROM knowledge_index_jobs").WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("processing"))
	mock.ExpectRollback()
	w := httptest.NewRecorder()
	svc.DeleteMaterialHTTP(w, deletionRequest("7", "teacher"))
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file changed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVectorFailureRestoresMaterialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stored.md")
	if err := os.WriteFile(path, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer qdrant.Close()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(db, config.Config{UploadDir: dir, QdrantURL: qdrant.URL})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT e.id,m.stored_name,m.source").WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "stored_name", "source"}).AddRow(11, "stored.md", "upload"))
	mock.ExpectQuery("SELECT status FROM knowledge_index_jobs").WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("ready"))
	mock.ExpectRollback()
	w := httptest.NewRecorder()
	svc.DeleteMaterialHTTP(w, deletionRequest("7", "teacher"))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file was not restored: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
