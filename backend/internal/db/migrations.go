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
	{"events", "timestamp", `ALTER TABLE events ADD COLUMN timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))`},
}

var schemaIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_generation_jobs_created_at ON generation_jobs(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_assets_generation_job_id ON assets(generation_job_id)`,
	`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC)`,
}

func Migrate(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
	}

	if err := backfillLegacyData(ctx, tx); err != nil {
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

func backfillLegacyData(ctx context.Context, tx *sql.Tx) error {
	if hasColumn(ctx, tx, "assets", "job_id") {
		if _, err := tx.ExecContext(ctx, `UPDATE assets SET generation_job_id = job_id WHERE generation_job_id = ''`); err != nil {
			return fmt.Errorf("backfill assets.generation_job_id: %w", err)
		}
	}
	if hasColumn(ctx, tx, "assets", "kind") {
		if _, err := tx.ExecContext(ctx, `UPDATE assets SET source_type = kind WHERE source_type = ''`); err != nil {
			return fmt.Errorf("backfill assets.source_type: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE assets SET rendered_type = kind WHERE rendered_type = ''`); err != nil {
			return fmt.Errorf("backfill assets.rendered_type: %w", err)
		}
	}
	if hasColumn(ctx, tx, "assets", "byte_size") {
		if _, err := tx.ExecContext(ctx, `UPDATE assets SET size_bytes = byte_size WHERE size_bytes = 0`); err != nil {
			return fmt.Errorf("backfill assets.size_bytes: %w", err)
		}
	}
	if hasColumn(ctx, tx, "events", "type") {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET event_type = type WHERE event_type = ''`); err != nil {
			return fmt.Errorf("backfill events.event_type: %w", err)
		}
	}
	if hasColumn(ctx, tx, "events", "request_path") {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET path = request_path WHERE path = ''`); err != nil {
			return fmt.Errorf("backfill events.path: %w", err)
		}
	}
	if hasColumn(ctx, tx, "events", "remote_addr") {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET source_ip = remote_addr WHERE source_ip = ''`); err != nil {
			return fmt.Errorf("backfill events.source_ip: %w", err)
		}
	}
	if hasColumn(ctx, tx, "events", "referrer") {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET referer = referrer WHERE referer = ''`); err != nil {
			return fmt.Errorf("backfill events.referer: %w", err)
		}
	}
	if hasColumn(ctx, tx, "events", "occurred_at") {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET timestamp = occurred_at WHERE timestamp = '' OR timestamp IS NULL`); err != nil {
			return fmt.Errorf("backfill events.timestamp from occurred_at: %w", err)
		}
	}
	if hasColumn(ctx, tx, "events", "created_at") {
		if _, err := tx.ExecContext(ctx, `UPDATE events SET timestamp = created_at WHERE timestamp = '' OR timestamp IS NULL`); err != nil {
			return fmt.Errorf("backfill events.timestamp from created_at: %w", err)
		}
	}
	if hasColumn(ctx, tx, "world_models", "prompt") {
		if _, err := tx.ExecContext(ctx, `UPDATE world_models SET json_blob = '{}' WHERE json_blob = ''`); err != nil {
			return fmt.Errorf("backfill world_models.json_blob: %w", err)
		}
	}
	return nil
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) bool {
	exists, err := columnExists(ctx, tx, table, column)
	return err == nil && exists
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
