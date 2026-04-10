package main

import (
	"context"
	"io"
	"log/slog"
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
