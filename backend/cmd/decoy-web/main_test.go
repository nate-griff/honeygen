package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
)

func TestRunShutsDownOnContextCancellation(t *testing.T) {
	cfg := config.Config{
		AppEnv:             "test",
		HTTPAddr:           "127.0.0.1:0",
		GeneratedAssetsDir: t.TempDir(),
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, cfg, logger)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not shut down after context cancellation")
	}
}

func TestLoadConfigUsesDecoyDefaultHTTPAddr(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("APP_CONFIG_PATH", "")
	t.Setenv("APP_ENV", "test")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "test-internal-event-token")

	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.HTTPAddr != ":8081" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8081")
	}
}

func TestLoadConfigPreservesExplicitHTTPAddrOverride(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("APP_CONFIG_PATH", "")
	t.Setenv("APP_ENV", "test")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "test-internal-event-token")

	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
}

func TestLoadConfigPreservesConfigFileHTTPAddr(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("APP_CONFIG_PATH", "")
	t.Setenv("APP_ENV", "test")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "test-internal-event-token")

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"http_addr":":8080"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
}
