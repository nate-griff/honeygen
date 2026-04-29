package events_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/events"
	"github.com/natet/honeygen/backend/internal/ipintel"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}

func newTestService(t *testing.T, enricher events.IPEnricher) *events.Service {
	t.Helper()
	db := openTestDB(t)
	repo := events.NewRepository(db)
	svc := events.NewService(repo, nil)
	if enricher != nil {
		svc.SetIPEnricher(enricher)
	}
	return svc
}

// stubEnricher implements events.IPEnricher for tests.
type stubEnricher struct {
	result ipintel.IPIntelResult
	err    error
	calls  int
}

func (s *stubEnricher) Enrich(_ context.Context, ip string) (ipintel.IPIntelResult, error) {
	s.calls++
	s.result.IP = ip
	return s.result, s.err
}

func TestIngest_AttachesIPIntelligenceToMetadata(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	_ = logger

	enricher := &stubEnricher{
		result: ipintel.IPIntelResult{
			Status:       ipintel.StatusEnriched,
			CountryCode:  "US",
			Country:      "United States",
			City:         "Los Angeles",
			Organization: "EXAMPLE-ORG",
			Network:      "203.0.113.0/24",
			Source:       "geoip+whois",
		},
	}
	svc := newTestService(t, enricher)

	event, err := svc.Ingest(context.Background(), events.IngestRequest{
		Timestamp:  time.Now().UTC(),
		Method:     "GET",
		Path:       "/generated/world-1/job-1/public/index.html",
		SourceIP:   "203.0.113.10",
		StatusCode: 200,
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	intel, ok := event.Metadata["ip_intelligence"]
	if !ok {
		t.Fatal("event.Metadata missing ip_intelligence key")
	}

	intelMap, ok := intel.(map[string]any)
	if !ok {
		t.Fatalf("ip_intelligence is %T, want map[string]any", intel)
	}

	if status, _ := intelMap["status"].(string); status != ipintel.StatusEnriched {
		t.Errorf("ip_intelligence.status = %q, want %q", status, ipintel.StatusEnriched)
	}
	if country, _ := intelMap["country"].(string); country != "United States" {
		t.Errorf("ip_intelligence.country = %q, want %q", country, "United States")
	}
	if org, _ := intelMap["organization"].(string); org != "EXAMPLE-ORG" {
		t.Errorf("ip_intelligence.organization = %q, want %q", org, "EXAMPLE-ORG")
	}
	if enricher.calls != 1 {
		t.Errorf("enricher called %d times, want 1", enricher.calls)
	}
}

func TestIngest_SkipsIPEnrichmentWhenNoEnricherConfigured(t *testing.T) {
	svc := newTestService(t, nil)

	event, err := svc.Ingest(context.Background(), events.IngestRequest{
		Timestamp:  time.Now().UTC(),
		Method:     "GET",
		Path:       "/generated/world-1/job-1/public/index.html",
		SourceIP:   "203.0.113.10",
		StatusCode: 200,
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	if _, ok := event.Metadata["ip_intelligence"]; ok {
		t.Error("expected no ip_intelligence key when enricher is not configured")
	}
}

func TestIngest_EnrichmentFailureDoesNotBreakIngest(t *testing.T) {
	errEnricher := &stubEnricher{err: &enrichTestError{msg: "lookup timeout"}}
	svc := newTestService(t, errEnricher)

	_, err := svc.Ingest(context.Background(), events.IngestRequest{
		Timestamp:  time.Now().UTC(),
		Method:     "GET",
		Path:       "/generated/world-1/job-1/public/index.html",
		SourceIP:   "1.2.3.4",
		StatusCode: 200,
	})
	if err != nil {
		t.Fatalf("Ingest() should not fail when enrichment errors; got = %v", err)
	}
}

func TestIngest_PreservesExistingMetadataWhenEnriching(t *testing.T) {
	enricher := &stubEnricher{
		result: ipintel.IPIntelResult{Status: ipintel.StatusEnriched, Country: "Japan"},
	}
	svc := newTestService(t, enricher)

	event, err := svc.Ingest(context.Background(), events.IngestRequest{
		Timestamp:  time.Now().UTC(),
		Method:     "RETR",
		Path:       "/generated/world-1/job-1/public/file.html",
		SourceIP:   "1.1.1.1",
		StatusCode: 0,
		Metadata:   map[string]any{"protocol": "ftp"},
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	if proto, _ := event.Metadata["protocol"].(string); proto != "ftp" {
		t.Errorf("existing metadata key protocol = %q, want %q", proto, "ftp")
	}
	if _, ok := event.Metadata["ip_intelligence"]; !ok {
		t.Error("ip_intelligence key missing even when pre-existing metadata exists")
	}
}

type enrichTestError struct{ msg string }

func (e *enrichTestError) Error() string { return e.msg }
