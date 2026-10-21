package db

var tutorSchema = []string{
	`CREATE TABLE IF NOT EXISTS tutor_skills (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, class_id BIGINT UNSIGNED NOT NULL,
 name VARCHAR(32) NOT NULL, trigger_text VARCHAR(120) NOT NULL, subject_text VARCHAR(32) NOT NULL DEFAULT '',
 tools_json JSON NOT NULL, description TEXT NOT NULL, source_text TEXT NOT NULL, enabled BOOLEAN NOT NULL,
 UNIQUE KEY uq_skill_class_name(class_id,name), FOREIGN KEY(class_id) REFERENCES classes(id)
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS tutor_agents (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, class_id BIGINT UNSIGNED NOT NULL,
 name VARCHAR(64) NOT NULL, prompt TEXT NOT NULL, enabled BOOLEAN NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_agent_class_name(class_id,name), FOREIGN KEY(class_id) REFERENCES classes(id)
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS tutor_agent_skills (
 agent_id BIGINT UNSIGNED NOT NULL, skill_id BIGINT UNSIGNED NOT NULL, PRIMARY KEY(agent_id,skill_id),
 FOREIGN KEY(agent_id) REFERENCES tutor_agents(id) ON DELETE CASCADE,
 FOREIGN KEY(skill_id) REFERENCES tutor_skills(id) ON DELETE CASCADE
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS tutor_agent_materials (
 agent_id BIGINT UNSIGNED NOT NULL, material_id BIGINT UNSIGNED NOT NULL, PRIMARY KEY(agent_id,material_id),
 FOREIGN KEY(agent_id) REFERENCES tutor_agents(id) ON DELETE CASCADE,
 FOREIGN KEY(material_id) REFERENCES materials(id) ON DELETE CASCADE
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS tutor_conversations (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, user_id BIGINT UNSIGNED NOT NULL, class_id BIGINT UNSIGNED NOT NULL,
 agent_id BIGINT UNSIGNED NOT NULL, title VARCHAR(24) NOT NULL DEFAULT '新的提问',
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(user_id) REFERENCES users(id), FOREIGN KEY(class_id) REFERENCES classes(id),
 FOREIGN KEY(agent_id) REFERENCES tutor_agents(id), INDEX idx_conversation_owner(user_id,class_id,updated_at)
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS tutor_messages (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, conversation_id BIGINT UNSIGNED NOT NULL,
 role ENUM('user','assistant') NOT NULL, content MEDIUMTEXT NOT NULL, citations_json JSON NOT NULL,
 skill_name VARCHAR(32) NOT NULL DEFAULT '', status ENUM('complete','incomplete') NOT NULL DEFAULT 'complete',
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(conversation_id) REFERENCES tutor_conversations(id) ON DELETE CASCADE,
 INDEX idx_message_conversation(conversation_id,id)
 ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}
