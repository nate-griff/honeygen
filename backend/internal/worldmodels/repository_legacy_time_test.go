package worldmodels

import (
	"context"
	"testing"
)

func TestSQLiteRepositoryGetAcceptsLegacySQLiteTimestamps(t *testing.T) {
	database := newTestDatabase(t)
	repository := NewRepository(database)

	_, err := database.ExecContext(context.Background(), `
		INSERT INTO world_models (id, name, description, json_blob, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "legacy-world", "Legacy World", "legacy record", `{}`, "2026-04-10 21:23:15", "2026-04-10 21:25:15")
	if err != nil {
		t.Fatalf("ExecContext() error = %v", err)
	}

	item, err := repository.Get(context.Background(), "legacy-world")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Fatalf("Get() = %+v, want parsed timestamps", item)
	}
}
