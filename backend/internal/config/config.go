package config

import (
	"errors"
	"fmt"
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
}

func Load() (Config, error) {
	required := []string{
		"API_ADDR", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"SESSION_SECRET", "SESSION_TTL_MINUTES", "UPLOAD_DIR", "MAX_UPLOAD_BYTES",
		"LOGIN_MAX_FAILURES", "LOGIN_LOCK_SECONDS", "SEED_TEACHER_A_PASSWORD",
		"SEED_STUDENT_A1_PASSWORD", "SEED_STUDENT_B1_PASSWORD",
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
