package tutor

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/httpx"
	"campusclaw/internal/mcp"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-sql-driver/mysql"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func identity(w http.ResponseWriter, r *http.Request, teacher bool) (auth.User, bool) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, 401, "需要登录")
		return u, false
	}
	if teacher && u.Role != "teacher" {
		httpx.Error(w, 403, "仅教师可以修改配置")
		return u, false
	}
	return u, true
}
func objectID(r *http.Request) uint64 {
	id, _ := strconv.ParseUint(r.PathValue("id"), 10, 64)
	return id
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	raw, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(raw) || json.Unmarshal(raw, value) != nil {
		httpx.Error(w, 400, "请求必须是有效 UTF-8 JSON")
		return false
	}
	return true
}
func fail(w http.ResponseWriter, err error) {
	if isMissing(err) {
		httpx.Error(w, 404, "资源不存在")
		return
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		httpx.Error(w, 400, "本班名称已存在")
		return
	}
	httpx.Error(w, 503, "服务暂时不可用，请稍后重试")
}

// @openapi {"method":"get","path":"/api/skills","id":"get_api_skills","summary":"读取本班技能","input":"","output":"SkillList","success":200,"codes":[401,404,503],"teacher":false}
func (s *Service) ListSkills(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	items, err := readSkills(r.Context(), s.db, `SELECT `+skillColumns+` FROM tutor_skills WHERE class_id=? ORDER BY id`, u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"skills": items})
}

// @openapi {"method":"get","path":"/api/skills/{id}","id":"get_api_skills_by_id","summary":"读取本班技能","input":"","output":"Skill","success":200,"codes":[401,404,503],"teacher":false}
func (s *Service) GetSkill(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	items, err := readSkills(r.Context(), s.db, `SELECT `+skillColumns+` FROM tutor_skills WHERE id=? AND class_id=?`, objectID(r), u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	if len(items) == 0 {
		fail(w, sql.ErrNoRows)
		return
	}
	httpx.JSON(w, 200, items[0])
}

// @openapi {"method":"post","path":"/api/skills","id":"post_api_skills","summary":"从原文创建技能","input":"SkillInput","output":"Skill","success":201,"codes":[400,401,403,404,503],"teacher":true}
// @openapi {"method":"put","path":"/api/skills/{id}","id":"put_api_skills_by_id","summary":"改写技能（含启停）","input":"SkillInput","output":"Skill","success":200,"codes":[400,401,403,404,503],"teacher":true}
func (s *Service) WriteSkill(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Source string `json:"source"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, err := ParseSkill(input.Source)
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	item.ClassID = u.ClassID
	item.ID = objectID(r)
	saved, err := s.saveSkill(r.Context(), item, r.Method == "POST", u.ID)
	if err != nil {
		if errors.Is(err, mcp.ErrReference) {
			httpx.Error(w, 400, err.Error())
		} else {
			fail(w, err)
		}
		return
	}
	code := 200
	if r.Method == "POST" {
		code = 201
	}
	httpx.JSON(w, code, saved)
}

// @openapi {"method":"delete","path":"/api/skills/{id}","id":"delete_api_skills_by_id","summary":"删除技能与绑定","input":"","output":"","success":204,"codes":[400,401,403,404,503],"teacher":true}
func (s *Service) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, true)
	if !ok {
		return
	}
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM tutor_skills WHERE id=? AND class_id=?`, objectID(r), u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		fail(w, err)
		return
	}
	if n == 0 {
		fail(w, sql.ErrNoRows)
		return
	}
	w.WriteHeader(204)
}

// @openapi {"method":"get","path":"/api/agents","id":"get_api_agents","summary":"列出本班助手","input":"","output":"AgentList","success":200,"codes":[401,503],"teacher":false}
func (s *Service) ListAgents(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	query := `SELECT id FROM tutor_agents WHERE class_id=?`
	if u.Role != "teacher" {
		query += ` AND enabled=1`
	}
	query += ` ORDER BY id`
	ids, err := readIDs(r.Context(), s.db, query, u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	items := []Agent{}
	for _, id := range ids {
		item, e := s.agent(r.Context(), id, u.ClassID, u.Role != "teacher")
		if isMissing(e) {
			continue
		}
		if e != nil {
			fail(w, e)
			return
		}
		items = append(items, item)
	}
	httpx.JSON(w, 200, map[string]any{"agents": items})
}

// @openapi {"method":"get","path":"/api/agents/{id}","id":"get_api_agents_by_id","summary":"读取助手","input":"","output":"Agent","success":200,"codes":[401,404,503],"teacher":false}
func (s *Service) GetAgent(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	item, err := s.agent(r.Context(), objectID(r), u.ClassID, u.Role != "teacher")
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 200, item)
}

