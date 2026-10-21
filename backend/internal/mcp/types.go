// Package mcp contains class-scoped registry, transport and host enforcement.
package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ProtocolVersion = "2025-06-18"

var ToolName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
var ServerName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
var ErrReference = errors.New("工具引用不可用，请核对本班服务器、连接、工具清单及批准版本")
var ErrQuota = errors.New("工具调用配额已用尽")

type Config struct {
	AllowedURLs                          []string
	AllowedTools                         map[string][]string
	MaxRounds, MaxCalls, MaxConversation int
	CallTimeout, StageTimeout            time.Duration
}

func LoadConfig() (Config, error) {
	c := Config{AllowedURLs: []string{"http://mcp-demo:8090"}, AllowedTools: map[string][]string{"http://mcp-demo:8090": {"formula_hint", "unit_convert"}}, MaxRounds: 3, MaxCalls: 6, MaxConversation: 24, CallTimeout: 5 * time.Second, StageTimeout: 30 * time.Second}
	if v := os.Getenv("MCP_ALLOWED_BASE_URLS"); v != "" {
		c.AllowedURLs = strings.Split(v, ",")
	}
	if v := os.Getenv("MCP_ALLOWED_TOOLS"); v != "" {
		var tools map[string][]string
		if json.Unmarshal([]byte(v), &tools) != nil || tools == nil {
			return c, errors.New("MCP_ALLOWED_TOOLS must be a URL to tool-name-array JSON object")
		}
		c.AllowedTools = tools
	}
	for _, v := range c.AllowedURLs {
		if _, e := normalizeURL(v); e != nil {
			return c, errors.New("invalid MCP_ALLOWED_BASE_URLS")
		}
	}
	for base, names := range c.AllowedTools {
		if !c.allowedURL(base) {
			return c, errors.New("MCP_ALLOWED_TOOLS contains an unapproved URL")
		}
		for _, name := range names {
			if !ToolName.MatchString(name) {
				return c, errors.New("invalid MCP tool allowlist name")
			}
		}
	}
	for _, v := range []struct {
		key string
		dst *int
		max int
	}{{"MCP_MAX_ROUNDS", &c.MaxRounds, 10}, {"MCP_MAX_CALLS_PER_MESSAGE", &c.MaxCalls, 32}, {"MCP_MAX_CALLS_PER_CONVERSATION", &c.MaxConversation, 1000}} {
		if raw := os.Getenv(v.key); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || n > v.max {
				return c, errors.New("invalid " + v.key)
			}
			*v.dst = n
		}
	}
	return c, nil
}
func (c Config) allowedURL(raw string) bool {
	u, e := normalizeURL(raw)
	if e != nil {
		return false
	}
	for _, v := range c.AllowedURLs {
		n, e := normalizeURL(strings.TrimSpace(v))
		if e == nil && n == u {
			return true
		}
	}
	return false
}
func (c Config) toolAllowed(base, name string) bool {
	base, _ = normalizeURL(base)
	for k, names := range c.AllowedTools {
		k, _ = normalizeURL(k)
		if k == base {
			for _, n := range names {
				if n == name {
					return true
				}
			}
		}
	}
	return false
}

type Server struct {
	ID        uint64 `json:"id"`
	ClassID   uint64 `json:"-"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Token     string `json:"-"`
	HasToken  bool   `json:"has_token"`
	Status    string `json:"status"`
	Revision  uint64 `json:"revision"`
	Protocol  string `json:"protocol_version"`
	LastError string `json:"last_error"`
}
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Hash        string          `json:"hash"`
}
type Candidate struct {
	Ref      string
	Server   Server
	Tool     Tool
	SkillID  uint64
	Approved bool
}
type CallInfo struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
	Text   string `json:"text"`
	Round  int    `json:"round"`
	CallID string `json:"call_id"`
}
type Scope struct{ ClassID, UserID, ConversationID, QuestionID uint64 }
type serverLock struct {
	sync.Mutex
	cancels map[string]context.CancelFunc
}
type Service struct {
	DB     *sql.DB
	Config Config
	mu     sync.Mutex
	locks  map[uint64]*serverLock
}

func New(db *sql.DB, c Config) *Service {
	return &Service{DB: db, Config: c, locks: map[uint64]*serverLock{}}
}
func (s *Service) lock(id uint64) *serverLock {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[id] == nil {
		s.locks[id] = &serverLock{cancels: map[string]context.CancelFunc{}}
	}
	return s.locks[id]
}
func ParseRef(ref string) (string, string, bool) {
	a, b, ok := strings.Cut(ref, "@")
	return a, b, ok && ToolName.MatchString(a) && ServerName.MatchString(b)
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
