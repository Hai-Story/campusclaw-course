package tutor

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/mcp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
)

type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const skillColumns = "id,class_id,name,trigger_text,subject_text,tools_json,description,source_text,enabled"

func readSkills(ctx context.Context, q querier, query string, args ...any) ([]Skill, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Skill{}
	for rows.Next() {
		var item Skill
		var tools []byte
		if err = rows.Scan(&item.ID, &item.ClassID, &item.Name, &item.Trigger, &item.Subject, &tools, &item.Description, &item.Source, &item.Enabled); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(tools, &item.Tools); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func readIDs(ctx context.Context, q querier, query string, args ...any) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func readAgent(ctx context.Context, q querier, id, classID uint64, onlyEnabled bool) (Agent, error) {
	var a Agent
	query := `SELECT id,class_id,name,prompt,enabled FROM tutor_agents WHERE id=? AND class_id=?`
	if onlyEnabled {
		query += ` AND enabled=1`
	}
	if err := q.QueryRowContext(ctx, query, id, classID).Scan(&a.ID, &a.ClassID, &a.Name, &a.Prompt, &a.Enabled); err != nil {
		return a, err
	}
	var err error
	a.SkillIDs, err = readIDs(ctx, q, `SELECT b.skill_id FROM tutor_agent_skills b JOIN tutor_skills s ON s.id=b.skill_id WHERE b.agent_id=? AND s.class_id=? ORDER BY b.skill_id`, id, classID)
	if err != nil {
		return a, err
	}
	a.MaterialIDs, err = readIDs(ctx, q, `SELECT b.material_id FROM tutor_agent_materials b JOIN materials m ON m.id=b.material_id WHERE b.agent_id=? AND m.class_id=? ORDER BY b.material_id`, id, classID)
	return a, err
}

func (s *Service) saveAgent(ctx context.Context, a Agent, create bool) (Agent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	if !create {
		var id uint64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM tutor_agents WHERE id=? AND class_id=? FOR UPDATE`, a.ID, a.ClassID).Scan(&id); err != nil {
			return a, err
		}
	}
	// Lock resource rows before replacing either binding set. Sorted IDs avoid lock-order inversions.
	for _, binding := range []struct {
		table string
		ids   []uint64
	}{{"tutor_skills", a.SkillIDs}, {"materials", a.MaterialIDs}} {
		ids := append([]uint64{}, binding.ids...)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			var found uint64
			if err = tx.QueryRowContext(ctx, `SELECT id FROM `+binding.table+` WHERE id=? AND class_id=? FOR SHARE`, id, a.ClassID).Scan(&found); err != nil {
				return a, err
			}
		}
	}
	if create {
		result, e := tx.ExecContext(ctx, `INSERT INTO tutor_agents(class_id,name,prompt,enabled) VALUES(?,?,?,?)`, a.ClassID, a.Name, a.Prompt, a.Enabled)
		if e != nil {
			return a, e
		}
		id, e := result.LastInsertId()
		if e != nil {
			return a, e
		}
		a.ID = uint64(id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE tutor_agents SET name=?,prompt=?,enabled=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND class_id=?`, a.Name, a.Prompt, a.Enabled, a.ID, a.ClassID)
		if err != nil {
			return a, err
		}
	}
	for _, binding := range []struct {
		table, col string
		ids        []uint64
	}{{"tutor_agent_skills", "skill_id", a.SkillIDs}, {"tutor_agent_materials", "material_id", a.MaterialIDs}} {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+binding.table+` WHERE agent_id=?`, a.ID); err != nil {
			return a, err
		}
		seen := map[uint64]bool{}
		for _, id := range binding.ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, err = tx.ExecContext(ctx, `INSERT INTO `+binding.table+`(agent_id,`+binding.col+`) VALUES(?,?)`, a.ID, id); err != nil {
				return a, err
			}
		}
	}
	saved, err := readAgent(ctx, tx, a.ID, a.ClassID, false)
	if err != nil {
		return a, err
	}
	if err = tx.Commit(); err != nil {
		return a, err
	}
	return saved, nil
}

// Read role prompt and both binding sets from one database snapshot.
func (s *Service) agent(ctx context.Context, id, classID uint64, onlyEnabled bool) (Agent, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Agent{}, err
	}
	defer tx.Rollback()
	a, err := readAgent(ctx, tx, id, classID, onlyEnabled)
	if err != nil {
		return a, err
	}
	return a, tx.Commit()
}

const conversationColumns = "id,user_id,class_id,agent_id,title,created_at,updated_at"

type scanner interface{ Scan(...any) error }

