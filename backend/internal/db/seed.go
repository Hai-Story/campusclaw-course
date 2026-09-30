package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"campusclaw/internal/config"
	"golang.org/x/crypto/bcrypt"
)

type seedUser struct {
	Username string
	Role     string
	Class    string
}

type seedMaterial struct {
	ClassName  string
	Uploader   string
	Title      string
	StoredName string
	Content    string
}

func Seed(ctx context.Context, database *sql.DB, cfg config.Config) error {
	if err := os.MkdirAll(cfg.UploadDir, 0o750); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}
	for _, className := range []string{"A 班", "B 班"} {
		if _, err := database.ExecContext(ctx, `INSERT IGNORE INTO classes (name) VALUES (?)`, className); err != nil {
			return fmt.Errorf("seed class %s: %w", className, err)
		}
	}
	users := []seedUser{
		{Username: "teacher_a", Role: "teacher", Class: "A 班"},
		{Username: "student_a1", Role: "student", Class: "A 班"},
		{Username: "student_b1", Role: "student", Class: "B 班"},
	}
	for _, user := range users {
		var exists int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ?`, user.Username).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedPasswords[user.Username]), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash seed password: %w", err)
		}
		_, err = database.ExecContext(ctx, `INSERT INTO users (username, password_hash, role, class_id)
			SELECT ?, ?, ?, id FROM classes WHERE name = ?`, user.Username, string(hash), user.Role, user.Class)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", user.Username, err)
		}
	}

	materials := []seedMaterial{
		{ClassName: "A 班", Uploader: "teacher_a", Title: "A 班 · 语文教研摘记", StoredName: "seed-class-a.md", Content: "# A 班教研摘记\n\n以文本细读为起点，记录本班课堂观察。\n"},
		{ClassName: "B 班", Uploader: "student_b1", Title: "B 班 · 数学复盘资料", StoredName: "seed-class-b.txt", Content: "B 班材料：本周聚焦方程建模与错因分类。\n"},
	}
	for _, item := range materials {
		if err := seedOneMaterial(ctx, database, cfg.UploadDir, item); err != nil {
			return err
		}
	}
	return nil
}

func seedOneMaterial(ctx context.Context, database *sql.DB, uploadDir string, item seedMaterial) error {
	var deleted int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM deleted_seed_materials WHERE stored_name=?`, item.StoredName).Scan(&deleted); err != nil {
		return err
	}
	if deleted > 0 {
		return nil
	}
	var materialID uint64
	err := database.QueryRowContext(ctx, `SELECT m.id FROM materials m JOIN classes c ON c.id=m.class_id
		WHERE c.name=? AND m.source='seed' AND m.title=? LIMIT 1`, item.ClassName, item.Title).Scan(&materialID)
	if err == nil {
		return ensureSeedFile(uploadDir, item.StoredName, item.Content)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	path := filepath.Join(uploadDir, item.StoredName)
	created := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(item.Content), 0o640); err != nil {
			return fmt.Errorf("write seed file: %w", err)
		}
		created = true
	} else if err != nil {
		return err
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		if created {
			_ = os.Remove(path)
		}
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO materials
		(class_id, uploader_id, title, original_name, stored_name, media_type, size_bytes, source)
		SELECT c.id, u.id, ?, ?, ?, ?, ?, 'seed' FROM classes c JOIN users u ON u.username=? WHERE c.name=?`,
		item.Title, item.StoredName, item.StoredName, mediaTypeFor(item.StoredName), len([]byte(item.Content)), item.Uploader, item.ClassName)
	if err != nil {
		if created {
			_ = os.Remove(path)
		}
		return fmt.Errorf("insert seed material: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		if created {
			_ = os.Remove(path)
		}
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO knowledge_entries (material_id, class_id, content, source)
		SELECT ?, id, ?, ? FROM classes WHERE name=?`, id, item.Content, item.StoredName, item.ClassName)
	if err != nil {
		if created {
			_ = os.Remove(path)
		}
		return fmt.Errorf("insert seed knowledge: %w", err)
	}
	if err := tx.Commit(); err != nil {
		if created {
			_ = os.Remove(path)
		}
		return err
	}
	return nil
}

func ensureSeedFile(uploadDir, name, content string) error {
	path := filepath.Join(uploadDir, name)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o640)
}

func mediaTypeFor(name string) string {
	if filepath.Ext(name) == ".md" {
		return "text/markdown; charset=utf-8"
	}
	return "text/plain; charset=utf-8"
}
