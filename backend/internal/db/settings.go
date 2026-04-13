package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(database *sql.DB) *SettingsStore {
	return &SettingsStore{db: database}
}

func (s *SettingsStore) Get(ctx context.Context, key string) (json.RawMessage, error) {
	var valueJSON string
	err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, key).Scan(&valueJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get setting %q: %w", key, err)
	}
	return json.RawMessage(valueJSON), nil
}

func (s *SettingsStore) Put(ctx context.Context, key string, value json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value_json, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at
	`, key, string(value))
	if err != nil {
		return fmt.Errorf("put setting %q: %w", key, err)
	}
	return nil
}