// @openapi {"method":"post","path":"/api/agents","id":"post_api_agents","summary":"创建本班助手","input":"AgentInput","output":"Agent","success":201,"codes":[400,401,403,404,503],"teacher":true}
// @openapi {"method":"put","path":"/api/agents/{id}","id":"put_api_agents_by_id","summary":"修改角色与绑定","input":"AgentInput","output":"Agent","success":200,"codes":[400,401,403,404,503],"teacher":true}
func (s *Service) WriteAgent(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Name        string   `json:"name"`
		Prompt      string   `json:"prompt"`
		Enabled     *bool    `json:"enabled"`
		SkillIDs    []uint64 `json:"skill_ids"`
		MaterialIDs []uint64 `json:"material_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Enabled == nil || input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || utf8.RuneCountInString(input.Prompt) > 2000 {
		httpx.Error(w, 400, "名称须为 1–64 字，提示词不超过 2000 字，enabled 须为布尔值")
		return
	}
	if input.SkillIDs == nil {
		input.SkillIDs = []uint64{}
	}
	if input.MaterialIDs == nil {
		input.MaterialIDs = []uint64{}
	}
	item, err := s.saveAgent(r.Context(), Agent{ID: objectID(r), ClassID: u.ClassID, Name: input.Name, Prompt: input.Prompt, Enabled: *input.Enabled, SkillIDs: input.SkillIDs, MaterialIDs: input.MaterialIDs}, r.Method == "POST")
	if err != nil {
		fail(w, err)
		return
	}
	code := 200
	if r.Method == "POST" {
		code = 201
	}
	httpx.JSON(w, code, item)
}

// @openapi {"method":"get","path":"/api/conversations","id":"get_api_conversations","summary":"本人会话列表","input":"","output":"ConversationList","success":200,"codes":[400,401,404,503]}
func (s *Service) ListConversations(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT `+conversationColumns+` FROM tutor_conversations WHERE user_id=? AND class_id=? ORDER BY updated_at DESC,id DESC`, u.ID, u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	items := []Conversation{}
	for rows.Next() {
		c, e := scanConversation(rows)
		if e != nil {
			fail(w, e)
			return
		}
		items = append(items, c)
	}
	if err = rows.Err(); err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"conversations": items})
}

// @openapi {"method":"post","path":"/api/conversations","id":"post_api_conversations","summary":"选定助手新建会话","input":"ConversationInput","output":"Conversation","success":201,"codes":[400,401,404,503]}
func (s *Service) CreateConversation(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	var input struct {
		AgentID uint64 `json:"agent_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.AgentID == 0 {
		httpx.Error(w, 400, "请选择本班已启用的智能体")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	var id uint64
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM tutor_agents WHERE id=? AND class_id=? AND enabled=1 FOR SHARE`, input.AgentID, u.ClassID).Scan(&id); err != nil {
		fail(w, err)
		return
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO tutor_conversations(user_id,class_id,agent_id,title) VALUES(?,?,?,'新的提问')`, u.ID, u.ClassID, id)
	if err != nil {
		fail(w, err)
		return
	}
	n, err := result.LastInsertId()
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, err)
		return
	}
	c, err := s.conversation(r.Context(), uint64(n), u)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 201, c)
}

// @openapi {"method":"get","path":"/api/conversations/{id}","id":"get_api_conversations_by_id","summary":"读取本人会话历史","input":"","output":"ConversationDetail","success":200,"codes":[400,401,404,503]}
func (s *Service) GetConversation(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, false)
	if !ok {
		return
	}
	c, err := s.conversation(r.Context(), objectID(r), u)
	if err != nil {
		fail(w, err)
		return
	}
	messages, err := s.messages(r.Context(), c.ID, false)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"conversation": c, "messages": messages})
}
