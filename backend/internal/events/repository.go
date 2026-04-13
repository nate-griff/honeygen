package events

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appdb "github.com/natet/honeygen/backend/internal/db"
)

var ErrNotFound = errors.New("event not found")

type Event struct {
	ID           string         `json:"id"`
	AssetID      string         `json:"asset_id,omitempty"`
	WorldModelID string         `json:"world_model_id,omitempty"`
	EventType    string         `json:"event_type"`
	Method       string         `json:"method"`
	Query        string         `json:"query"`
	Path         string         `json:"path"`
	SourceIP     string         `json:"source_ip"`
	UserAgent    string         `json:"user_agent"`
	Referer      string         `json:"referer"`
	StatusCode   int            `json:"status_code"`
	BytesSent    int            `json:"bytes_sent"`
	Timestamp    time.Time      `json:"timestamp"`
	Level        string         `json:"level"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type ListOptions struct {
	Limit        int
	Offset       int
	WorldModelID string
	Path         string
	SourceIP     string
	StatusCode   int
}

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Create(ctx context.Context, event Event) (Event, error) {
	if event.ID == "" {
		event.ID = newEventID()
	}
	if event.EventType == "" {
		event.EventType = "http_request"
	}
	if event.Level == "" {
		event.Level = "info"
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return Event{}, fmt.Errorf("encode event metadata: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO events (
			id, asset_id, world_model_id, event_type, method, query, path,
			source_ip, user_agent, referer, status_code, bytes_sent, timestamp,
			level, metadata_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, nullableString(event.AssetID), nullableString(event.WorldModelID), event.EventType, event.Method, event.Query, event.Path, event.SourceIP, event.UserAgent, event.Referer, event.StatusCode, event.BytesSent, event.Timestamp.UTC().Format(time.RFC3339), event.Level, string(metadataJSON)); err != nil {
		return Event{}, fmt.Errorf("create event %q: %w", event.ID, err)
	}

	return r.Get(ctx, event.ID)
}

func (r *Repository) Get(ctx context.Context, id string) (Event, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, asset_id, world_model_id, event_type, method, query, path,
		       source_ip, user_agent, referer, status_code, bytes_sent, timestamp,
		       level, metadata_json, created_at
		FROM events
		WHERE id = ?
	`, id)

	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("get event %q: %w", id, err)
	}
	return event, nil
}

func (r *Repository) List(ctx context.Context, options ListOptions) ([]Event, error) {
	query := `
		SELECT id, asset_id, world_model_id, event_type, method, query, path,
		       source_ip, user_agent, referer, status_code, bytes_sent, timestamp,
		       level, metadata_json, created_at
		FROM events
	`
	var (
		conditions []string
		args       []any
	)
	if strings.TrimSpace(options.Path) != "" {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, "%"+strings.TrimSpace(options.Path)+"%")
	}
	if strings.TrimSpace(options.WorldModelID) != "" {
		conditions = append(conditions, "world_model_id = ?")
		args = append(args, strings.TrimSpace(options.WorldModelID))
	}
	if strings.TrimSpace(options.SourceIP) != "" {
		conditions = append(conditions, "source_ip = ?")
		args = append(args, strings.TrimSpace(options.SourceIP))
	}
	if options.StatusCode > 0 {
		conditions = append(conditions, "status_code = ?")
		args = append(args, options.StatusCode)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY datetime(timestamp) DESC, datetime(created_at) DESC LIMIT %d OFFSET %d", normalizeLimit(options.Limit), normalizeOffset(options.Offset))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	items := []Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return items, nil
}

type eventRowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventRowScanner) (Event, error) {
	var (
		item         Event
		assetID      sql.NullString
		worldModelID sql.NullString
		timestampRaw string
		createdAtRaw string
		metadataJSON string
	)

	err := scanner.Scan(
		&item.ID,
		&assetID,
		&worldModelID,
		&item.EventType,
		&item.Method,
		&item.Query,
		&item.Path,
		&item.SourceIP,
		&item.UserAgent,
		&item.Referer,
		&item.StatusCode,
		&item.BytesSent,
		&timestampRaw,
		&item.Level,
		&metadataJSON,
		&createdAtRaw,
	)
	if err != nil {
		return Event{}, err
	}

	item.AssetID = assetID.String
	item.WorldModelID = worldModelID.String
	if metadataJSON != "" && metadataJSON != "null" {
		if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
			return Event{}, fmt.Errorf("decode event metadata for %q: %w", item.ID, err)
		}
	}
	item.Timestamp, err = appdb.ParseTimestamp(timestampRaw)
	if err != nil {
		return Event{}, fmt.Errorf("parse event timestamp %q: %w", timestampRaw, err)
	}
	item.CreatedAt, err = appdb.ParseTimestamp(createdAtRaw)
	if err != nil {
		return Event{}, fmt.Errorf("parse event created_at %q: %w", createdAtRaw, err)
	}
	return item, nil
}

func newEventID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return "event_" + hex.EncodeToString(buf)
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 1000 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
