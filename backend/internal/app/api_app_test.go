package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/natet/honeygen/backend/internal/config"
	"github.com/natet/honeygen/backend/internal/deployments"
)

func TestNewAPIAppStartsDeploymentFromStorageRoot(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Config{
		ServiceName:              "honeygen-api",
		ServiceVersion:           "test",
		AppEnv:                   "test",
		HTTPAddr:                 ":0",
		InternalEventIngestToken: "test-internal-event-token",
		SQLitePath:               filepath.Join(tempDir, "sqlite", "api.db"),
		GeneratedAssetsDir:       filepath.Join(tempDir, "storage", "generated"),
		StorageRoot:              filepath.Join(tempDir, "storage"),
	}

	application, err := NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp() error = %v", err)
	}
	defer func() {
		if err := application.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	generatedDir := filepath.Join(cfg.StorageRoot, "generated", "northbridge-financial", job.ID, "public")
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	deployment, err := application.DeploymentRepo.Create(context.Background(), deployments.Deployment{
		GenerationJobID: job.ID,
		WorldModelID:    "northbridge-financial",
		Protocol:        "http",
		Port:            0,
		RootPath:        "/",
	})
	if err != nil {
		t.Fatalf("DeploymentRepo.Create() error = %v", err)
	}

	if err := application.DeploymentManager.Start(context.Background(), deployment.ID); err != nil {
		t.Fatalf("DeploymentManager.Start() error = %v", err)
	}
}
