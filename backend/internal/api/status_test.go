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
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/config"
)

func TestStatusEndpointReturnsServiceReadinessAndCounts(t *testing.T) {
	cfg := config.Config{
		ServiceName:        "honeygen-api",
		ServiceVersion:     "test",
		AppEnv:             "test",
		HTTPAddr:           ":0",
		SQLitePath:         filepath.Join(t.TempDir(), "status.db"),
		GeneratedAssetsDir: filepath.Join(t.TempDir(), "generated"),
		StorageRoot:        filepath.Join(t.TempDir(), "storage"),
		Provider: config.ProviderConfig{
			BaseURL: "https://provider.example/v1",
			APIKey:  "test-key",
			Model:   "gpt-4.1-mini",
		},
	}

	application, err := app.NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp() error = %v", err)
	}
	defer application.Close()

	_, err = application.DB.ExecContext(context.Background(), `
		INSERT INTO world_models (id, name, description, json_blob, created_at, updated_at)
		VALUES ('world-1', 'World 1', '', '{}', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');
		INSERT INTO generation_jobs (id, world_model_id, status, started_at, summary_json, created_at, completed_at)
		VALUES ('job-1', 'world-1', 'completed', '2024-01-01T00:01:00Z', '{}', '2024-01-01T00:00:00Z', '2024-01-01T00:05:00Z');
		INSERT INTO assets (id, generation_job_id, world_model_id, source_type, rendered_type, path, mime_type, size_bytes, tags_json, previewable, checksum, created_at)
		VALUES ('asset-1', 'job-1', 'world-1', 'provider', 'pdf', 'C:\temp\a.pdf', 'application/pdf', 123, '[]', 1, 'sum-1', '2024-01-01T00:06:00Z'),
		       ('asset-2', 'job-1', 'world-1', 'provider', 'image', 'C:\temp\a.png', 'image/png', 321, '[]', 1, 'sum-2', '2024-01-01T00:07:00Z');
		INSERT INTO events (id, asset_id, world_model_id, event_type, path, source_ip, user_agent, metadata_json)
		VALUES ('event-recent', 'asset-1', 'world-1', 'asset.viewed', '/generated/a.pdf', '127.0.0.1', 'test-agent', '{}');
		INSERT INTO events (id, asset_id, world_model_id, event_type, path, source_ip, user_agent, metadata_json, timestamp)
		VALUES ('event-old', 'asset-1', 'world-1', 'asset.viewed', '/generated/a.pdf', '127.0.0.1', 'test-agent', '{}', ?);
	`, time.Now().Add(-48*time.Hour).UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("seed data error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Service struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"service"`
		Database struct {
			Ready bool `json:"ready"`
		} `json:"database"`
		Storage struct {
			Root string `json:"root"`
		} `json:"storage"`
		Provider struct {
			Mode  string `json:"mode"`
			Ready bool   `json:"ready"`
		} `json:"provider"`
		Counts struct {
			Assets       int `json:"assets"`
			RecentEvents int `json:"recent_events"`
		} `json:"counts"`
		LatestJob struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			AssetCount int    `json:"asset_count"`
		} `json:"latest_job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.Service.Name != "honeygen-api" {
		t.Fatalf("service.name = %q, want %q", response.Service.Name, "honeygen-api")
	}
	if response.Database.Ready != true {
		t.Fatalf("database.ready = %v, want true", response.Database.Ready)
	}
	if response.Storage.Root != cfg.StorageRoot {
		t.Fatalf("storage.root = %q, want %q", response.Storage.Root, cfg.StorageRoot)
	}
	if response.Provider.Mode != "external" || response.Provider.Ready != true {
		t.Fatalf("provider = %+v, want external ready", response.Provider)
	}
	if response.Counts.Assets != 2 {
		t.Fatalf("counts.assets = %d, want %d", response.Counts.Assets, 2)
	}
	if response.Counts.RecentEvents != 1 {
		t.Fatalf("counts.recent_events = %d, want %d", response.Counts.RecentEvents, 1)
	}
	if response.LatestJob.ID != "job-1" || response.LatestJob.Status != "completed" || response.LatestJob.AssetCount != 2 {
		t.Fatalf("latest_job = %+v, want id=job-1 status=completed asset_count=2", response.LatestJob)
	}
}
