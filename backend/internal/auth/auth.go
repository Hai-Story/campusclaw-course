package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"campusclaw/internal/httpx"
)

const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

type tokenClaims struct {
	Subject string `json:"sub"`
	ID      string `json:"jti"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
}

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
	claims, ok := a.bearerClaims(r)
	if !ok {
		return User{}, errors.New("invalid bearer token")
	}
	var user User
	err := a.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.role, u.class_id, c.name
		FROM sessions s JOIN users u ON u.id=s.user_id JOIN classes c ON c.id=u.class_id
		WHERE s.token_hash=? AND s.expires_at>UTC_TIMESTAMP(6)`, hashToken(claims.ID)).
		Scan(&user.ID, &user.Username, &user.Role, &user.ClassID, &user.ClassName)
	if err != nil {
		return User{}, err
	}
	if claims.Subject != strconv.FormatUint(user.ID, 10) {
		return User{}, errors.New("token subject mismatch")
	}
	return user, nil
}

func (a *Auth) createSession(ctx context.Context, user User) (string, error) {
	rawToken, err := randomToken()
	if err != nil {
		return "", err
	}
	_, err = a.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, role, class_id, expires_at)
		VALUES (?, ?, ?, ?, ?)`, hashToken(rawToken), user.ID, user.Role, user.ClassID, time.Now().UTC().Add(a.ttl))
	if err != nil {
		return "", err
	}
	return a.signBearer(rawToken, user), nil
}

func (a *Auth) destroySession(ctx context.Context, rawToken string) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, hashToken(rawToken))
	return err
}

func (a *Auth) signBearer(rawToken string, user User) string {
	now := time.Now().UTC()
	claims, _ := json.Marshal(tokenClaims{
		Subject: strconv.FormatUint(user.ID, 10), ID: rawToken,
		Issued: now.Unix(), Expires: now.Add(a.ttl).Unix(),
	})
	data := jwtHeader + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(data))
	return data + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *Auth) verifyBearer(value string) (tokenClaims, bool) {
	parts := strings.Split(value, ".")
	if len(value) > 4096 || len(parts) != 3 || parts[0] != jwtHeader {
		return tokenClaims{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return tokenClaims{}, false
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return tokenClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, false
	}
	var claims tokenClaims
	if json.Unmarshal(payload, &claims) != nil || claims.ID == "" || claims.Subject == "" ||
		claims.Issued > time.Now().Unix() || claims.Expires <= time.Now().Unix() || claims.Expires <= claims.Issued {
		return tokenClaims{}, false
	}
	return claims, true
}

func (a *Auth) bearerClaims(r *http.Request) (tokenClaims, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return tokenClaims{}, false
	}
	fields := strings.Fields(values[0])
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return tokenClaims{}, false
	}
	return a.verifyBearer(fields[1])
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
