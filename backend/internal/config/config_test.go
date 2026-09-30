package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsMissingEnvironment(t *testing.T) {
	for _, key := range []string{
		"API_ADDR", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"SESSION_SECRET", "SESSION_TTL_MINUTES", "UPLOAD_DIR", "MAX_UPLOAD_BYTES",
		"LOGIN_MAX_FAILURES", "LOGIN_LOCK_SECONDS", "SEED_TEACHER_A_PASSWORD",
		"SEED_STUDENT_A1_PASSWORD", "SEED_STUDENT_B1_PASSWORD",
	} {
		t.Setenv(key, "")
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("expected a missing environment error, got %v", err)
	}
}

func TestLoadAcceptsCompleteEnvironment(t *testing.T) {
	values := map[string]string{
		"API_ADDR": ":8081", "DB_HOST": "db", "DB_PORT": "3306", "DB_NAME": "campusclaw",
		"DB_USER": "app", "DB_PASSWORD": "db-pass", "SESSION_SECRET": strings.Repeat("s", 32),
		"SESSION_TTL_MINUTES": "480", "UPLOAD_DIR": "/tmp/uploads", "MAX_UPLOAD_BYTES": "1024",
		"LOGIN_MAX_FAILURES": "5", "LOGIN_LOCK_SECONDS": "300", "SEED_TEACHER_A_PASSWORD": "one",
		"SEED_STUDENT_A1_PASSWORD": "two", "SEED_STUDENT_B1_PASSWORD": "three",
		"QDRANT_URL": "http://qdrant:6333", "GATEWAY_BASE_URL": "http://gateway:8000/v1",
		"GATEWAY_API_KEY": "test-key", "EMBEDDING_MODEL": "test-embedding", "CHAT_MODEL": "test-chat",
		"EMBEDDING_DIMENSION": "4", "INDEX_RETRY_MAX": "3", "INDEX_RETRY_SECONDS": "2",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxUploadBytes != 1024 || cfg.LoginMaxFailures != 5 || cfg.EmbeddingDim != 4 || cfg.IndexVersion == "" {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}
}
