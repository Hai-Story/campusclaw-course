package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"campusclaw/internal/httpx"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	key := strings.ToLower(request.Username) + "|" + clientIP(r)
	now := time.Now()
	if a.limiter.Locked(key, now) {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(request.Password))
		credentialError(w)
		return
	}

	var user User
	var passwordHash string
	err := a.db.QueryRowContext(r.Context(), `SELECT u.id, u.username, u.password_hash, u.role, u.class_id, c.name
		FROM users u JOIN classes c ON c.id=u.class_id WHERE u.username=?`, request.Username).
		Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &user.ClassID, &user.ClassName)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(request.Password))
		a.limiter.Fail(key, now)
		credentialError(w)
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务暂时不可用")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)) != nil {
		a.limiter.Fail(key, now)
		credentialError(w)
		return
	}
	if a.limiter.Locked(key, now) {
		credentialError(w)
		return
	}

	token, err := a.createSession(r.Context(), user)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务暂时不可用")
		return
	}
	a.limiter.Reset(key)
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user, "token": token, "token_type": "Bearer", "expires_in": int(a.ttl.Seconds())})
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := a.bearerClaims(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "需要登录")
		return
	}
	if err := a.destroySession(r.Context(), claims.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "服务暂时不可用")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "需要登录")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user})
}

func credentialError(w http.ResponseWriter) {
	httpx.Error(w, http.StatusUnauthorized, "用户名或密码错误")
}

func clientIP(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func generateDummyHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte("campusclaw-dummy-password"), bcrypt.DefaultCost)
}
