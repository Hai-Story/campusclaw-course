package tutor

import (
	"campusclaw/internal/knowledge"
	"campusclaw/internal/mcp"
	"context"
	"database/sql"
	"sync"
	"time"
)

type Agent struct {
	ID          uint64   `json:"id"`
	ClassID     uint64   `json:"class_id"`
	Name        string   `json:"name"`
	Prompt      string   `json:"prompt"`
	Enabled     bool     `json:"enabled"`
	SkillIDs    []uint64 `json:"skill_ids"`
	MaterialIDs []uint64 `json:"material_ids"`
}
type Skill struct {
	ID          uint64   `json:"id"`
	ClassID     uint64   `json:"class_id"`
	Name        string   `json:"name"`
	Trigger     string   `json:"trigger"`
	Subject     string   `json:"subject"`
	Tools       []string `json:"tools"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	Enabled     bool     `json:"enabled"`
}
type Conversation struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	ClassID   uint64    `json:"class_id"`
	AgentID   uint64    `json:"agent_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Message struct {
	ID        uint64          `json:"id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Citations []knowledge.Hit `json:"citations"`
	SkillName string          `json:"skill_name"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	ToolCalls []mcp.CallInfo  `json:"tool_calls,omitempty"`
}
type Knowledge interface {
	SearchWithinMaterials(context.Context, uint64, string, string, int, []uint64) (knowledge.SearchResponse, error)
	StreamAnswer(context.Context, []knowledge.HistoryTurn, func(string) error) error
}
type Service struct {
	db        *sql.DB
	MCP       *mcp.Service
	knowledge Knowledge
	mu        sync.Mutex
	active    map[uint64]bool
}

func New(database *sql.DB, k Knowledge) *Service {
	return &Service{db: database, knowledge: k, active: make(map[uint64]bool)}
}
func (s *Service) acquire(id uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[id] {
		return false
	}
	s.active[id] = true
	return true
}
func (s *Service) release(id uint64) { s.mu.Lock(); delete(s.active, id); s.mu.Unlock() }
