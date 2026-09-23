package materials

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"campusclaw/internal/auth"
	"campusclaw/internal/httpx"
)

type Handler struct {
	db             *sql.DB
	uploadDir      string
	maxUploadBytes int64
}

type material struct {
	ID           uint64 `json:"id"`
	ClassID      uint64 `json:"class_id"`
	ClassName    string `json:"class_name"`
	Title        string `json:"title"`
	OriginalName string `json:"original_name"`
	StoredName   string `json:"-"`
	MediaType    string `json:"media_type"`
	SizeBytes    int64  `json:"size_bytes"`
	CreatedAt    string `json:"created_at"`
	Content      string `json:"content,omitempty"`
}

func New(database *sql.DB, uploadDir string, maxUploadBytes int64) *Handler {
	return &Handler{db: database, uploadDir: uploadDir, maxUploadBytes: maxUploadBytes}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + q + "%"
	rows, err := h.db.QueryContext(r.Context(), `SELECT m.id, m.class_id, c.name, m.title, m.original_name,
		m.media_type, m.size_bytes, DATE_FORMAT(m.created_at, '%Y-%m-%dT%H:%i:%sZ')
		FROM materials m JOIN classes c ON c.id=m.class_id
		JOIN knowledge_entries k ON k.material_id=m.id
		WHERE m.class_id=? AND (?='' OR m.title LIKE ? OR k.content LIKE ?)
		ORDER BY m.created_at DESC, m.id DESC`, user.ClassID, q, like, like)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料列表读取失败")
		return
	}
	defer rows.Close()
	items := make([]material, 0)
	for rows.Next() {
		var item material
		if err := rows.Scan(&item.ID, &item.ClassID, &item.ClassName, &item.Title, &item.OriginalName,
			&item.MediaType, &item.SizeBytes, &item.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "材料列表读取失败")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料列表读取失败")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"materials": items})
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	item, ok := h.authorizedMaterial(w, r)
	if !ok {
		return
	}
	item.StoredName = ""
	httpx.JSON(w, http.StatusOK, map[string]any{"material": item})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	item, ok := h.authorizedMaterial(w, r)
	if !ok {
		return
	}
	if filepath.Base(item.StoredName) != item.StoredName {
		httpx.Error(w, http.StatusInternalServerError, "文件不可用")
		return
	}
	path := filepath.Join(h.uploadDir, item.StoredName)
	file, err := os.Open(path)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", item.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.OriginalName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, file)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user.Role != "teacher" {
		httpx.Error(w, http.StatusForbidden, "仅教师可以上传材料")
		return
	}

	requestLimit := h.maxUploadBytes + (1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, requestLimit)
	if r.ContentLength > requestLimit && r.ContentLength >= 0 {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "文件超过大小上限")
		return
	}
	if err := r.ParseMultipartForm(requestLimit); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "文件超过大小上限")
		} else {
			httpx.Error(w, http.StatusBadRequest, "上传请求格式错误")
		}
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	originalName := safeDisplayName(header.Filename)
	extension := strings.ToLower(filepath.Ext(originalName))
	if extension != ".txt" && extension != ".md" {
		httpx.Error(w, http.StatusBadRequest, "仅支持 .txt 和 .md 文件")
		return
	}
	content, err := io.ReadAll(io.LimitReader(file, h.maxUploadBytes+1))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "文件读取失败")
		return
	}
	if int64(len(content)) > h.maxUploadBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "文件超过大小上限")
		return
	}
	if len(content) == 0 || !utf8.Valid(content) || strings.TrimSpace(string(content)) == "" {
		httpx.Error(w, http.StatusBadRequest, "文件必须是非空 UTF-8 文本")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(originalName, extension)
	}
	if title == "" || utf8.RuneCountInString(title) > 255 {
		httpx.Error(w, http.StatusBadRequest, "标题不能为空且不得超过 255 个字符")
		return
	}
	storedName, err := generatedName(extension)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "文件保存失败")
		return
	}
	path := filepath.Join(h.uploadDir, storedName)
	stored, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "文件保存失败")
		return
	}
	written := false
	defer func() {
		_ = stored.Close()
		if !written {
			_ = os.Remove(path)
		}
	}()
	if _, err := stored.Write(content); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "文件保存失败")
		return
	}
	if err := stored.Sync(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "文件保存失败")
		return
	}
	if err := stored.Close(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "文件保存失败")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料入库失败")
		return
	}
	defer tx.Rollback()
	mediaType := "text/plain; charset=utf-8"
	if extension == ".md" {
		mediaType = "text/markdown; charset=utf-8"
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO materials
		(class_id, uploader_id, title, original_name, stored_name, media_type, size_bytes, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'upload')`, user.ClassID, user.ID, title, originalName, storedName, mediaType, len(content))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料入库失败")
		return
	}
	materialID, err := result.LastInsertId()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料入库失败")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO knowledge_entries (material_id, class_id, content, source)
		VALUES (?, ?, ?, ?)`, materialID, user.ClassID, string(content), originalName)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料入库失败")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料入库失败")
		return
	}
	written = true
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": materialID, "message": "材料已入库"})
}

func (h *Handler) authorizedMaterial(w http.ResponseWriter, r *http.Request) (material, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return material{}, false
	}
	var item material
	err = h.db.QueryRowContext(r.Context(), `SELECT m.id, m.class_id, c.name, m.title, m.original_name, m.stored_name,
		m.media_type, m.size_bytes, DATE_FORMAT(m.created_at, '%Y-%m-%dT%H:%i:%sZ'), k.content
		FROM materials m JOIN classes c ON c.id=m.class_id JOIN knowledge_entries k ON k.material_id=m.id
		WHERE m.id=?`, id).Scan(&item.ID, &item.ClassID, &item.ClassName, &item.Title, &item.OriginalName,
		&item.StoredName, &item.MediaType, &item.SizeBytes, &item.CreatedAt, &item.Content)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return material{}, false
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "材料读取失败")
		return material{}, false
	}
	user, _ := auth.UserFromContext(r.Context())
	if item.ClassID != user.ClassID {
		log.Printf("cross-class material access denied: user=%d user_class=%d material=%d material_class=%d", user.ID, user.ClassID, item.ID, item.ClassID)
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return material{}, false
	}
	return item, true
}

func generatedName(extension string) (string, error) {
	buffer := make([]byte, 20)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer) + extension, nil
}

func safeDisplayName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	runes := []rune(name)
	if len(runes) > 255 {
		name = string(runes[len(runes)-255:])
	}
	return name
}

func (m material) String() string {
	return fmt.Sprintf("material(%d, class=%d)", m.ID, m.ClassID)
}
