package mcp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

const columns = "id,class_id,name,base_url,token_text,status,revision,protocol_version,last_error"

type scanner interface{ Scan(...any) error }

func scanServer(r scanner) (Server, error) {
	var v Server
	e := r.Scan(&v.ID, &v.ClassID, &v.Name, &v.BaseURL, &v.Token, &v.Status, &v.Revision, &v.Protocol, &v.LastError)
	v.HasToken = v.Token != ""
	return v, e
}
func (s *Service) server(ctx context.Context, id, class uint64) (Server, error) {
	return scanServer(s.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM mcp_servers WHERE id=? AND class_id=?`, id, class))
}
func listTools(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, id uint64) ([]Tool, error) {
	rows, e := q.QueryContext(ctx, `SELECT name,description,schema_json,descriptor_hash FROM mcp_tools WHERE server_id=? ORDER BY name`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Tool{}
	for rows.Next() {
		var t Tool
		var schema []byte
		if e = rows.Scan(&t.Name, &t.Description, &schema, &t.Hash); e != nil {
			return nil, e
		}
		t.InputSchema = schema
		out = append(out, t)
	}
	return out, rows.Err()
}

var errConnectionChanged = errors.New("连接配置已改变，请重新连接")

func (s *Service) connect(ctx context.Context, id, class uint64) (Server, []Tool, error) {
	v, tools, err := s.connectAttempt(ctx, id, class)
	if err != nil && !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, errConnectionChanged) && v.ID != 0 {
		save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		// Fail closed even when discovery succeeded but committing its snapshot did not.
		_, _ = s.DB.ExecContext(save, `UPDATE mcp_servers SET status='disconnected',revision=revision+1,last_error='连接未完成，请重新连接' WHERE id=? AND class_id=? AND revision=?`, id, class, v.Revision)
	}
	return v, tools, err
}
func (s *Service) connectAttempt(ctx context.Context, id, class uint64) (Server, []Tool, error) {
	lock := s.lock(id)
	lock.Lock()
	v, e := s.server(ctx, id, class)
	if e == nil {
		_, e = s.DB.ExecContext(ctx, `UPDATE mcp_servers SET status='disconnected' WHERE id=? AND class_id=? AND revision=?`, id, class, v.Revision)
	}
	lock.Unlock()
	if e != nil {
		return v, nil, e
	}
	stage, cancel := context.WithTimeout(ctx, s.Config.StageTimeout)
	defer cancel()
	tools, discoverErr := s.discover(stage, v)
	lock.Lock()
	defer lock.Unlock()
	current, e := s.server(ctx, id, class)
	if e != nil {
		return current, nil, e
	}
	if current.Revision != v.Revision {
		return current, nil, errConnectionChanged
	}
	if discoverErr != nil {
		_, e = s.DB.ExecContext(ctx, `UPDATE mcp_servers SET status='disconnected',last_error=?,revision=revision+1,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=? AND revision=?`, discoverErr.Error(), id, class, v.Revision)
		if e != nil {
			return current, nil, e
		}
		current.Status = "disconnected"
		current.LastError = discoverErr.Error()
		return current, nil, discoverErr
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return v, nil, e
	}
	defer tx.Rollback()
	// Keep the last discovered version in changes even after disconnect removed the live cache.
	rows, e := tx.QueryContext(ctx, `SELECT tool_name,new_json FROM mcp_tool_changes WHERE server_id=? ORDER BY id`, id)
	if e != nil {
		return v, nil, e
	}
	previous := map[string]json.RawMessage{}
	for rows.Next() {
		var n string
		var raw []byte
		if e = rows.Scan(&n, &raw); e != nil {
			rows.Close()
			return v, nil, e
		}
		previous[n] = raw
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, nil, e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id=?`, id); e != nil {
		return v, nil, e
	}
	found := map[string]bool{}
	for _, t := range tools {
		found[t.Name] = true
		var old Tool
		json.Unmarshal(previous[t.Name], &old)
		raw, _ := json.Marshal(t)
		if old.Hash != t.Hash {
			var oldJSON any
			if len(previous[t.Name]) > 0 {
				oldJSON = string(previous[t.Name])
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_tool_changes(class_id,server_id,server_name,tool_name,old_json,new_json) VALUES(?,?,?,?,?,?)`, class, id, v.Name, t.Name, oldJSON, string(raw)); e != nil {
				return v, nil, e
			}
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_tools(server_id,name,description,schema_json,descriptor_hash) VALUES(?,?,?,?,?)`, id, t.Name, t.Description, string(t.InputSchema), t.Hash); e != nil {
			return v, nil, e
		}
	}
	for n, raw := range previous {
		if found[n] || len(raw) == 0 {
			continue
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO mcp_tool_changes(class_id,server_id,server_name,tool_name,old_json,new_json) VALUES(?,?,?,?,?,NULL)`, class, id, v.Name, n, string(raw)); e != nil {
			return v, nil, e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_servers SET status='connected',revision=revision+1,protocol_version=?,last_error='',updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=?`, ProtocolVersion, id, class); e != nil {
		return v, nil, e
	}
	if e = tx.Commit(); e != nil {
		return v, nil, e
	}
	v.Status = "connected"
	v.Protocol = ProtocolVersion
	v.Revision++
	v.LastError = ""
	return v, tools, nil
}
func (s *Service) disconnect(ctx context.Context, id, class uint64) (Server, error) {
	lock := s.lock(id)
	lock.Lock()
	defer lock.Unlock()
	v, e := s.server(ctx, id, class)
	if e != nil {
		return v, e
	}
	for _, cancel := range lock.cancels {
		cancel()
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id=?`, id); e != nil {
		return v, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE mcp_servers SET status='disconnected',revision=revision+1,last_error='',updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=?`, id, class); e != nil {
		return v, e
	}
	if e = tx.Commit(); e != nil {
		return v, e
	}
	v.Status = "disconnected"
	v.Revision++
	v.LastError = ""
	return v, nil
}

// Approve validates all references in the same transaction as the skill write.
func (s *Service) Approve(ctx context.Context, tx *sql.Tx, skill, class, teacher uint64, refs []string) error {
	type approval struct {
		server     uint64
		name, hash string
	}
	items := []approval{}
	for _, ref := range refs {
		if ref == "knowledge_search" {
			continue
		}
		name, server, ok := ParseRef(ref)
		if !ok {
			return ErrReference
		}
		v, e := scanServer(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM mcp_servers WHERE class_id=? AND name=? FOR SHARE`, class, server))
		if errors.Is(e, sql.ErrNoRows) {
			return ErrReference
		}
		if e != nil {
			return e
		}
		if v.Status != "connected" || !s.Config.toolAllowed(v.BaseURL, name) {
			return ErrReference
		}
		var hash string
		if e = tx.QueryRowContext(ctx, `SELECT descriptor_hash FROM mcp_tools WHERE server_id=? AND name=? FOR SHARE`, v.ID, name).Scan(&hash); errors.Is(e, sql.ErrNoRows) {
			return ErrReference
		} else if e != nil {
			return e
		}
		items = append(items, approval{v.ID, name, hash})
	}
	if _, e := tx.ExecContext(ctx, `DELETE FROM mcp_tool_approvals WHERE skill_id=?`, skill); e != nil {
		return e
	}
	for _, v := range items {
		if _, e := tx.ExecContext(ctx, `INSERT INTO mcp_tool_approvals(skill_id,server_id,tool_name,descriptor_hash,teacher_id) VALUES(?,?,?,?,?)`, skill, v.server, v.name, v.hash, teacher); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) Candidates(ctx context.Context, class, skill uint64, refs []string) ([]Candidate, error) {
	out := []Candidate{}
	for _, ref := range refs {
		if ref == "knowledge_search" {
			continue
		}
		name, server, valid := ParseRef(ref)
		c := Candidate{Ref: ref, SkillID: skill}
		if !valid {
			out = append(out, c)
			continue
		}
		v, e := scanServer(s.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM mcp_servers WHERE class_id=? AND name=?`, class, server))
		if errors.Is(e, sql.ErrNoRows) {
			out = append(out, c)
			continue
		}
		if e != nil {
			return nil, e
		}
		c.Server = v
		if v.Status == "connected" && s.Config.toolAllowed(v.BaseURL, name) {
			var approved string
			var schema []byte
			e = s.DB.QueryRowContext(ctx, `SELECT t.name,t.description,t.schema_json,t.descriptor_hash,COALESCE(a.descriptor_hash,'') FROM mcp_tools t LEFT JOIN mcp_tool_approvals a ON a.server_id=t.server_id AND a.tool_name=t.name AND a.skill_id=? WHERE t.server_id=? AND t.name=?`, skill, v.ID, name).Scan(&c.Tool.Name, &c.Tool.Description, &schema, &c.Tool.Hash, &approved)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return nil, e
			}
			c.Tool.InputSchema = schema
			c.Approved = e == nil && approved == c.Tool.Hash
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *Service) reserve(ctx context.Context, scope Scope) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `INSERT IGNORE INTO mcp_conversation_usage(conversation_id,attempts) SELECT id,0 FROM tutor_conversations WHERE id=? AND class_id=? AND user_id=?`, scope.ConversationID, scope.ClassID, scope.UserID); e != nil {
		return e
	}
	r, e := tx.ExecContext(ctx, `UPDATE mcp_conversation_usage SET attempts=attempts+1 WHERE conversation_id=? AND attempts<?`, scope.ConversationID, s.Config.MaxConversation)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrQuota
	}
	return tx.Commit()
}
func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("random source unavailable")
	}
	return hex.EncodeToString(b)
}
func (s *Service) auditStart(ctx context.Context, scope Scope, c Candidate, info CallInfo) error {
	if c.Server.ID == 0 {
		return nil
	}
	_, e := s.DB.ExecContext(ctx, `INSERT INTO mcp_call_audits(class_id,server_id,server_name,tool_ref,user_id,conversation_id,question_id,call_id,round_no,status) VALUES(?,?,?,?,?,?,?,?,?,'pending')`, scope.ClassID, c.Server.ID, c.Server.Name, info.Ref, scope.UserID, scope.ConversationID, scope.QuestionID, info.CallID, info.Round)
	return e
}
func (s *Service) auditFinish(ctx context.Context, c Candidate, info CallInfo, duration time.Duration) error {
	if c.Server.ID == 0 {
		return nil
	}
	_, e := s.DB.ExecContext(ctx, `UPDATE mcp_call_audits SET status=?,summary=?,duration_ms=? WHERE call_id=? AND class_id=?`, info.Status, truncate(info.Text, 200), max(int64(0), duration.Milliseconds()), info.CallID, c.Server.ClassID)
	return e
}
func (s *Service) Execute(ctx context.Context, scope Scope, c Candidate, raw string, round int, quotaAllowed bool) (CallInfo, error) {
	info := CallInfo{Ref: c.Ref, Status: "denied", Text: "工具未连接、未批准或不在本次候选范围内", Round: round, CallID: newID()}
	start := time.Now()
	if quotaAllowed {
		e := s.reserve(ctx, scope)
		if e != nil && !errors.Is(e, ErrQuota) {
			return info, e
		}
		quotaAllowed = e == nil
	}
	if !quotaAllowed {
		info.Text = "工具调用预算已耗尽"
	}
	if e := s.auditStart(ctx, scope, c, info); e != nil {
		return info, e
	}
	var callErr error
	if quotaAllowed && c.Approved {
		args, e := validateArgs(c.Tool, raw)
		if e != nil {
			info.Text = "工具参数不符合 inputSchema，未发出请求"
		} else {
			lock := s.lock(c.Server.ID)
			lock.Lock()
			// Re-read authorization at dispatch time, rather than trusting the request snapshot.
			var bound int
			err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tutor_agent_skills b JOIN tutor_conversations v ON v.agent_id=b.agent_id JOIN tutor_skills k ON k.id=b.skill_id WHERE v.id=? AND v.class_id=? AND v.user_id=? AND b.skill_id=? AND k.class_id=? AND k.enabled=1`, scope.ConversationID, scope.ClassID, scope.UserID, c.SkillID, scope.ClassID).Scan(&bound)
			current := []Candidate{}
			if err == nil && bound > 0 {
				current, err = s.Candidates(ctx, scope.ClassID, c.SkillID, []string{c.Ref})
			}
			if err != nil {
				lock.Unlock()
				info.Status = "error"
				info.Text = "工具授权复核失败，未发出请求"
				save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
				defer cancel()
				_ = s.auditFinish(save, c, info, time.Since(start))
				return info, err
			}
			if len(current) == 1 && current[0].Approved && current[0].Server.ID == c.Server.ID && current[0].Tool.Hash == c.Tool.Hash {
				c = current[0]
				callCtx, cancel := context.WithTimeout(ctx, s.Config.CallTimeout)
				lock.cancels[info.CallID] = cancel
				// Start the HTTP dispatch while holding the same lock used by disconnect. The request
				// may already be in flight when disconnect cancels it; no new dispatch starts after it.
				started := make(chan struct{})
				result := make(chan struct {
					text string
					e    error
				}, 1)
				var once sync.Once
				notify := func() { once.Do(func() { close(started) }) }
				callCtx = context.WithValue(callCtx, dispatchKey{}, notify)
				go func() {
					defer notify()
					text, e := s.remoteCall(callCtx, c, args)
					result <- struct {
						text string
						e    error
					}{text, e}
				}()
				<-started
				lock.Unlock()
				got := <-result
				cancel()
				lock.Lock()
				delete(lock.cancels, info.CallID)
				lock.Unlock()
				info.Status = "ok"
				info.Text = got.text
				if got.e != nil {
					info.Status = "error"
					info.Text = got.e.Error()
				}
			} else {
				lock.Unlock()
			}
		}
	}
	// Short independent context preserves the audit after client cancellation.
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if e := s.auditFinish(save, c, info, time.Since(start)); e != nil {
		info.Status = "error"
		info.Text = "审计收尾失败，远端结果未知"
		callErr = e
	}
	return info, callErr
}
