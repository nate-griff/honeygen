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
	assertIndexExists(t, db, "idx_deployments_port_unique")
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
	assertIndexExists(t, db, "idx_deployments_port_unique")

	var timestamp string
	if err := db.QueryRowContext(context.Background(), `SELECT timestamp FROM events WHERE id = 'event-1'`).Scan(&timestamp); err != nil {
		t.Fatalf("query migrated timestamp error = %v", err)
	}
	if timestamp != "2024-01-02T03:04:05Z" {
		t.Fatalf("timestamp = %q, want %q", timestamp, "2024-01-02T03:04:05Z")
	}
}

func TestMigrateRebuildsLegacyAssetsTableForCurrentWrites(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	legacySchema := `
CREATE TABLE world_models (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	json_blob TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE generation_jobs (
	id TEXT PRIMARY KEY,
	world_model_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);
CREATE TABLE assets (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	world_model_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	path TEXT NOT NULL,
	mime_type TEXT NOT NULL DEFAULT '',
	byte_size INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (job_id) REFERENCES generation_jobs(id),
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);`
	if _, err := db.ExecContext(context.Background(), legacySchema); err != nil {
		t.Fatalf("create legacy schema error = %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO world_models (id, name, description, json_blob) VALUES ('world-1', 'World 1', '', '{}');
		INSERT INTO generation_jobs (id, world_model_id, status) VALUES ('job-legacy', 'world-1', 'completed');
		INSERT INTO assets (id, job_id, world_model_id, kind, path, mime_type, byte_size)
		VALUES ('asset-legacy', 'job-legacy', 'world-1', 'html', 'legacy.html', 'text/html', 99);
	`); err != nil {
		t.Fatalf("seed legacy schema error = %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	assertColumnMissing(t, db, "assets", "job_id")
	assertColumnMissing(t, db, "assets", "kind")
	assertColumnMissing(t, db, "assets", "byte_size")

	var generationJobID, renderedType string
	var sizeBytes int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT generation_job_id, rendered_type, size_bytes FROM assets WHERE id = 'asset-legacy'`,
	).Scan(&generationJobID, &renderedType, &sizeBytes); err != nil {
		t.Fatalf("query migrated legacy asset error = %v", err)
	}
	if generationJobID != "job-legacy" {
		t.Fatalf("generation_job_id = %q, want %q", generationJobID, "job-legacy")
	}
	if renderedType != "html" {
		t.Fatalf("rendered_type = %q, want %q", renderedType, "html")
	}
	if sizeBytes != 99 {
		t.Fatalf("size_bytes = %d, want %d", sizeBytes, 99)
	}

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO assets (id, generation_job_id, world_model_id, source_type, rendered_type, path, mime_type, size_bytes, tags_json, previewable, checksum)
		VALUES ('asset-new', 'job-legacy', 'world-1', 'generated', 'html', 'new.html', 'text/html', 10, '[]', 1, 'sum-1')
	`); err != nil {
		t.Fatalf("insert current asset shape error = %v", err)
	}
}

func TestMigrateRebuildsLegacyEventsTableForCurrentWrites(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	legacySchema := `
CREATE TABLE world_models (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	json_blob TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE generation_jobs (
	id TEXT PRIMARY KEY,
	world_model_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);
CREATE TABLE assets (
	id TEXT PRIMARY KEY,
	generation_job_id TEXT NOT NULL,
	world_model_id TEXT NOT NULL,
	source_type TEXT NOT NULL,
	rendered_type TEXT NOT NULL,
	path TEXT NOT NULL,
	mime_type TEXT NOT NULL DEFAULT '',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	tags_json TEXT NOT NULL DEFAULT '[]',
	previewable INTEGER NOT NULL DEFAULT 0,
	checksum TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (generation_job_id) REFERENCES generation_jobs(id),
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);
CREATE TABLE events (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	asset_id TEXT,
	level TEXT NOT NULL DEFAULT 'info',
	type TEXT NOT NULL,
	message TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	world_model_id TEXT,
	event_type TEXT NOT NULL DEFAULT '',
	request_path TEXT NOT NULL DEFAULT '',
	remote_addr TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	referrer TEXT NOT NULL DEFAULT '',
	occurred_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	method TEXT NOT NULL DEFAULT '',
	query TEXT NOT NULL DEFAULT '',
	status_code INTEGER NOT NULL DEFAULT 0,
	bytes_sent INTEGER NOT NULL DEFAULT 0,
	path TEXT NOT NULL DEFAULT '',
	source_ip TEXT NOT NULL DEFAULT '',
	referer TEXT NOT NULL DEFAULT '',
	timestamp TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	if _, err := db.ExecContext(context.Background(), legacySchema); err != nil {
		t.Fatalf("create legacy schema error = %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO world_models (id, name, description, json_blob) VALUES ('world-1', 'World 1', '', '{}');
		INSERT INTO generation_jobs (id, world_model_id, status) VALUES ('job-legacy', 'world-1', 'completed');
		INSERT INTO assets (id, generation_job_id, world_model_id, source_type, rendered_type, path, mime_type, size_bytes, tags_json, previewable, checksum)
		VALUES ('asset-legacy', 'job-legacy', 'world-1', 'generated', 'html', 'legacy.html', 'text/html', 99, '[]', 1, 'sum-1');
		INSERT INTO events (id, job_id, asset_id, level, type, message, metadata_json, created_at, world_model_id, request_path, remote_addr, user_agent, referrer, occurred_at, method, query, status_code, bytes_sent)
		VALUES ('event-legacy', 'job-legacy', 'asset-legacy', 'info', 'asset.viewed', 'legacy message', '{\"source\":\"legacy\"}', '2024-01-02 03:04:05', 'world-1', '/generated/legacy.html', '203.0.113.10', 'legacy-agent', 'https://legacy.example', '2024-01-02 03:04:05', 'GET', 'download=1', 200, 512);
	`); err != nil {
		t.Fatalf("seed legacy schema error = %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	for _, column := range []string{"job_id", "type", "message", "request_path", "remote_addr", "referrer", "occurred_at"} {
		assertColumnMissing(t, db, "events", column)
	}

	var eventType, path, sourceIP, referer, timestamp string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT event_type, path, source_ip, referer, timestamp FROM events WHERE id = 'event-legacy'`,
	).Scan(&eventType, &path, &sourceIP, &referer, &timestamp); err != nil {
		t.Fatalf("query migrated legacy event error = %v", err)
	}
	if eventType != "asset.viewed" {
		t.Fatalf("event_type = %q, want %q", eventType, "asset.viewed")
	}
	if path != "/generated/legacy.html" {
		t.Fatalf("path = %q, want %q", path, "/generated/legacy.html")
	}
	if sourceIP != "203.0.113.10" {
		t.Fatalf("source_ip = %q, want %q", sourceIP, "203.0.113.10")
	}
	if referer != "https://legacy.example" {
		t.Fatalf("referer = %q, want %q", referer, "https://legacy.example")
	}
	if timestamp != "2024-01-02 03:04:05" {
		t.Fatalf("timestamp = %q, want %q", timestamp, "2024-01-02 03:04:05")
	}

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO events (id, asset_id, world_model_id, event_type, method, query, path, source_ip, user_agent, referer, status_code, bytes_sent, timestamp, level, metadata_json)
		VALUES ('event-new', 'asset-legacy', 'world-1', 'asset.viewed', 'GET', 'download=1', '/generated/legacy.html', '203.0.113.11', 'new-agent', 'https://new.example', 200, 256, '2024-01-02T03:05:05Z', 'info', '{}')
	`); err != nil {
		t.Fatalf("insert current event shape error = %v", err)
	}
}

func TestMigrateCreatesIPIntelCacheTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var name string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'ip_intel_cache'`,
	).Scan(&name); err != nil {
		t.Fatalf("expected ip_intel_cache table to exist, error = %v", err)
	}
	if name != "ip_intel_cache" {
		t.Fatalf("table name = %q, want %q", name, "ip_intel_cache")
	}

	assertColumnExists(t, db, "ip_intel_cache", "ip")
	assertColumnExists(t, db, "ip_intel_cache", "status")
	assertColumnExists(t, db, "ip_intel_cache", "payload_json")
	assertColumnExists(t, db, "ip_intel_cache", "enriched_at")
	assertColumnExists(t, db, "ip_intel_cache", "created_at")
	assertColumnExists(t, db, "ip_intel_cache", "updated_at")
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

func assertColumnMissing(t *testing.T, db *sql.DB, table, column string) {
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
			t.Fatalf("did not expect column %q in table %q", column, table)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}
}
