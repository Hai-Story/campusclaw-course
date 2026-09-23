package materials

import (
	"bytes"
	"database/sql"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"campusclaw/internal/auth"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestStudentUploadIsRejectedBeforeWriting(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dir := t.TempDir()
	handler := New(database, dir, 1024)
	request := multipartRequest(t, "notes.md", []byte("# valid"))
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: 2, Role: "student", ClassID: 1}))
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("student upload changed directory: entries=%d err=%v", len(entries), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCrossClassAndMissingResponsesAreIdentical(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	handler := New(database, t.TempDir(), 1024)
	columns := []string{"id", "class_id", "name", "title", "original_name", "stored_name", "media_type", "size_bytes", "created_at", "content"}
	mock.ExpectQuery("SELECT m.id, m.class_id").WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(7, 2, "B 班", "B 班材料", "b.md", "server.md", "text/markdown", 8, "2026-09-23T00:00:00Z", "secret"))
	crossRequest := httptest.NewRequest(http.MethodGet, "/api/materials/7", nil)
	crossRequest.SetPathValue("id", "7")
	crossRequest = crossRequest.WithContext(auth.WithUser(crossRequest.Context(), auth.User{ID: 1, Role: "teacher", ClassID: 1}))
	crossRecorder := httptest.NewRecorder()
	handler.Detail(crossRecorder, crossRequest)

	mock.ExpectQuery("SELECT m.id, m.class_id").WithArgs(uint64(999)).WillReturnError(sql.ErrNoRows)
	missingRequest := httptest.NewRequest(http.MethodGet, "/api/materials/999", nil)
	missingRequest.SetPathValue("id", "999")
	missingRequest = missingRequest.WithContext(auth.WithUser(missingRequest.Context(), auth.User{ID: 1, Role: "teacher", ClassID: 1}))
	missingRecorder := httptest.NewRecorder()
	handler.Detail(missingRecorder, missingRequest)

	if crossRecorder.Code != http.StatusNotFound {
		t.Fatalf("cross-class request should return 404, got %d", crossRecorder.Code)
	}
	if missingRecorder.Code != http.StatusNotFound || crossRecorder.Body.String() != missingRecorder.Body.String() {
		t.Fatalf("responses differ: cross=%d %q missing=%d %q", crossRecorder.Code, crossRecorder.Body.String(), missingRecorder.Code, missingRecorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidUploadCommitsBothRowsAndFile(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dir := t.TempDir()
	handler := New(database, dir, 1024)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO materials")).
		WithArgs(uint64(1), uint64(10), "lesson", "lesson.md", sqlmock.AnyArg(), "text/markdown; charset=utf-8", 13).
		WillReturnResult(sqlmock.NewResult(44, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO knowledge_entries")).
		WithArgs(int64(44), uint64(1), "# lesson plan", "lesson.md").
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()
	request := multipartRequest(t, "lesson.md", []byte("# lesson plan"))
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: 10, Role: "teacher", ClassID: 1}))
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one stored file: entries=%d err=%v", len(entries), err)
	}
	if filepath.Ext(entries[0].Name()) != ".md" || entries[0].Name() == "lesson.md" {
		t.Fatalf("storage name was not server generated: %s", entries[0].Name())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseFailureRemovesStoredFile(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dir := t.TempDir()
	handler := New(database, dir, 1024)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO materials")).WillReturnError(errors.New("forced failure"))
	mock.ExpectRollback()
	request := multipartRequest(t, "lesson.txt", []byte("valid content"))
	request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: 10, Role: "teacher", ClassID: 1}))
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed upload left a file: entries=%d err=%v", len(entries), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func multipartRequest(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/materials", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
