package ipintel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrCacheMiss is returned when no cached entry exists for an IP.
var ErrCacheMiss = errors.New("ip intel cache miss")

const cacheTTL = 7 * 24 * time.Hour

// Cache persists enrichment results keyed by IP address in SQLite.
type Cache struct {
	db *sql.DB
}

// NewCache returns a new Cache backed by the given database.
func NewCache(db *sql.DB) *Cache {
	return &Cache{db: db}
}

// Get returns the cached IPIntelResult for ip, or ErrCacheMiss if absent.
func (c *Cache) Get(ctx context.Context, ip string) (IPIntelResult, error) {
	var payloadJSON string
	var enrichedAt string
	err := c.db.QueryRowContext(ctx,
		`SELECT payload_json, enriched_at FROM ip_intel_cache WHERE ip = ?`, ip,
	).Scan(&payloadJSON, &enrichedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return IPIntelResult{}, ErrCacheMiss
	}
	if err != nil {
		return IPIntelResult{}, fmt.Errorf("get ip intel cache for %q: %w", ip, err)
	}
	parsedEnrichedAt, err := time.Parse(time.RFC3339, enrichedAt)
	if err != nil {
		return IPIntelResult{}, fmt.Errorf("parse ip intel cache timestamp for %q: %w", ip, err)
	}
	if time.Since(parsedEnrichedAt) > cacheTTL {
		return IPIntelResult{}, ErrCacheMiss
	}
	var result IPIntelResult
	if err := json.Unmarshal([]byte(payloadJSON), &result); err != nil {
		return IPIntelResult{}, fmt.Errorf("decode ip intel cache payload for %q: %w", ip, err)
	}
	return result, nil
}

// Set stores result in the cache, overwriting any existing entry.
func (c *Cache) Set(ctx context.Context, result IPIntelResult) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if result.CachedAt == "" {
		result.CachedAt = now
	}
	payloadJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode ip intel result for %q: %w", result.IP, err)
	}
	_, err = c.db.ExecContext(ctx, `
		INSERT INTO ip_intel_cache (ip, status, payload_json, enriched_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET
			status      = excluded.status,
			payload_json = excluded.payload_json,
			enriched_at = excluded.enriched_at,
			updated_at  = excluded.updated_at
	`, result.IP, result.Status, string(payloadJSON), now, now)
	if err != nil {
		return fmt.Errorf("set ip intel cache for %q: %w", result.IP, err)
	}
	return nil
}
