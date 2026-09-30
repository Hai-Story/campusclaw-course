package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func testAuth(db *sql.DB) *Auth {
	return &Auth{
		db: db, secret: []byte(strings.Repeat("s", 32)), ttl: time.Hour,
		limiter: NewLimiter(5, time.Minute),
	}
}

func TestLoginIssuesBearerWithoutCookie(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	a := testAuth(database)
	hash, err := bcrypt.GenerateFromPassword([]byte("example-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM users u JOIN classes c ON c.id=u.class_id WHERE u.username=?")).
		WithArgs("teacher_a").WillReturnRows(sqlmock.NewRows([]string{
		"id", "username", "password_hash", "role", "class_id", "name",
	}).AddRow(1, "teacher_a", string(hash), "teacher", 7, "A 班"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO sessions")).
		WithArgs(sqlmock.AnyArg(), uint64(1), "teacher", uint64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"teacher_a","password":"example-password"}`))
	w := httptest.NewRecorder()
	a.Login(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", w.Code, w.Body.String())
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("login issued a cookie")
	}
	var result struct {
		Token     string `json:"token"`
		TokenType string `json:"token_type"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	claims, ok := a.verifyBearer(result.Token)
	if !ok || claims.Subject != "1" || result.TokenType != "Bearer" || result.ExpiresIn != 3600 {
		t.Fatalf("invalid login token metadata: claims=%+v type=%q expires=%d", claims, result.TokenType, result.ExpiresIn)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireRejectsCookieOnlyAndInvalidBearer(t *testing.T) {
	a := testAuth(nil)
	valid := a.signBearer("opaque-token", User{ID: 1})
	for _, test := range []struct {
		name, authorization string
	}{
		{"cookie only", ""},
		{"wrong scheme", "Basic " + valid},
		{"tampered", valid + "tampered"},
		{"expired", (&Auth{secret: a.secret, ttl: -time.Minute}).signBearer("expired", User{ID: 1})},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			r.AddCookie(&http.Cookie{Name: "campus_session", Value: "legacy-session"})
			if test.authorization != "" {
				r.Header.Set("Authorization", test.authorization)
			}
			w := httptest.NewRecorder()
			a.Require(http.HandlerFunc(a.Me)).ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestBearerReadsCurrentUserAndLogoutRevokesSession(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	a := testAuth(database)
	rawToken := "opaque-token"
	token := a.signBearer(rawToken, User{ID: 1})
	query := regexp.QuoteMeta("FROM sessions s JOIN users u ON u.id=s.user_id JOIN classes c ON c.id=u.class_id")
	columns := []string{"id", "username", "role", "class_id", "name"}
	mock.ExpectQuery(query).WithArgs(hashToken(rawToken)).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(1, "teacher_a", "student", 8, "B 班"))
	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.Header.Set("Authorization", "Bearer "+token)
	me.AddCookie(&http.Cookie{Name: "campus_session", Value: "legacy-session"})
	w := httptest.NewRecorder()
	a.Require(http.HandlerFunc(a.Me)).ServeHTTP(w, me)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"class_id":8`) {
		t.Fatalf("bearer did not return current database user: %d %s", w.Code, w.Body.String())
	}

	mock.ExpectQuery(query).WithArgs(hashToken(rawToken)).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(1, "teacher_a", "student", 8, "B 班"))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM sessions WHERE token_hash=?")).
		WithArgs(hashToken(rawToken)).WillReturnResult(sqlmock.NewResult(0, 1))
	logout := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logout.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	a.Require(http.HandlerFunc(a.Logout)).ServeHTTP(w, logout)
	if w.Code != http.StatusNoContent || len(w.Result().Cookies()) != 0 {
		t.Fatalf("logout returned %d or set a cookie", w.Code)
	}

	mock.ExpectQuery(query).WithArgs(hashToken(rawToken)).WillReturnError(sql.ErrNoRows)
	w = httptest.NewRecorder()
	a.Require(http.HandlerFunc(a.Me)).ServeHTTP(w, me)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token returned %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
