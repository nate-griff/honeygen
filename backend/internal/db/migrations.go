package db

import (
	"context"
	"database/sql"
	"fmt"
)

const baseSchemaSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS world_models (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	json_blob TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS generation_jobs (
	id TEXT PRIMARY KEY,
	world_model_id TEXT NOT NULL,
	status TEXT NOT NULL,
	provider_job_id TEXT NOT NULL DEFAULT '',
	started_at TEXT,
	summary_json TEXT NOT NULL DEFAULT '{}',
	error_message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	completed_at TEXT,
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);

CREATE TABLE IF NOT EXISTS assets (
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
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	FOREIGN KEY (generation_job_id) REFERENCES generation_jobs(id),
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);

CREATE TABLE IF NOT EXISTS events (
	id TEXT PRIMARY KEY,
	asset_id TEXT,
	world_model_id TEXT,
	event_type TEXT NOT NULL,
	method TEXT NOT NULL DEFAULT '',
	query TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL DEFAULT '',
	source_ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	referer TEXT NOT NULL DEFAULT '',
	status_code INTEGER NOT NULL DEFAULT 0,
	bytes_sent INTEGER NOT NULL DEFAULT 0,
	timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	level TEXT NOT NULL DEFAULT 'info',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	FOREIGN KEY (asset_id) REFERENCES assets(id)
);

CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value_json TEXT NOT NULL,
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS deployments (
	id TEXT PRIMARY KEY,
	generation_job_id TEXT NOT NULL,
	world_model_id TEXT NOT NULL,
	protocol TEXT NOT NULL DEFAULT 'http',
	port INTEGER NOT NULL,
	root_path TEXT NOT NULL DEFAULT '/',
	status TEXT NOT NULL DEFAULT 'stopped',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	FOREIGN KEY (generation_job_id) REFERENCES generation_jobs(id),
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
);

CREATE TABLE IF NOT EXISTS ip_intel_cache (
	ip TEXT PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'pending',
	payload_json TEXT NOT NULL DEFAULT '{}',
	enriched_at TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
`

var schemaUpgrades = []struct {
	table  string
	column string
	sql    string
}{
	{"world_models", "json_blob", `ALTER TABLE world_models ADD COLUMN json_blob TEXT NOT NULL DEFAULT '{}'`},
	{"generation_jobs", "started_at", `ALTER TABLE generation_jobs ADD COLUMN started_at TEXT`},
	{"generation_jobs", "summary_json", `ALTER TABLE generation_jobs ADD COLUMN summary_json TEXT NOT NULL DEFAULT '{}'`},
	{"assets", "generation_job_id", `ALTER TABLE assets ADD COLUMN generation_job_id TEXT NOT NULL DEFAULT ''`},
	{"assets", "source_type", `ALTER TABLE assets ADD COLUMN source_type TEXT NOT NULL DEFAULT ''`},
	{"assets", "rendered_type", `ALTER TABLE assets ADD COLUMN rendered_type TEXT NOT NULL DEFAULT ''`},
	{"assets", "size_bytes", `ALTER TABLE assets ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0`},
	{"assets", "tags_json", `ALTER TABLE assets ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]'`},
	{"assets", "previewable", `ALTER TABLE assets ADD COLUMN previewable INTEGER NOT NULL DEFAULT 0`},
	{"assets", "checksum", `ALTER TABLE assets ADD COLUMN checksum TEXT NOT NULL DEFAULT ''`},
	{"events", "world_model_id", `ALTER TABLE events ADD COLUMN world_model_id TEXT`},
	{"events", "event_type", `ALTER TABLE events ADD COLUMN event_type TEXT NOT NULL DEFAULT ''`},
	{"events", "method", `ALTER TABLE events ADD COLUMN method TEXT NOT NULL DEFAULT ''`},
	{"events", "query", `ALTER TABLE events ADD COLUMN query TEXT NOT NULL DEFAULT ''`},
	{"events", "path", `ALTER TABLE events ADD COLUMN path TEXT NOT NULL DEFAULT ''`},
	{"events", "source_ip", `ALTER TABLE events ADD COLUMN source_ip TEXT NOT NULL DEFAULT ''`},
	{"events", "user_agent", `ALTER TABLE events ADD COLUMN user_agent TEXT NOT NULL DEFAULT ''`},
	{"events", "referer", `ALTER TABLE events ADD COLUMN referer TEXT NOT NULL DEFAULT ''`},
	{"events", "status_code", `ALTER TABLE events ADD COLUMN status_code INTEGER NOT NULL DEFAULT 0`},
	{"events", "bytes_sent", `ALTER TABLE events ADD COLUMN bytes_sent INTEGER NOT NULL DEFAULT 0`},
	{"events", "timestamp", `ALTER TABLE events ADD COLUMN timestamp TEXT`},
}

var schemaIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_generation_jobs_created_at ON generation_jobs(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_generation_job_id ON assets(generation_job_id)`,
	`CREATE INDEX IF NOT EXISTS idx_events_world_model_id ON events(world_model_id)`,
	`CREATE INDEX IF NOT EXISTS idx_events_source_ip ON events(source_ip)`,
	`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_deployments_port_unique ON deployments(port)`,
}

func Migrate(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	eventTimestampAdded := false

	if _, err := tx.ExecContext(ctx, baseSchemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	for _, upgrade := range schemaUpgrades {
		exists, err := columnExists(ctx, tx, upgrade.table, upgrade.column)
		if err != nil {
			return fmt.Errorf("inspect %s.%s: %w", upgrade.table, upgrade.column, err)
		}
		if exists {
			continue
		}
		if _, err := tx.ExecContext(ctx, upgrade.sql); err != nil {
			return fmt.Errorf("apply schema upgrade %s.%s: %w", upgrade.table, upgrade.column, err)
		}
		if upgrade.table == "events" && upgrade.column == "timestamp" {
			eventTimestampAdded = true
		}
	}

	if eventTimestampAdded {
		createdAtExists, err := columnExists(ctx, tx, "events", "created_at")
		if err != nil {
			return fmt.Errorf("inspect events.created_at: %w", err)
		}
		if createdAtExists {
			if _, err := tx.ExecContext(ctx, `UPDATE events SET timestamp = created_at WHERE timestamp IS NULL OR timestamp = ''`); err != nil {
				return fmt.Errorf("backfill events.timestamp from created_at: %w", err)
			}
		}
	}

	if err := migrateLegacyAssetsTable(ctx, tx); err != nil {
		return err
	}
	if err := migrateLegacyEventsTable(ctx, tx); err != nil {
		return err
	}

	for _, statement := range schemaIndexes {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply index: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES ('2026-04-10-backend-foundation')`); err != nil {
		return fmt.Errorf("record schema migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}

	return nil
}

func migrateLegacyEventsTable(ctx context.Context, tx *sql.Tx) error {
	legacyJobIDExists, err := columnExists(ctx, tx, "events", "job_id")
	if err != nil {
		return fmt.Errorf("inspect events.job_id: %w", err)
	}
	if !legacyJobIDExists {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE events_rebuilt (
	id TEXT PRIMARY KEY,
	asset_id TEXT,
	world_model_id TEXT,
	event_type TEXT NOT NULL,
	method TEXT NOT NULL DEFAULT '',
	query TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL DEFAULT '',
	source_ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	referer TEXT NOT NULL DEFAULT '',
	status_code INTEGER NOT NULL DEFAULT 0,
	bytes_sent INTEGER NOT NULL DEFAULT 0,
	timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	level TEXT NOT NULL DEFAULT 'info',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	FOREIGN KEY (asset_id) REFERENCES assets(id)
)`); err != nil {
		return fmt.Errorf("create rebuilt events table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO events_rebuilt (
	id,
	asset_id,
	world_model_id,
	event_type,
	method,
	query,
	path,
	source_ip,
	user_agent,
	referer,
	status_code,
	bytes_sent,
	timestamp,
	level,
	metadata_json,
	created_at
)
SELECT
	id,
	asset_id,
	world_model_id,
	COALESCE(NULLIF(event_type, ''), type),
	method,
	query,
	COALESCE(NULLIF(path, ''), request_path),
	COALESCE(NULLIF(source_ip, ''), remote_addr),
	user_agent,
	COALESCE(NULLIF(referer, ''), referrer),
	status_code,
	bytes_sent,
	COALESCE(NULLIF(occurred_at, ''), NULLIF(timestamp, ''), created_at),
	level,
	metadata_json,
	created_at
FROM events
`); err != nil {
		return fmt.Errorf("copy legacy events into rebuilt table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE events`); err != nil {
		return fmt.Errorf("drop legacy events table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE events_rebuilt RENAME TO events`); err != nil {
		return fmt.Errorf("rename rebuilt events table: %w", err)
	}

	return nil
}

func migrateLegacyAssetsTable(ctx context.Context, tx *sql.Tx) error {
	legacyJobIDExists, err := columnExists(ctx, tx, "assets", "job_id")
	if err != nil {
		return fmt.Errorf("inspect assets.job_id: %w", err)
	}
	if !legacyJobIDExists {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE assets_rebuilt (
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
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	FOREIGN KEY (generation_job_id) REFERENCES generation_jobs(id),
	FOREIGN KEY (world_model_id) REFERENCES world_models(id)
)`); err != nil {
		return fmt.Errorf("create rebuilt assets table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO assets_rebuilt (
	id,
	generation_job_id,
	world_model_id,
	source_type,
	rendered_type,
	path,
	mime_type,
	size_bytes,
	tags_json,
	previewable,
	checksum,
	created_at
)
SELECT
	id,
	COALESCE(NULLIF(generation_job_id, ''), job_id),
	world_model_id,
	COALESCE(NULLIF(source_type, ''), 'generated'),
	COALESCE(NULLIF(rendered_type, ''), kind),
	path,
	mime_type,
	CASE WHEN size_bytes > 0 THEN size_bytes ELSE byte_size END,
	tags_json,
	previewable,
	checksum,
	created_at
FROM assets
`); err != nil {
		return fmt.Errorf("copy legacy assets into rebuilt table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE assets`); err != nil {
		return fmt.Errorf("drop legacy assets table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE assets_rebuilt RENAME TO assets`); err != nil {
		return fmt.Errorf("rename rebuilt assets table: %w", err)
	}

	return nil
}

func columnExists(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultVal, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}

	return false, rows.Err()
}
