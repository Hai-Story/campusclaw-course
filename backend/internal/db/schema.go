package db

import (
	"context"
	"database/sql"
	"fmt"
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS classes (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100) NOT NULL UNIQUE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS users (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(100) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		role ENUM('teacher','student') NOT NULL,
		class_id BIGINT UNSIGNED NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_users_class FOREIGN KEY (class_id) REFERENCES classes(id),
		INDEX idx_users_class (class_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS sessions (
		token_hash CHAR(64) NOT NULL PRIMARY KEY,
		user_id BIGINT UNSIGNED NOT NULL,
		role ENUM('teacher','student') NOT NULL,
		class_id BIGINT UNSIGNED NOT NULL,
		expires_at DATETIME(6) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		CONSTRAINT fk_sessions_class FOREIGN KEY (class_id) REFERENCES classes(id),
		INDEX idx_sessions_expiry (expires_at),
		INDEX idx_sessions_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS materials (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		class_id BIGINT UNSIGNED NOT NULL,
		uploader_id BIGINT UNSIGNED NOT NULL,
		title VARCHAR(255) NOT NULL,
		original_name VARCHAR(255) NOT NULL,
		stored_name VARCHAR(255) NOT NULL UNIQUE,
		media_type VARCHAR(100) NOT NULL,
		size_bytes BIGINT UNSIGNED NOT NULL,
		source ENUM('seed','upload') NOT NULL DEFAULT 'upload',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_materials_class FOREIGN KEY (class_id) REFERENCES classes(id),
		CONSTRAINT fk_materials_uploader FOREIGN KEY (uploader_id) REFERENCES users(id),
		INDEX idx_materials_class_created (class_id, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS knowledge_entries (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		material_id BIGINT UNSIGNED NOT NULL UNIQUE,
		class_id BIGINT UNSIGNED NOT NULL,
		content MEDIUMTEXT NOT NULL,
		source VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_knowledge_material FOREIGN KEY (material_id) REFERENCES materials(id) ON DELETE CASCADE,
		CONSTRAINT fk_knowledge_class FOREIGN KEY (class_id) REFERENCES classes(id),
		INDEX idx_knowledge_class (class_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

func Migrate(ctx context.Context, database *sql.DB) error {
	for index, statement := range schema {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migration statement %d: %w", index+1, err)
		}
	}
	return nil
}
