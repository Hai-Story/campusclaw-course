package mcp

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/httpx"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-sql-driver/mysql"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func teacher(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.Error(w, 401, "需要登录")
		return u, false
	}
	if u.Role != "teacher" {
		httpx.Error(w, 403, "仅教师可管理 MCP")
		return u, false
	}
	return u, true
}
func objectID(r *http.Request) uint64 {
	id, _ := strconv.ParseUint(r.PathValue("id"), 10, 64)
	return id
}
func failure(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		httpx.Error(w, 404, "资源不存在")
		return
	}
	var my *mysql.MySQLError
	if errors.As(e, &my) && my.Number == 1062 {
		httpx.Error(w, 400, "本班名称已存在")
		return
	}
	httpx.Error(w, 503, "MCP 服务暂时不可用，请稍后重试")
}

// @openapi {"method":"get","path":"/api/mcp/servers","id":"get_mcp_servers","summary":"教师列出本班 MCP 服务器","input":"","output":"MCPServerList","success":200,"codes":[401,403,503],"teacher":true}
func (s *Service) ListServers(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	rows, e := s.DB.QueryContext(r.Context(), `SELECT `+columns+` FROM mcp_servers WHERE class_id=? ORDER BY id`, u.ClassID)
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		v, e := scanServer(rows)
		if e != nil {
			failure(w, e)
			return
		}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	httpx.JSON(w, 200, map[string]any{"servers": out})
}

// @openapi {"method":"get","path":"/api/mcp/servers/{id}","id":"get_mcp_server","summary":"教师读取本班 MCP 服务器","input":"","output":"MCPServer","success":200,"codes":[401,403,404,503],"teacher":true}
func (s *Service) GetServer(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	v, e := s.server(r.Context(), objectID(r), u.ClassID)
	if e != nil {
		failure(w, e)
		return
	}
	httpx.JSON(w, 200, v)
}

