package tutor

import (
	"campusclaw/internal/httpx"
	"net/http"
	"strings"
	"unicode/utf8"
)

type PromptTemplate struct {
	ID      uint64 `json:"id"`
	ClassID uint64 `json:"class_id"`
	Name    string `json:"name"`
	Prompt  string `json:"prompt"`
	Builtin bool   `json:"builtin"`
}

const templateColumns = "id,class_id,name,prompt,builtin"

// @openapi {"method":"get","path":"/api/prompt-templates","id":"get_api_prompt_templates","summary":"教师读取本班提示词模板","input":"","output":"PromptTemplateList","success":200,"codes":[401,403,503],"teacher":true}
func (s *Service) ListTemplates(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, true)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT `+templateColumns+` FROM tutor_prompt_templates WHERE class_id=? ORDER BY builtin DESC,id`, u.ClassID)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	result := []PromptTemplate{}
	for rows.Next() {
		var item PromptTemplate
		if err = rows.Scan(&item.ID, &item.ClassID, &item.Name, &item.Prompt, &item.Builtin); err != nil {
			fail(w, err)
			return
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"templates": result})
}

// @openapi {"method":"post","path":"/api/prompt-templates","id":"post_api_prompt_templates","summary":"创建本班自定义模板","input":"PromptTemplateInput","output":"PromptTemplate","success":201,"codes":[400,401,403,503],"teacher":true}
// @openapi {"method":"put","path":"/api/prompt-templates/{id}","id":"put_api_prompt_templates_by_id","summary":"修改本班自定义模板","input":"PromptTemplateInput","output":"PromptTemplate","success":200,"codes":[400,401,403,404,503],"teacher":true}
func (s *Service) WriteTemplate(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Name   string `json:"name"`
		Prompt string `json:"prompt"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || input.Prompt == "" || utf8.RuneCountInString(input.Prompt) > 2000 {
		httpx.Error(w, 400, "模板名称须为 1–64 字，提示词须为 1–2000 字")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	item := PromptTemplate{ID: objectID(r), ClassID: u.ClassID, Name: input.Name, Prompt: input.Prompt}
	code := 200
	if r.Method == "POST" {
		result, e := tx.ExecContext(r.Context(), `INSERT INTO tutor_prompt_templates(class_id,name,prompt,builtin) VALUES(?,?,?,0)`, u.ClassID, item.Name, item.Prompt)
		if e != nil {
			fail(w, e)
			return
		}
		id, e := result.LastInsertId()
		if e != nil {
			fail(w, e)
			return
		}
		item.ID = uint64(id)
		code = 201
	} else {
		var builtin bool
		if err = tx.QueryRowContext(r.Context(), `SELECT builtin FROM tutor_prompt_templates WHERE id=? AND class_id=? FOR UPDATE`, item.ID, u.ClassID).Scan(&builtin); err != nil {
			fail(w, err)
			return
		}
		if builtin {
			httpx.Error(w, 403, "内置模板只读，请复制为自定义模板")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE tutor_prompt_templates SET name=?,prompt=? WHERE id=? AND class_id=?`, item.Name, item.Prompt, item.ID, u.ClassID); err != nil {
			fail(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, code, item)
}

// @openapi {"method":"delete","path":"/api/prompt-templates/{id}","id":"delete_api_prompt_templates_by_id","summary":"删除本班自定义模板","input":"","output":"","success":204,"codes":[401,403,404,503],"teacher":true}
func (s *Service) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	u, ok := identity(w, r, true)
	if !ok {
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	var builtin bool
	if err = tx.QueryRowContext(r.Context(), `SELECT builtin FROM tutor_prompt_templates WHERE id=? AND class_id=? FOR UPDATE`, objectID(r), u.ClassID).Scan(&builtin); err != nil {
		fail(w, err)
		return
	}
	if builtin {
		httpx.Error(w, 403, "内置模板只读，请复制为自定义模板")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM tutor_prompt_templates WHERE id=? AND class_id=?`, objectID(r), u.ClassID); err != nil {
		fail(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}
