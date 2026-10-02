package knowledge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"campusclaw/internal/auth"
	"campusclaw/internal/httpx"
)

var citationPattern = regexp.MustCompile(`\[(\d+)\]`)

func validQuestion(q string) bool {
	return q != "" && utf8.ValidString(q) && utf8.RuneCountInString(q) <= 200
}

func (s *Service) SearchHTTP(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validQuestion(q) {
		httpx.Error(w, http.StatusBadRequest, "查询须为 1 至 200 个字符")
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "hybrid"
	}
	if mode != "keyword" && mode != "vector" && mode != "hybrid" {
		httpx.Error(w, http.StatusBadRequest, "检索模式无效")
		return
	}
	limit := 10
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 20 {
			httpx.Error(w, http.StatusBadRequest, "limit 须为 1 至 20")
			return
		}
		limit = parsed
	}
	result, err := s.Search(r.Context(), user.ClassID, q, mode, limit)
	if err != nil {
		writeSearchError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func writeSearchError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnavailable) {
		httpx.Error(w, http.StatusServiceUnavailable, "检索服务暂时不可用，请稍后重试")
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "知识库检索失败")
}

func (s *Service) AskHTTP(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var input struct {
		Question string        `json:"question"`
		History  []HistoryTurn `json:"history"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, "问答请求格式错误")
		return
	}
	question := strings.TrimSpace(input.Question)
	if !validQuestion(question) {
		httpx.Error(w, http.StatusBadRequest, "问题须为 1 至 200 个字符")
		return
	}
	if len(input.History) > 6 {
		httpx.Error(w, http.StatusBadRequest, "历史对话过长")
		return
	}
	history := make([]HistoryTurn, 0, len(input.History))
	for _, turn := range input.History {
		if turn.Role != "user" && turn.Role != "assistant" {
			continue
		}
		if utf8.RuneCountInString(turn.Content) > 1000 {
			httpx.Error(w, http.StatusBadRequest, "历史消息过长")
			return
		}
		history = append(history, turn)
	}
	result, err := s.Search(r.Context(), user.ClassID, question, "hybrid", 4)
	if err != nil {
		writeSearchError(w, err)
		return
	}
	if len(result.Hits) == 0 {
		httpx.JSON(w, http.StatusOK, map[string]any{"answer": NoEvidence, "citations": []Hit{}, "index_state": result.IndexState})
		return
	}
	answer, err := s.gateway.Answer(r.Context(), question, result.Hits, history)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "问答服务暂时不可用")
		return
	}
	answer = addSourceReferences(answer, len(result.Hits))
	if !validCitations(answer, len(result.Hits)) {
		httpx.Error(w, http.StatusBadGateway, "回答引用无效")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"answer": answer, "citations": result.Hits, "index_state": result.IndexState})
}

// Some compatible chat models omit inline citation markers despite receiving
// numbered evidence. In that case, label the retrieved sources explicitly.
func addSourceReferences(answer string, count int) string {
	if citationPattern.MatchString(answer) || count == 0 {
		return answer
	}
	var sources strings.Builder
	sources.WriteString(answer)
	sources.WriteString("\n\n可核对的检索来源：")
	for i := 1; i <= count; i++ {
		if i > 1 {
			sources.WriteByte(' ')
		}
		sources.WriteString("[")
		sources.WriteString(strconv.Itoa(i))
		sources.WriteString("]")
	}
	return sources.String()
}

func validCitations(answer string, count int) bool {
	refs := citationPattern.FindAllStringSubmatch(answer, -1)
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		n, err := strconv.Atoi(ref[1])
		if err != nil || n < 1 || n > count {
			return false
		}
	}
	return true
}

func (s *Service) ReindexHTTP(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user.Role != "teacher" {
		httpx.Error(w, http.StatusForbidden, "仅教师可以重建索引")
		return
	}
	materialID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || materialID == 0 {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	strategy := Strategy{Mode: "auto"}
	if r.ContentLength != 0 {
		var input struct {
			Strategy Strategy `json:"strategy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpx.Error(w, http.StatusBadRequest, "策略格式错误")
			return
		}
		strategy = input.Strategy
	}
	strategy, err = ValidateStrategy(strategy)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "切分策略参数无效")
		return
	}
	var entryID uint64
	var content string
	err = s.db.QueryRowContext(r.Context(), `SELECT e.id,e.content FROM knowledge_entries e
		JOIN materials m ON m.id=e.material_id AND m.class_id=e.class_id
		WHERE m.id=? AND m.class_id=?`, materialID, user.ClassID).Scan(&entryID, &content)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "重建索引失败")
		return
	}
	strategyJSON, _ := json.Marshal(strategy)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "重建索引失败")
		return
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT status FROM knowledge_index_jobs
		WHERE entry_id=? AND index_version=? FOR UPDATE`, entryID, s.cfg.IndexVersion).Scan(&status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusInternalServerError, "重建索引失败")
		return
	}
	if status == "processing" {
		httpx.Error(w, http.StatusConflict, "索引正在处理")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO knowledge_index_jobs
		(entry_id,class_id,content_hash,index_version,strategy_json,status,attempts,next_attempt_at)
		VALUES (?,?,?,?,?,'pending',0,UTC_TIMESTAMP(6))
		ON DUPLICATE KEY UPDATE content_hash=VALUES(content_hash),strategy_json=VALUES(strategy_json),
		status='pending',attempts=0,leased_until=NULL,next_attempt_at=UTC_TIMESTAMP(6),last_error=NULL`,
		entryID, user.ClassID, contentHash(content), s.cfg.IndexVersion, string(strategyJSON))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "重建索引失败")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE knowledge_chunks SET index_status='pending'
		WHERE entry_id=? AND index_version=?`, entryID, s.cfg.IndexVersion); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "重建索引失败")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "重建索引失败")
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"message": "索引重建已排队"})
}
