package db

import (
	"campusclaw/internal/catalog"
	"context"
	"database/sql"
	"encoding/json"
)

func migrateBuiltins(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS builtin_initializations (
 class_id BIGINT UNSIGNED NOT NULL, version VARCHAR(32) NOT NULL, objects_json JSON NOT NULL,
 PRIMARY KEY(class_id,version), FOREIGN KEY(class_id) REFERENCES classes(id)
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	for _, col := range []struct{ table, name string }{{"tutor_skills", "name"}, {"tutor_messages", "skill_name"}} {
		var length int
		if err := database.QueryRowContext(ctx, `SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, col.table, col.name).Scan(&length); err != nil {
			return err
		}
		if length < 64 {
			suffix := " NOT NULL"
			if col.name == "skill_name" {
				suffix += " DEFAULT ''"
			}
			if _, err := database.ExecContext(ctx, "ALTER TABLE "+col.table+" MODIFY COLUMN "+col.name+" VARCHAR(64)"+suffix); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedVersion records success atomically; deletion or renaming never triggers reseeding.
func seedVersion(ctx context.Context, database *sql.DB, class uint64, version string, fn func(*sql.Tx) ([]uint64, error)) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT IGNORE INTO builtin_initializations(class_id,version,objects_json) VALUES(?,?,'[]')`, class, version)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tx.Commit()
	}
	ids, err := fn(tx)
	if err != nil {
		return err
	}
	if ids == nil {
		ids = []uint64{}
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE builtin_initializations SET objects_json=? WHERE class_id=? AND version=?`, string(raw), class, version); err != nil {
		return err
	}
	return tx.Commit()
}

// SeedBuiltins covers every class, including classes created after initial installation.
func SeedBuiltins(ctx context.Context, database *sql.DB) error {
	rows, err := database.QueryContext(ctx, `SELECT id FROM classes ORDER BY id`)
	if err != nil {
		return err
	}
	var classes []uint64
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		classes = append(classes, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, class := range classes {
		if err = seedSkills(ctx, database, class); err != nil {
			return err
		}
	}
	return nil
}
func seedSkills(ctx context.Context, database *sql.DB, class uint64) error {
	return seedVersion(ctx, database, class, "skills-v1", func(tx *sql.Tx) ([]uint64, error) {
		ids := []uint64{}
		for _, skill := range catalog.Skills {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tutor_skills WHERE class_id=? AND name=?`, class, skill.Name).Scan(&count); err != nil {
				return nil, err
			}
			if count > 0 {
				continue
			}
			result, err := tx.ExecContext(ctx, `INSERT INTO tutor_skills(class_id,name,trigger_text,subject_text,tools_json,description,source_text,enabled) VALUES(?,?,?,?,'["knowledge_search"]',?,?,1)`, class, skill.Name, skill.Trigger, skill.Subject, skill.Description, skill.Source())
			if err != nil {
				return nil, err
			}
			id, err := result.LastInsertId()
			if err != nil {
				return nil, err
			}
			ids = append(ids, uint64(id))
		}
		return ids, nil
	})
}
