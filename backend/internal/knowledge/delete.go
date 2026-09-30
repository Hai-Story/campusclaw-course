package knowledge

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"campusclaw/internal/auth"
	"campusclaw/internal/httpx"
)

func (s *Service) DeleteMaterialHTTP(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user.Role != "teacher" {
		httpx.Error(w, http.StatusForbidden, "仅教师可以删除材料")
		return
	}
	materialID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || materialID == 0 {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	defer tx.Rollback()

	var entryID uint64
	var storedName, source string
	err = tx.QueryRowContext(r.Context(), `SELECT e.id,m.stored_name,m.source FROM materials m
		JOIN knowledge_entries e ON e.material_id=m.id AND e.class_id=m.class_id
		WHERE m.id=? AND m.class_id=? FOR UPDATE`, materialID, user.ClassID).
		Scan(&entryID, &storedName, &source)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "未找到材料")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	if filepath.Base(storedName) != storedName || storedName == "." {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}

	rows, err := tx.QueryContext(r.Context(), `SELECT status FROM knowledge_index_jobs WHERE entry_id=? FOR UPDATE`, entryID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	processing := false
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			rows.Close()
			httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
			return
		}
		processing = processing || status == "processing"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	if processing {
		httpx.Error(w, http.StatusConflict, "材料正在建立索引，请稍后重试")
		return
	}

	path := filepath.Join(s.cfg.UploadDir, storedName)
	quarantined := ""
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	target := path + ".deleting-" + hex.EncodeToString(suffix[:])
	if err := os.Rename(path, target); err == nil {
		quarantined = target
	} else if !errors.Is(err, os.ErrNotExist) {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	committed := false
	defer func() {
		if !committed && quarantined != "" {
			if err := os.Rename(quarantined, path); err != nil {
				log.Printf("restore material file after failed delete: %v", err)
			}
		}
	}()

	if err := s.vectors.DeleteEntry(r.Context(), user.ClassID, entryID); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "向量索引暂时不可用，请稍后重试")
		return
	}
	if source == "seed" {
		if _, err := tx.ExecContext(r.Context(), `INSERT IGNORE INTO deleted_seed_materials (stored_name) VALUES (?)`, storedName); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM materials WHERE id=? AND class_id=?`, materialID, user.ClassID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "删除材料失败")
		return
	}
	committed = true
	if quarantined != "" {
		if err := os.Remove(quarantined); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove deleted material file: %v", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
