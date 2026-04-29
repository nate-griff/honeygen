package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
)

func TestRunShutsDownOnContextCancellation(t *testing.T) {
	cfg := config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    "127.0.0.1:0",
		AdminPassword:               "test-admin-password",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		InternalEventIngestToken:    "test-internal-event-token",
		SQLitePath:                  filepath.Join(t.TempDir(), "api.db"),
		GeneratedAssetsDir:          filepath.Join(t.TempDir(), "generated"),
		StorageRoot:                 filepath.Join(t.TempDir(), "storage"),
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
