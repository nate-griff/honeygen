package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Recorder interface {
	Record(context.Context, IngestRequest) error
}

type HTTPRecorder struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPRecorder(baseURL, token string, client *http.Client) Recorder {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return &HTTPRecorder{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  client,
	}
}

func (r *HTTPRecorder) Record(ctx context.Context, payload IngestRequest) error {
	if r == nil || r.baseURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/internal/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create ingestion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set(InternalIngestTokenHeader, r.token)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("post event payload: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("event ingestion returned status %d", resp.StatusCode)
	}

	return nil
}
