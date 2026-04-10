package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestGenerationJobRecorderRecordProviderFailurePersistsMessage(t *testing.T) {
	t.Parallel()

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
		INSERT INTO generation_jobs (id, world_model_id, status, error_message, created_at, updated_at)
		VALUES ('job-1', 'world-1', 'failed', '', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("seed job data error = %v", err)
	}

	recorder := NewGenerationJobRecorder(database)
	err = recorder.RecordProviderFailure(context.Background(), "job-1", "provider request failed")
	if err != nil {
		t.Fatalf("RecordProviderFailure() error = %v", err)
	}

	var message string
	if err := database.QueryRowContext(context.Background(), `SELECT error_message FROM generation_jobs WHERE id = 'job-1'`).Scan(&message); err != nil {
		t.Fatalf("query error_message error = %v", err)
	}
	if message != "provider request failed" {
		t.Fatalf("error_message = %q, want %q", message, "provider request failed")
	}
}

func TestGenerationJobRecorderRecordProviderFailureReturnsNotFound(t *testing.T) {
	t.Parallel()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer database.Close()

	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	recorder := NewGenerationJobRecorder(database)
	err = recorder.RecordProviderFailure(context.Background(), "missing-job", "provider returned status 502")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("RecordProviderFailure() error = %v, want sql.ErrNoRows", err)
	}
}
