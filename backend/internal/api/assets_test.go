package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/config"
)

func TestAssetContentEndpointDoesNotInlineDOCXPreviewEvenIfMarkedPreviewable(t *testing.T) {
	application := newAssetPreviewTestAPIApp(t)
	router := NewRouter(application)
	seedAssetPreviewGenerationJob(t, application, "world-1", "job-1")

	storedFile, err := application.Storage.Write(context.Background(), "generated/world-1/job-1/intranet/policies/acceptable-use-policy.docx", []byte("<?xml version=\"1.0\"?><w:document>raw xml</w:document>"))
	if err != nil {
		t.Fatalf("Storage.Write() error = %v", err)
	}

	if _, err := application.AssetRepo.Create(context.Background(), assets.Asset{
		ID:              "asset-docx-1",
		GenerationJobID: "job-1",
		WorldModelID:    "world-1",
		SourceType:      "generated",
		RenderedType:    "docx",
		Path:            storedFile.Path,
		MIMEType:        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		SizeBytes:       storedFile.SizeBytes,
		Previewable:     true,
		Checksum:        storedFile.Checksum,
	}); err != nil {
		t.Fatalf("AssetRepo.Create() error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodGet, "/api/assets/asset-docx-1/content", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Previewable bool   `json:"previewable"`
		Content     string `json:"content"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Previewable {
		t.Fatalf("response = %+v, want non-previewable DOCX response", response)
	}
	if response.Content != "" {
		t.Fatalf("response.Content = %q, want empty content for DOCX preview", response.Content)
	}
	if response.Message == "" {
		t.Fatalf("response = %+v, want binary preview message", response)
	}
}

func newAssetPreviewTestAPIApp(t *testing.T) *app.APIApp {
	t.Helper()

	cfg := config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    ":0",
		AdminPassword:               "test-admin-password",
		InternalEventIngestToken:    "test-internal-event-token",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		SQLitePath:                  filepath.Join(t.TempDir(), "api.db"),
		GeneratedAssetsDir:          filepath.Join(t.TempDir(), "generated"),
		StorageRoot:                 filepath.Join(t.TempDir(), "storage"),
	}

	application, err := app.NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp() error = %v", err)
	}

	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	return application
}

func seedAssetPreviewGenerationJob(t *testing.T, application *app.APIApp, worldModelID, jobID string) {
	t.Helper()

	if _, err := application.DB.ExecContext(context.Background(), `
		INSERT INTO world_models (id, name, description, json_blob, created_at, updated_at)
		VALUES (?, 'World 1', '', '{}', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`, worldModelID); err != nil {
		t.Fatalf("seed world model error = %v", err)
	}

	if _, err := application.DB.ExecContext(context.Background(), `
		INSERT INTO generation_jobs (id, world_model_id, status, error_message, created_at, updated_at)
		VALUES (?, ?, 'failed', '', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`, jobID, worldModelID); err != nil {
		t.Fatalf("seed generation job error = %v", err)
	}
}
