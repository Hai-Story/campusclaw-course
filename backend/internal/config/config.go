package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIAddr          string
	DBHost           string
	DBPort           string
	DBName           string
	DBUser           string
	DBPassword       string
	SessionSecret    string
	SessionTTL       time.Duration
	UploadDir        string
	MaxUploadBytes   int64
	LoginMaxFailures int
	LoginLock        time.Duration
	SeedPasswords    map[string]string
	QdrantURL        string
	GatewayBaseURL   string
	GatewayAPIKey    string
	EmbeddingModel   string
	ChatModel        string
	EmbeddingDim     int
	IndexVersion     string
	IndexRetryMax    int
	IndexRetry       time.Duration
}

func Load() (Config, error) {
	required := []string{
		"API_ADDR", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"SESSION_SECRET", "SESSION_TTL_MINUTES", "UPLOAD_DIR", "MAX_UPLOAD_BYTES",
		"LOGIN_MAX_FAILURES", "LOGIN_LOCK_SECONDS", "SEED_TEACHER_A_PASSWORD",
		"SEED_STUDENT_A1_PASSWORD", "SEED_STUDENT_B1_PASSWORD",
		"QDRANT_URL", "GATEWAY_BASE_URL", "GATEWAY_API_KEY", "EMBEDDING_MODEL",
		"CHAT_MODEL", "EMBEDDING_DIMENSION", "INDEX_RETRY_MAX", "INDEX_RETRY_SECONDS",
	}
	values := make(map[string]string, len(required))
	var missing []string
	for _, key := range required {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			missing = append(missing, key)
		}
		values[key] = value
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if len(values["SESSION_SECRET"]) < 32 {
		return Config{}, errors.New("SESSION_SECRET must be at least 32 characters")
	}

	ttlMinutes, err := positiveInt(values["SESSION_TTL_MINUTES"], "SESSION_TTL_MINUTES")
	if err != nil {
		return Config{}, err
	}
	maxBytes, err := positiveInt64(values["MAX_UPLOAD_BYTES"], "MAX_UPLOAD_BYTES")
	if err != nil {
		return Config{}, err
	}
	maxFailures, err := positiveInt(values["LOGIN_MAX_FAILURES"], "LOGIN_MAX_FAILURES")
	if err != nil {
		return Config{}, err
	}
	lockSeconds, err := positiveInt(values["LOGIN_LOCK_SECONDS"], "LOGIN_LOCK_SECONDS")
	if err != nil {
		return Config{}, err
	}
	embeddingDim, err := positiveInt(values["EMBEDDING_DIMENSION"], "EMBEDDING_DIMENSION")
	if err != nil || embeddingDim > 4096 {
		return Config{}, errors.New("EMBEDDING_DIMENSION must be between 1 and 4096")
	}
	retryMax, err := positiveInt(values["INDEX_RETRY_MAX"], "INDEX_RETRY_MAX")
	if err != nil {
		return Config{}, err
	}
	retrySeconds, err := positiveInt(values["INDEX_RETRY_SECONDS"], "INDEX_RETRY_SECONDS")
	if err != nil {
		return Config{}, err
	}
	for _, name := range []string{"QDRANT_URL", "GATEWAY_BASE_URL"} {
		parsed, err := url.Parse(values[name])
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Config{}, fmt.Errorf("%s must be an absolute HTTP URL", name)
		}
	}
	versionInput := fmt.Sprintf("chunks-v2|%s|%d", values["EMBEDDING_MODEL"], embeddingDim)
	versionHash := sha256.Sum256([]byte(versionInput))

	return Config{
		APIAddr:          values["API_ADDR"],
		DBHost:           values["DB_HOST"],
		DBPort:           values["DB_PORT"],
		DBName:           values["DB_NAME"],
		DBUser:           values["DB_USER"],
		DBPassword:       values["DB_PASSWORD"],
		SessionSecret:    values["SESSION_SECRET"],
		SessionTTL:       time.Duration(ttlMinutes) * time.Minute,
		UploadDir:        values["UPLOAD_DIR"],
		MaxUploadBytes:   maxBytes,
		LoginMaxFailures: maxFailures,
		LoginLock:        time.Duration(lockSeconds) * time.Second,
		SeedPasswords: map[string]string{
			"teacher_a":  values["SEED_TEACHER_A_PASSWORD"],
			"student_a1": values["SEED_STUDENT_A1_PASSWORD"],
			"student_b1": values["SEED_STUDENT_B1_PASSWORD"],
		},
		QdrantURL:      strings.TrimRight(values["QDRANT_URL"], "/"),
		GatewayBaseURL: strings.TrimRight(values["GATEWAY_BASE_URL"], "/"),
		GatewayAPIKey:  values["GATEWAY_API_KEY"],
		EmbeddingModel: values["EMBEDDING_MODEL"],
		ChatModel:      values["CHAT_MODEL"],
		EmbeddingDim:   embeddingDim,
		IndexVersion:   "v2-" + hex.EncodeToString(versionHash[:12]),
		IndexRetryMax:  retryMax,
		IndexRetry:     time.Duration(retrySeconds) * time.Second,
	}, nil
}

func positiveInt(value, name string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func positiveInt64(value, name string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}
