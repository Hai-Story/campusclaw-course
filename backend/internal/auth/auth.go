package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"campusclaw/internal/httpx"
)

const CookieName = "campus_session"

type User struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	ClassID   uint64 `json:"class_id"`
	ClassName string `json:"class_name"`
}

type contextKey struct{}

type Auth struct {
	db        *sql.DB
	secret    []byte
	ttl       time.Duration
	secure    bool
	limiter   *Limiter
	dummyHash []byte
}

func New(database *sql.DB, secret string, ttl time.Duration, maxFailures int, lockFor time.Duration) (*Auth, error) {
	dummyHash, err := generateDummyHash()
	if err != nil {
		return nil, err
	}
	return &Auth{
		db: database, secret: []byte(secret), ttl: ttl,
		limiter: NewLimiter(maxFailures, lockFor), dummyHash: dummyHash,
	}, nil
}

func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.Authenticate(r.Context(), r)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "需要登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, user)))
	})
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(contextKey{}).(User)
	return user, ok
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

func (a *Auth) Authenticate(ctx context.Context, r *http.Request) (User, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return User{}, err
	}
	rawToken, ok := a.verifyCookie(cookie.Value)
	if !ok {
		return User{}, errors.New("invalid session signature")
	}
	var user User
	err = a.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.role, u.class_id, c.name
		FROM sessions s JOIN users u ON u.id=s.user_id JOIN classes c ON c.id=u.class_id
		WHERE s.token_hash=? AND s.expires_at>UTC_TIMESTAMP(6)`, hashToken(rawToken)).
		Scan(&user.ID, &user.Username, &user.Role, &user.ClassID, &user.ClassName)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (a *Auth) rotateSession(ctx context.Context, oldCookieValue string, user User) (string, error) {
	rawToken, err := randomToken()
	if err != nil {
		return "", err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if oldToken, ok := a.verifyCookie(oldCookieValue); ok {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, hashToken(oldToken)); err != nil {
			return "", err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, role, class_id, expires_at)
		VALUES (?, ?, ?, ?, ?)`, hashToken(rawToken), user.ID, user.Role, user.ClassID, time.Now().UTC().Add(a.ttl))
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return a.signCookie(rawToken), nil
}

func (a *Auth) destroySession(ctx context.Context, cookieValue string) error {
	rawToken, ok := a.verifyCookie(cookieValue)
	if !ok {
		return nil
	}
	_, err := a.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, hashToken(rawToken))
	return err
}

func (a *Auth) setCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: value, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: a.secure, MaxAge: int(a.ttl.Seconds()),
	})
}

func (a *Auth) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: a.secure, MaxAge: -1,
	})
}

func (a *Auth) signCookie(rawToken string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(rawToken))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return rawToken + "." + signature
}

func (a *Auth) verifyCookie(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return parts[0], hmac.Equal([]byte(expected), []byte(parts[1]))
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
