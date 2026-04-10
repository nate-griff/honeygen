package decoy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
	"github.com/natet/honeygen/backend/internal/events"
)

func TestNewHandlerServesGeneratedFilesAndPostsEvents(t *testing.T) {
	generatedDir := t.TempDir()
	reportPath := filepath.Join(generatedDir, "world-1", "job-1", "public", "report.txt")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("hello from decoy"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	eventCh := make(chan events.IngestRequest, 1)
	const internalToken = "decoy-shared-secret"
	ingestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/events" {
			t.Fatalf("ingest path = %q, want %q", r.URL.Path, "/internal/events")
		}
		if got := r.Header.Get(events.InternalIngestTokenHeader); got != internalToken {
			t.Fatalf("ingest token = %q, want %q", got, internalToken)
		}
		defer r.Body.Close()

		var payload events.IngestRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		eventCh <- payload
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"evt-1"}`)
	}))
	defer ingestServer.Close()

	handler, err := NewHandler(config.Config{
		GeneratedAssetsDir:       generatedDir,
		InternalAPIBaseURL:       ingestServer.URL,
		InternalEventIngestToken: internalToken,
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/generated/world-1/job-1/public/report.txt?download=1", nil)
	req.Header.Set("User-Agent", "integration-test")
	req.Header.Set("Referer", "https://example.test/source")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "hello from decoy" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "hello from decoy")
	}

	select {
	case payload := <-eventCh:
		if payload.Method != http.MethodGet {
			t.Fatalf("payload.Method = %q, want %q", payload.Method, http.MethodGet)
		}
		if payload.Path != "/generated/world-1/job-1/public/report.txt" {
			t.Fatalf("payload.Path = %q, want %q", payload.Path, "/generated/world-1/job-1/public/report.txt")
		}
		if payload.Query != "download=1" {
			t.Fatalf("payload.Query = %q, want %q", payload.Query, "download=1")
		}
		if payload.StatusCode != http.StatusOK {
			t.Fatalf("payload.StatusCode = %d, want %d", payload.StatusCode, http.StatusOK)
		}
		if payload.BytesSent != len("hello from decoy") {
			t.Fatalf("payload.BytesSent = %d, want %d", payload.BytesSent, len("hello from decoy"))
		}
		if payload.UserAgent != "integration-test" {
			t.Fatalf("payload.UserAgent = %q, want %q", payload.UserAgent, "integration-test")
		}
		if payload.Referer != "https://example.test/source" {
			t.Fatalf("payload.Referer = %q, want %q", payload.Referer, "https://example.test/source")
		}
		if payload.SourceIP == "" {
			t.Fatal("payload.SourceIP was empty")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for decoy event payload")
	}
}

func TestNewHandlerSkipsHealthChecks(t *testing.T) {
	requests := make(chan struct{}, 1)
	ingestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusCreated)
	}))
	defer ingestServer.Close()

	handler, err := NewHandler(config.Config{
		GeneratedAssetsDir: t.TempDir(),
		InternalAPIBaseURL: ingestServer.URL,
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case <-requests:
		t.Fatal("health check should not be ingested")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestNewHandlerShowsLandingPageWhenRootIsEmpty(t *testing.T) {
	handler, err := NewHandler(config.Config{
		GeneratedAssetsDir: t.TempDir(),
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
	}
	if rec.Body.Len() == 0 {
		t.Fatal("landing page body was empty")
	}
}

func TestLandingHandlerCachesDiscoveredLinks(t *testing.T) {
	discoverCalls := 0
	handler := landingHandlerWithDiscover(t.TempDir(), func(_ string, _ int) []string {
		discoverCalls++
		return []string{"/generated/world-1/job-1/public/report.txt"}
	})

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}

	if discoverCalls != 1 {
		t.Fatalf("discoverCalls = %d, want %d", discoverCalls, 1)
	}
}

func TestSourceIPFromRequestIgnoresForwardedHeaderFromUntrustedRemote(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/generated/report.txt", nil)
	req.RemoteAddr = "198.51.100.10:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.55")

	if got := sourceIPFromRequest(req); got != "198.51.100.10" {
		t.Fatalf("sourceIPFromRequest() = %q, want %q", got, "198.51.100.10")
	}
}

func TestSourceIPFromRequestUsesForwardedHeaderForTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/generated/report.txt", nil)
	req.RemoteAddr = "127.0.0.1:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.55, 127.0.0.1")

	if got := sourceIPFromRequest(req); got != "203.0.113.55" {
		t.Fatalf("sourceIPFromRequest() = %q, want %q", got, "203.0.113.55")
	}
}