// @openapi {"method":"post","path":"/api/mcp/servers","id":"post_mcp_server","summary":"登记本班 MCP 服务器","input":"MCPServerInput","output":"MCPServer","success":201,"codes":[400,401,403,503],"teacher":true}
// @openapi {"method":"put","path":"/api/mcp/servers/{id}","id":"put_mcp_server","summary":"修改服务器并使连接失效","input":"MCPServerInput","output":"MCPServer","success":200,"codes":[400,401,403,404,503],"teacher":true}
func (s *Service) WriteServer(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	var input struct {
		Name    string  `json:"name"`
		BaseURL string  `json:"base_url"`
		Token   *string `json:"token"`
	}
	id := objectID(r)
	lock := s.lock(id)
	lock.Lock()
	defer lock.Unlock()
	var old Server
	var e error
	if r.Method == "PUT" {
		old, e = s.server(r.Context(), id, u.ClassID)
		if e != nil {
			failure(w, e)
			return
		}
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	if e != nil || !utf8.Valid(raw) || json.Unmarshal(raw, &input) != nil {
		httpx.Error(w, 400, "请求须为有效 JSON")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	base, e := normalizeURL(input.BaseURL)
	if !ServerName.MatchString(input.Name) || e != nil || !s.Config.allowedURL(base) {
		httpx.Error(w, 400, "名称须为小写引用名，Base URL 须在部署白名单内")
		return
	}
	if input.Token != nil && (len(*input.Token) > 4096 || strings.ContainsAny(*input.Token, "\r\n")) {
		httpx.Error(w, 400, "令牌格式无效")
		return
	}
	token := old.Token
	if input.Token != nil {
		token = *input.Token
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	if r.Method == "POST" {
		res, err := tx.ExecContext(r.Context(), `INSERT INTO mcp_servers(class_id,name,base_url,token_text) VALUES(?,?,?,?)`, u.ClassID, input.Name, base, token)
		if err != nil {
			failure(w, err)
			return
		}
		n, err := res.LastInsertId()
		if err != nil {
			failure(w, err)
			return
		}
		id = uint64(n)
	} else {
		for _, cancel := range lock.cancels {
			cancel()
		}
		if _, e = tx.ExecContext(r.Context(), `UPDATE mcp_servers SET name=?,base_url=?,token_text=?,status='disconnected',revision=revision+1,last_error='',updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=?`, input.Name, base, token, id, u.ClassID); e != nil {
			failure(w, e)
			return
		}
		if _, e = tx.ExecContext(r.Context(), `DELETE FROM mcp_tools WHERE server_id=?`, id); e != nil {
			failure(w, e)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		failure(w, e)
		return
	}
	v, e := s.server(r.Context(), id, u.ClassID)
	if e != nil {
		failure(w, e)
		return
	}
	code := 200
	if r.Method == "POST" {
		code = 201
	}
	httpx.JSON(w, code, v)
}

// @openapi {"method":"delete","path":"/api/mcp/servers/{id}","id":"delete_mcp_server","summary":"删除服务器并保留历史审计","input":"","output":"","success":204,"codes":[401,403,404,503],"teacher":true}
func (s *Service) DeleteServer(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	id := objectID(r)
	lock := s.lock(id)
	lock.Lock()
	defer lock.Unlock()
	if _, e := s.server(r.Context(), id, u.ClassID); e != nil {
		failure(w, e)
		return
	}
	for _, cancel := range lock.cancels {
		cancel()
	}
	if _, e := s.DB.ExecContext(r.Context(), `DELETE FROM mcp_servers WHERE id=? AND class_id=?`, id, u.ClassID); e != nil {
		failure(w, e)
		return
	}
	w.WriteHeader(204)
}

// @openapi {"method":"post","path":"/api/mcp/servers/{id}/connect","id":"connect_mcp_server","summary":"握手并原子替换发现清单","input":"","output":"MCPConnection","success":200,"codes":[401,403,404,503],"teacher":true}
func (s *Service) Connect(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	v, tools, e := s.connect(r.Context(), objectID(r), u.ClassID)
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			failure(w, e)
		} else {
			httpx.Error(w, 503, "连接未完成：请核对地址、令牌、协议或工具声明，并刷新服务器状态")
		}
		return
	}
	httpx.JSON(w, 200, map[string]any{"server": v, "tools": tools})
}

// @openapi {"method":"post","path":"/api/mcp/servers/{id}/disconnect","id":"disconnect_mcp_server","summary":"断开并清空可用工具缓存","input":"","output":"MCPServer","success":200,"codes":[401,403,404,503],"teacher":true}
func (s *Service) Disconnect(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	v, e := s.disconnect(r.Context(), objectID(r), u.ClassID)
	if e != nil {
		failure(w, e)
		return
	}
	httpx.JSON(w, 200, v)
}

// @openapi {"method":"get","path":"/api/mcp/servers/{id}/tools","id":"get_mcp_tools","summary":"查看完整工具声明","input":"","output":"MCPToolList","success":200,"codes":[401,403,404,503],"teacher":true}
func (s *Service) Tools(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	v, e := s.server(r.Context(), objectID(r), u.ClassID)
	if e != nil {
		failure(w, e)
		return
	}
	tools := []Tool{}
	if v.Status == "connected" {
		tools, e = listTools(r.Context(), s.DB, v.ID)
		if e != nil {
			failure(w, e)
			return
		}
	}
	httpx.JSON(w, 200, map[string]any{"tools": tools})
}
func page(r *http.Request) (int, uint64, bool) {
	limit := 20
	before := uint64(0)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			return 0, 0, false
		}
		limit = n
	}
	if raw := r.URL.Query().Get("before"); raw != "" {
		n, e := strconv.ParseUint(raw, 10, 64)
		if e != nil || n < 1 {
			return 0, 0, false
		}
		before = n
	}
	return limit, before, true
}

