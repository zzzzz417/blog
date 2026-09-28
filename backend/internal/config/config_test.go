package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvFileAndProcessOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(`# Local configuration
BLOG_ADDR=:9091
BLOG_CONTENT_DIR="posts with spaces"
BLOG_LOG_FORMAT='text'
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLOG_ENV_FILE", path)
	t.Setenv("BLOG_ADDR", ":9092")
	t.Setenv("BLOG_CONTENT_DIR", "")
	t.Setenv("BLOG_LOG_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9092" || cfg.ContentDir != "posts with spaces" || cfg.LogFormat != "text" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsMalformedEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("BROKEN_LINE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLOG_ENV_FILE", path)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("expected line-numbered configuration error, got %v", err)
	}
}