func scanConversation(row scanner) (Conversation, error) {
	var c Conversation
	err := row.Scan(&c.ID, &c.UserID, &c.ClassID, &c.AgentID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (s *Service) conversation(ctx context.Context, id uint64, u auth.User) (Conversation, error) {
	return scanConversation(s.db.QueryRowContext(ctx, `SELECT `+conversationColumns+` FROM tutor_conversations WHERE id=? AND user_id=? AND class_id=?`, id, u.ID, u.ClassID))
}
func (s *Service) messages(ctx context.Context, id uint64, recent bool) ([]Message, error) {
	query := `SELECT id,role,content,citations_json,skill_name,status,created_at,tool_calls_json FROM tutor_messages WHERE conversation_id=?`
	if recent {
		query += ` AND role IN ('user','assistant') AND TRIM(content)<>'' ORDER BY id DESC LIMIT 6`
	} else {
		query += ` ORDER BY id ASC`
	}
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []Message{}
	for rows.Next() {
		var m Message
		var citations, calls []byte
		if err = rows.Scan(&m.ID, &m.Role, &m.Content, &citations, &m.SkillName, &m.Status, &m.CreatedAt, &calls); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(citations, &m.Citations); err != nil {
			return nil, err
		}
		if len(calls) > 0 {
			if err = json.Unmarshal(calls, &m.ToolCalls); err != nil {
				return nil, err
			}
		}
		messages = append(messages, m)
	}
	if recent {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	return messages, rows.Err()
}
func (s *Service) saveQuestion(ctx context.Context, c Conversation, question string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tutor_messages WHERE conversation_id=?`, c.ID).Scan(&count); err != nil {
		return err
	}
	title := c.Title
	if count == 0 {
		title = truncate(question, 24)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tutor_messages(conversation_id,role,content,citations_json) VALUES(?,'user',?,'[]')`, c.ID, question); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tutor_conversations SET title=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, title, c.ID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) saveAnswer(ctx context.Context, id uint64, content, skill, status string, citations any, calls ...[]mcp.CallInfo) (uint64, error) {
	data, err := json.Marshal(citations)
	if err != nil {
		return 0, err
	}
	var toolData any
	if len(calls) > 0 && len(calls[0]) > 0 {
		raw, e := json.Marshal(calls[0])
		if e != nil {
			return 0, e
		}
		toolData = string(raw)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO tutor_messages(conversation_id,role,content,citations_json,skill_name,status,tool_calls_json) VALUES(?,'assistant',?,?,?,?,?)`, id, content, string(data), skill, status, toolData)
	if err != nil {
		return 0, err
	}
	n, err := res.LastInsertId()
	return uint64(n), err
}

func (s *Service) config(ctx context.Context, id, classID uint64) (Agent, []Skill, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Agent{}, nil, err
	}
	defer tx.Rollback()
	a, err := readAgent(ctx, tx, id, classID, false)
	if err != nil {
		return a, nil, err
	}
	skills, err := readSkills(ctx, tx, `SELECT s.id,s.class_id,s.name,s.trigger_text,s.subject_text,s.tools_json,s.description,s.source_text,s.enabled FROM tutor_skills s JOIN tutor_agent_skills b ON b.skill_id=s.id WHERE b.agent_id=? AND s.class_id=? AND s.enabled=1 ORDER BY s.id`, id, classID)
	if err != nil {
		return a, nil, err
	}
	if err = tx.Commit(); err != nil {
		return a, nil, err
	}
	return a, skills, nil
}

var errNotFound = sql.ErrNoRows

func isMissing(err error) bool { return errors.Is(err, errNotFound) }

func (s *Service) saveSkill(ctx context.Context, item Skill, create bool, teacher uint64) (Skill, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	if !create {
		var id uint64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM tutor_skills WHERE id=? AND class_id=? FOR UPDATE`, item.ID, item.ClassID).Scan(&id); err != nil {
			return item, err
		}
	}
	tools, _ := json.Marshal(item.Tools)
	if create {
		res, e := tx.ExecContext(ctx, `INSERT INTO tutor_skills(class_id,name,trigger_text,subject_text,tools_json,description,source_text,enabled) VALUES(?,?,?,?,?,?,?,?)`, item.ClassID, item.Name, item.Trigger, item.Subject, string(tools), item.Description, item.Source, item.Enabled)
		if e != nil {
			return item, e
		}
		id, e := res.LastInsertId()
		if e != nil {
			return item, e
		}
		item.ID = uint64(id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE tutor_skills SET name=?,trigger_text=?,subject_text=?,tools_json=?,description=?,source_text=?,enabled=? WHERE id=? AND class_id=?`, item.Name, item.Trigger, item.Subject, string(tools), item.Description, item.Source, item.Enabled, item.ID, item.ClassID)
		if err != nil {
			return item, err
		}
	}
	if s.MCP != nil {
		if err = s.MCP.Approve(ctx, tx, item.ID, item.ClassID, teacher, item.Tools); err != nil {
			return item, err
		}
	} else {
		for _, ref := range item.Tools {
			if ref != "knowledge_search" {
				return item, mcp.ErrReference
			}
		}
	}
	return item, tx.Commit()
}
