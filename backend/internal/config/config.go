package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr           string
	DatabasePath   string
	ContentDir     string
	WebDir         string
	LogLevel       string
	LogFormat      string
	EditorKey      string
	JWTSecret      string
	TokenTTL       time.Duration
	CookieSecure   bool
	UploadDir      string
	UploadMaxBytes int64
	MaxTags        int
}

func Load() (Config, error) {
	path := ".env"
	if cwd, err := os.Getwd(); err == nil && filepath.Base(cwd) == "backend" {
		path = filepath.Join("..", ".env")
	}
	if override := os.Getenv("BLOG_ENV_FILE"); override != "" {
		path = override
	}
	values, err := readEnvFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("load %s: %w", path, err)
	}
	ttl, err := time.ParseDuration(env("BLOG_TOKEN_TTL", "8h", values))
	if err != nil || ttl < time.Minute || ttl > 7*24*time.Hour {
		return Config{}, fmt.Errorf("BLOG_TOKEN_TTL must be between 1m and 168h")
	}
	secure, err := strconv.ParseBool(env("BLOG_COOKIE_SECURE", "false", values))
	if err != nil {
		return Config{}, fmt.Errorf("BLOG_COOKIE_SECURE must be true or false")
	}
	maxMB, err := strconv.ParseInt(env("BLOG_UPLOAD_MAX_MB", "100", values), 10, 64)
	if err != nil || maxMB < 1 || maxMB > 1024 {
		return Config{}, fmt.Errorf("BLOG_UPLOAD_MAX_MB must be between 1 and 1024")
	}
	maxTags, err := strconv.Atoi(env("BLOG_MAX_TAGS", "10", values))
	if err != nil || maxTags < 1 || maxTags > 50 {
		return Config{}, fmt.Errorf("BLOG_MAX_TAGS must be between 1 and 50")
	}
	cfg := Config{
		Addr:         env("BLOG_ADDR", ":8080", values),
		DatabasePath: env("BLOG_DB_PATH", "./data/blog.db", values),
		ContentDir:   env("BLOG_CONTENT_DIR", "./content/posts", values),
		WebDir:       env("BLOG_WEB_DIR", "../frontend/dist", values),
		LogLevel:     env("BLOG_LOG_LEVEL", "info", values),
		LogFormat:    env("BLOG_LOG_FORMAT", "json", values),
		EditorKey:    env("BLOG_EDITOR_KEY", "", values),
		JWTSecret:    env("BLOG_JWT_SECRET", "", values),
		TokenTTL:     ttl, CookieSecure: secure,
		UploadDir:      env("BLOG_UPLOAD_DIR", "./data/uploads", values),
		UploadMaxBytes: maxMB << 20, MaxTags: maxTags,
	}
	if cfg.EditorKey != "" && (len(cfg.EditorKey) < 16 || len(cfg.JWTSecret) < 32) {
		return Config{}, fmt.Errorf("editor login requires BLOG_EDITOR_KEY (at least 16 characters) and BLOG_JWT_SECRET (at least 32 characters)")
	}
	return cfg, nil
}

func env(key, fallback string, values map[string]string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value := values[key]; value != "" {
		return value
	}
	return fallback
}

func readEnvFile(path string) (map[string]string, error) {
	values := make(map[string]string)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		entry := strings.TrimSpace(scanner.Text())
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || !validKey(key) {
			return nil, fmt.Errorf("line %d: expected KEY=value", line)
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, `"`) {
			value, err = strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid quoted value: %w", line, err)
			}
		} else if strings.HasPrefix(value, "'") {
			if len(value) < 2 || !strings.HasSuffix(value, "'") {
				return nil, fmt.Errorf("line %d: invalid quoted value", line)
			}
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func validKey(key string) bool {
	for i, char := range key {
		if char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || i > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return key != ""
}
