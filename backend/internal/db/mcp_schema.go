package db

import (
	"context"
	"database/sql"
	"fmt"
)

var mcpSchema = []string{
	`CREATE TABLE IF NOT EXISTS mcp_servers (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, class_id BIGINT UNSIGNED NOT NULL,
 name VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, base_url VARCHAR(1024) NOT NULL, token_text TEXT NOT NULL,
 status ENUM('connected','disconnected') NOT NULL DEFAULT 'disconnected', revision BIGINT UNSIGNED NOT NULL DEFAULT 0,
 protocol_version VARCHAR(32) NOT NULL DEFAULT '', last_error VARCHAR(255) NOT NULL DEFAULT '',
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_mcp_server_class(class_id,name), FOREIGN KEY(class_id) REFERENCES classes(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS mcp_tools (
 server_id BIGINT UNSIGNED NOT NULL, name VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 description TEXT NOT NULL, schema_json JSON NOT NULL, descriptor_hash CHAR(64) NOT NULL,
 PRIMARY KEY(server_id,name), FOREIGN KEY(server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS mcp_tool_approvals (
 skill_id BIGINT UNSIGNED NOT NULL, server_id BIGINT UNSIGNED NOT NULL, tool_name VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 descriptor_hash CHAR(64) NOT NULL, teacher_id BIGINT UNSIGNED NOT NULL, approved_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(skill_id,server_id,tool_name), FOREIGN KEY(skill_id) REFERENCES tutor_skills(id) ON DELETE CASCADE,
 FOREIGN KEY(server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE, FOREIGN KEY(teacher_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS mcp_call_audits (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, class_id BIGINT UNSIGNED NOT NULL, server_id BIGINT UNSIGNED NULL,
 server_name VARCHAR(32) NOT NULL, tool_ref VARCHAR(96) NOT NULL, user_id BIGINT UNSIGNED NOT NULL,
 conversation_id BIGINT UNSIGNED NOT NULL, question_id BIGINT UNSIGNED NOT NULL,
 call_id VARCHAR(64) NOT NULL UNIQUE, round_no INT UNSIGNED NOT NULL, status ENUM('pending','ok','error','denied') NOT NULL,
 summary VARCHAR(255) NOT NULL DEFAULT '', duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(server_id) REFERENCES mcp_servers(id) ON DELETE SET NULL,
 FOREIGN KEY(user_id) REFERENCES users(id), FOREIGN KEY(conversation_id) REFERENCES tutor_conversations(id),
 FOREIGN KEY(question_id) REFERENCES tutor_messages(id), INDEX idx_mcp_audit_class(class_id,created_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS mcp_conversation_usage (
 conversation_id BIGINT UNSIGNED PRIMARY KEY, attempts INT UNSIGNED NOT NULL DEFAULT 0,
 FOREIGN KEY(conversation_id) REFERENCES tutor_conversations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS mcp_tool_changes (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, class_id BIGINT UNSIGNED NOT NULL, server_id BIGINT UNSIGNED NULL,
 server_name VARCHAR(32) NOT NULL, tool_name VARCHAR(48) NOT NULL, old_json JSON NULL, new_json JSON NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(class_id) REFERENCES classes(id), FOREIGN KEY(server_id) REFERENCES mcp_servers(id) ON DELETE SET NULL,
 INDEX idx_mcp_changes(class_id,server_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

func migrateMCP(ctx context.Context, database *sql.DB) error {
	for _, statement := range mcpSchema {
		if _, e := database.ExecContext(ctx, statement); e != nil {
			return fmt.Errorf("MCP migration: %w", e)
		}
	}
	var count int
	if e := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tutor_messages' AND column_name='tool_calls_json'`).Scan(&count); e != nil {
		return e
	}
	if count == 0 {
		if _, e := database.ExecContext(ctx, `ALTER TABLE tutor_messages ADD COLUMN tool_calls_json JSON NULL`); e != nil {
			return e
		}
	}
	// This application is single-instance: pending rows belong to a previously interrupted process.
	_, e := database.ExecContext(ctx, `UPDATE mcp_call_audits SET status='error',summary='进程中断，远端结果未知' WHERE status='pending'`)
	return e
}
