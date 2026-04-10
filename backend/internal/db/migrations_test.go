package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestMigrateCreatesCoreTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	for _, table := range []string{"world_models", "generation_jobs", "assets", "events", "settings"} {
		var name string
		err := db.QueryRowContext(
			context.Background(),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %q to exist, query error = %v", table, err)
		}
		if name != table {
			t.Fatalf("table name = %q, want %q", name, table)
		}
	}
}

func TestMigrateCreatesSpecColumns(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	assertColumnExists(t, db, "world_models", "json_blob")
	assertColumnExists(t, db, "generation_jobs", "started_at")
	assertColumnExists(t, db, "generation_jobs", "summary_json")
	assertColumnExists(t, db, "assets", "generation_job_id")
	assertColumnExists(t, db, "assets", "rendered_type")
	assertColumnExists(t, db, "assets", "size_bytes")
	assertColumnExists(t, db, "assets", "tags_json")
	assertColumnExists(t, db, "assets", "previewable")
	assertColumnExists(t, db, "assets", "checksum")
	assertColumnExists(t, db, "events", "event_type")
	assertColumnExists(t, db, "events", "method")
	assertColumnExists(t, db, "events", "query")
	assertColumnExists(t, db, "events", "path")
	assertColumnExists(t, db, "events", "source_ip")
	assertColumnExists(t, db, "events", "referer")
	assertColumnExists(t, db, "events", "status_code")
	assertColumnExists(t, db, "events", "bytes_sent")
	assertColumnExists(t, db, "events", "timestamp")
	assertIndexExists(t, db, "idx_events_source_ip")
}

func TestMigrateUpgradesExistingMVPSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	existingSchema := `
CREATE TABLE world_models (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE generation_jobs (
	id TEXT PRIMARY KEY,
	world_model_id TEXT NOT NULL,
	status TEXT NOT NULL,
	provider_job_id TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at TEXT
);
CREATE TABLE assets (
	id TEXT PRIMARY KEY,
	generation_job_id TEXT NOT NULL,
	world_model_id TEXT NOT NULL,
	path TEXT NOT NULL,
	mime_type TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE events (
	id TEXT PRIMARY KEY,
	asset_id TEXT,
	level TEXT NOT NULL DEFAULT 'info',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE settings (
	key TEXT PRIMARY KEY,
	value_json TEXT NOT NULL,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := db.ExecContext(context.Background(), existingSchema); err != nil {
		t.Fatalf("create existing schema error = %v", err)
	}
	if _, err := db.ExecContext(
		context.Background(),
		`INSERT INTO events (id, asset_id, level, metadata_json, created_at) VALUES ('event-1', NULL, 'info', '{}', '2024-01-02T03:04:05Z')`,
	); err != nil {
		t.Fatalf("insert existing event error = %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	assertColumnExists(t, db, "world_models", "json_blob")
	assertColumnExists(t, db, "generation_jobs", "started_at")
	assertColumnExists(t, db, "generation_jobs", "summary_json")
	assertColumnExists(t, db, "assets", "generation_job_id")
	assertColumnExists(t, db, "assets", "source_type")
	assertColumnExists(t, db, "assets", "rendered_type")
	assertColumnExists(t, db, "assets", "size_bytes")
	assertColumnExists(t, db, "events", "timestamp")
	assertColumnExists(t, db, "events", "event_type")
	assertColumnExists(t, db, "events", "path")
	assertIndexExists(t, db, "idx_events_source_ip")

	var timestamp string
	if err := db.QueryRowContext(context.Background(), `SELECT timestamp FROM events WHERE id = 'event-1'`).Scan(&timestamp); err != nil {
		t.Fatalf("query migrated timestamp error = %v", err)
	}
	if timestamp != "2024-01-02T03:04:05Z" {
		t.Fatalf("timestamp = %q, want %q", timestamp, "2024-01-02T03:04:05Z")
	}
}

func assertIndexExists(t *testing.T, db *sql.DB, index string) {
	t.Helper()

	var name string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`,
		index,
	).Scan(&name); err != nil {
		t.Fatalf("expected index %q to exist, query error = %v", index, err)
	}
	if name != index {
		t.Fatalf("index name = %q, want %q", name, index)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s) error = %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultV, &primaryKey); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		if name == column {
			return
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}

	t.Fatalf("expected column %q in table %q", column, table)
}
