package events

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/assets"
)

const InternalIngestTokenHeader = "X-Honeygen-Internal-Event-Token"

type IngestRequest struct {
	Timestamp  time.Time      `json:"timestamp"`
	EventType  string         `json:"event_type,omitempty"`
	Method     string         `json:"method"`
	Path       string         `json:"path"`
	Query      string         `json:"query"`
	SourceIP   string         `json:"source_ip"`
	UserAgent  string         `json:"user_agent"`
	Referer    string         `json:"referer"`
	StatusCode int            `json:"status_code"`
	BytesSent  int            `json:"bytes_sent"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type assetFinder interface {
	FindByPath(context.Context, string) (assets.Asset, error)
}

type Service struct {
	repository *Repository
	assets     assetFinder
	now        func() time.Time
}

func NewService(repository *Repository, assets assetFinder) *Service {
	return &Service{
		repository: repository,
		assets:     assets,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) Ingest(ctx context.Context, request IngestRequest) (Event, error) {
	if s == nil || s.repository == nil {
		return Event{}, fmt.Errorf("event repository is not configured")
	}

	method := strings.TrimSpace(request.Method)
	if method == "" {
		return Event{}, ValidationError{Message: "method is required"}
	}

	normalizedPath := normalizeRequestPath(request.Path)
	if normalizedPath == "" {
		return Event{}, ValidationError{Message: "path is required"}
	}
	if request.StatusCode < 0 {
		return Event{}, ValidationError{Message: "status_code must be zero or greater"}
	}
	if request.BytesSent < 0 {
		return Event{}, ValidationError{Message: "bytes_sent must be zero or greater"}
	}

	event := Event{
		EventType:  strings.TrimSpace(request.EventType),
		Method:     method,
		Query:      strings.TrimSpace(request.Query),
		Path:       normalizedPath,
		SourceIP:   strings.TrimSpace(request.SourceIP),
		UserAgent:  strings.TrimSpace(request.UserAgent),
		Referer:    strings.TrimSpace(request.Referer),
		StatusCode: request.StatusCode,
		BytesSent:  request.BytesSent,
		Timestamp:  request.Timestamp.UTC(),
		Level:      "info",
		Metadata:   request.Metadata,
	}
	if event.EventType == "" {
		event.EventType = "http_request"
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = s.now()
	}

	if s.assets != nil {
		assetPath := strings.TrimPrefix(normalizedPath, "/")
		asset, err := s.assets.FindByPath(ctx, assetPath)
		if err == nil {
			event.AssetID = asset.ID
			event.WorldModelID = asset.WorldModelID
		} else if !errors.Is(err, assets.ErrNotFound) {
			return Event{}, err
		}
	}
	if event.WorldModelID == "" {
		event.WorldModelID = worldModelIDFromPath(normalizedPath)
	}

	return s.repository.Create(ctx, event)
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Event, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("event repository is not configured")
	}
	return s.repository.List(ctx, options)
}

func (s *Service) Get(ctx context.Context, id string) (Event, error) {
	if s == nil || s.repository == nil {
		return Event{}, fmt.Errorf("event repository is not configured")
	}
	return s.repository.Get(ctx, id)
}

func normalizeRequestPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func worldModelIDFromPath(raw string) string {
	segments := strings.Split(strings.Trim(normalizeRequestPath(raw), "/"), "/")
	if len(segments) < 2 || segments[0] != "generated" {
		return ""
	}
	return segments[1]
}
