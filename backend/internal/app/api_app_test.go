package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/natet/honeygen/backend/internal/config"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/deployments"
)

func TestNewAPIAppStartsDeploymentFromStorageRoot(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    ":0",
		AdminPassword:               "test-admin-password",
		InternalEventIngestToken:    "test-internal-event-token",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		SQLitePath:                  filepath.Join(tempDir, "sqlite", "api.db"),
		GeneratedAssetsDir:          filepath.Join(tempDir, "storage", "generated"),
		StorageRoot:                 filepath.Join(tempDir, "storage"),
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

func TestProviderConfigCodecRoundTrip(t *testing.T) {
	codec, err := newProviderConfigCodec("test-provider-config-encryption-key")
	if err != nil {
		t.Fatalf("newProviderConfigCodec() error = %v", err)
	}

	ciphertext, err := codec.EncryptString("super-secret-api-key")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	if ciphertext == "super-secret-api-key" {
		t.Fatal("EncryptString() returned plaintext ciphertext")
	}

	plaintext, err := codec.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("DecryptString() error = %v", err)
	}
	if plaintext != "super-secret-api-key" {
		t.Fatalf("DecryptString() = %q, want %q", plaintext, "super-secret-api-key")
	}
}

func TestLoadSavedProviderConfigDecryptsEncryptedAPIKey(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    ":0",
		AdminPassword:               "test-admin-password",
		InternalEventIngestToken:    "test-internal-event-token",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		SQLitePath:                  filepath.Join(tempDir, "sqlite", "api.db"),
		GeneratedAssetsDir:          filepath.Join(tempDir, "storage", "generated"),
		StorageRoot:                 filepath.Join(tempDir, "storage"),
	}

	application, err := NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp() error = %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	database, err := appdb.OpenSQLite(context.Background(), cfg.SQLitePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer database.Close()
	if err := appdb.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := appdb.NewSettingsStore(database)
	codec, err := newProviderConfigCodec(cfg.ProviderConfigEncryptionKey)
	if err != nil {
		t.Fatalf("newProviderConfigCodec() error = %v", err)
	}
	encryptedAPIKey, err := codec.EncryptString("persisted-api-key")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	if err := store.Put(context.Background(), "provider", []byte(`{"base_url":"https://provider.example/v1","encrypted_api_key":"`+encryptedAPIKey+`","model":"gpt-4.1-mini"}`)); err != nil {
		t.Fatalf("store.Put() error = %v", err)
	}
	_ = database.Close()

	reloaded, err := NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp(reloaded) error = %v", err)
	}
	defer func() {
		if err := reloaded.Close(); err != nil {
			t.Fatalf("reloaded.Close() error = %v", err)
		}
	}()

	if reloaded.Config.Provider.APIKey != "persisted-api-key" {
		t.Fatalf("reloaded.Config.Provider.APIKey = %q, want %q", reloaded.Config.Provider.APIKey, "persisted-api-key")
	}
	if reloaded.Config.Provider.BaseURL != "https://provider.example/v1" {
		t.Fatalf("reloaded.Config.Provider.BaseURL = %q, want %q", reloaded.Config.Provider.BaseURL, "https://provider.example/v1")
	}
}