// @openapi {"method":"get","path":"/api/mcp/servers/{id}/changes","id":"get_mcp_changes","summary":"查看工具声明变更与初次发现记录","input":"","output":"MCPChangeList","success":200,"codes":[400,401,403,404,503],"teacher":true,"query":[{"name":"limit","in":"query","schema":{"type":"integer","minimum":1,"maximum":100}},{"name":"before","in":"query","schema":{"type":"integer"}}]}
func (s *Service) Changes(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	id := objectID(r)
	if _, e := s.server(r.Context(), id, u.ClassID); e != nil {
		failure(w, e)
		return
	}
	limit, before, ok := page(r)
	if !ok {
		httpx.Error(w, 400, "分页参数无效")
		return
	}
	query := `SELECT id,tool_name,old_json,new_json,created_at FROM mcp_tool_changes WHERE class_id=? AND server_id=?`
	args := []any{u.ClassID, id}
	if before > 0 {
		query += ` AND id<?`
		args = append(args, before)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, e := s.DB.QueryContext(r.Context(), query, args...)
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	out := []any{}
	var next uint64
	for rows.Next() {
		var id uint64
		var name string
		var old, new []byte
		var at time.Time
		if e = rows.Scan(&id, &name, &old, &new, &at); e != nil {
			failure(w, e)
			return
		}
		out = append(out, map[string]any{"id": id, "tool_name": name, "old": json.RawMessage(old), "new": json.RawMessage(new), "created_at": at})
		next = id
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	httpx.JSON(w, 200, map[string]any{"changes": out, "next_before": next})
}

// @openapi {"method":"get","path":"/api/mcp/audits","id":"get_mcp_audits","summary":"教师查看本班工具调用审计","input":"","output":"MCPAuditList","success":200,"codes":[400,401,403,503],"teacher":true,"query":[{"name":"limit","in":"query","schema":{"type":"integer","minimum":1,"maximum":100}},{"name":"before","in":"query","schema":{"type":"string"}},{"name":"status","in":"query","schema":{"type":"string","enum":["ok","error","denied"]}},{"name":"server_id","in":"query","schema":{"type":"integer"}}]}
func (s *Service) Audits(w http.ResponseWriter, r *http.Request) {
	u, ok := teacher(w, r)
	if !ok {
		return
	}
	limit, _, ok := pageWithoutCursor(r)
	if !ok {
		httpx.Error(w, 400, "分页参数无效")
		return
	}
	query := `SELECT id,server_id,server_name,tool_ref,user_id,conversation_id,question_id,call_id,round_no,status,summary,duration_ms,created_at FROM mcp_call_audits WHERE class_id=? AND status<>'pending'`
	args := []any{u.ClassID}
	if raw := r.URL.Query().Get("before"); raw != "" {
		parts := strings.Split(raw, "|")
		if len(parts) != 2 {
			httpx.Error(w, 400, "游标无效")
			return
		}
		at, e := time.Parse(time.RFC3339Nano, parts[0])
		id, e2 := strconv.ParseUint(parts[1], 10, 64)
		if e != nil || e2 != nil || id == 0 {
			httpx.Error(w, 400, "游标无效")
			return
		}
		query += ` AND (created_at<? OR (created_at=? AND id<?))`
		args = append(args, at, at, id)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if status != "ok" && status != "error" && status != "denied" {
			httpx.Error(w, 400, "状态无效")
			return
		}
		query += ` AND status=?`
		args = append(args, status)
	}
	if raw := r.URL.Query().Get("server_id"); raw != "" {
		id, e := strconv.ParseUint(raw, 10, 64)
		if e != nil || id == 0 {
			httpx.Error(w, 400, "服务器编号无效")
			return
		}
		query += ` AND server_id=?`
		args = append(args, id)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	args = append(args, limit)
	rows, e := s.DB.QueryContext(r.Context(), query, args...)
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	out := []any{}
	next := ""
	for rows.Next() {
		var id, user, conversation, question uint64
		var server sql.NullInt64
		var name, ref, call, status, text string
		var round int
		var duration int64
		var at time.Time
		if e = rows.Scan(&id, &server, &name, &ref, &user, &conversation, &question, &call, &round, &status, &text, &duration, &at); e != nil {
			failure(w, e)
			return
		}
		var sid any
		if server.Valid {
			sid = server.Int64
		}
		out = append(out, map[string]any{"id": id, "server_id": sid, "server_name": name, "ref": ref, "user_id": user, "conversation_id": conversation, "question_id": question, "call_id": call, "round": round, "status": status, "text": text, "duration_ms": duration, "created_at": at})
		next = at.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatUint(id, 10)
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	httpx.JSON(w, 200, map[string]any{"audits": out, "next_before": next})
}
func pageWithoutCursor(r *http.Request) (int, uint64, bool) {
	copy := r.Clone(r.Context())
	u := *r.URL
	q := u.Query()
	q.Del("before")
	u.RawQuery = q.Encode()
	copy.URL = &u
	return page(copy)
}
