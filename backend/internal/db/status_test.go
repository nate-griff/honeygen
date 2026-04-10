package db

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestStatusQueriesReadStatusSummary(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer database.Close()

	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	_, err = database.ExecContext(context.Background(), `
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

	queries := NewStatusQueries(database)
	summary, err := queries.ReadStatusSummary(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("ReadStatusSummary() error = %v", err)
	}

	if summary.Counts.Assets != 2 {
		t.Fatalf("counts.assets = %d, want %d", summary.Counts.Assets, 2)
	}
	if summary.Counts.RecentEvents != 1 {
		t.Fatalf("counts.recent_events = %d, want %d", summary.Counts.RecentEvents, 1)
	}
	if summary.LatestJob == nil {
		t.Fatal("latest job = nil, want job summary")
	}
	if summary.LatestJob.ID != "job-1" || summary.LatestJob.Status != "completed" || summary.LatestJob.AssetCount != 2 {
		t.Fatalf("latest job = %+v, want id=job-1 status=completed asset_count=2", summary.LatestJob)
	}
}
