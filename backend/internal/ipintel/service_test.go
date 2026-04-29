package ipintel_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/natet/honeygen/backend/internal/ipintel"
)

// stubGeoIP implements geoIPLookup for tests — exported via
// ipintel.NewStubGeoIP for use in external test packages.
type stubGeoIP struct {
	record ipintel.GeoIPRecord
	err    error
	calls  int
}

func (s *stubGeoIP) LookupCity(ip net.IP) (ipintel.GeoIPRecord, error) {
	s.calls++
	return s.record, s.err
}

// stubWHOIS implements whoisLookup for tests.
type stubWHOIS struct {
	record ipintel.WHOISRecord
	err    error
	calls  int
}

func (s *stubWHOIS) Lookup(_ context.Context, _ string) (ipintel.WHOISRecord, error) {
	s.calls++
	return s.record, s.err
}

func TestService_BlankIPReturnsSkipped(t *testing.T) {
	svc := ipintel.NewService(ipintel.NewCache(openTestDB(t)), nil, nil)

	result, err := svc.Enrich(context.Background(), "")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}
	if result.Status != ipintel.StatusSkipped {
		t.Errorf("Status = %q, want %q", result.Status, ipintel.StatusSkipped)
	}
}

func TestService_PrivateIPReturnsPrivateWithoutLookup(t *testing.T) {
	geo := &stubGeoIP{}
	whois := &stubWHOIS{}
	svc := ipintel.NewService(ipintel.NewCache(openTestDB(t)), geo, whois)

	for _, ip := range []string{
		"127.0.0.1",
		"10.0.0.1",
		"192.168.1.100",
		"172.16.5.5",
		"::1",
		"fc00::1",
	} {
		t.Run(ip, func(t *testing.T) {
			result, err := svc.Enrich(context.Background(), ip)
			if err != nil {
				t.Fatalf("Enrich(%q) error = %v", ip, err)
			}
			if result.Status != ipintel.StatusPrivate {
				t.Errorf("Status = %q, want %q", result.Status, ipintel.StatusPrivate)
			}
		})
	}

	if geo.calls != 0 {
		t.Errorf("GeoIP was called %d times, want 0 for private IPs", geo.calls)
	}
	if whois.calls != 0 {
		t.Errorf("WHOIS was called %d times, want 0 for private IPs", whois.calls)
	}
}

func TestService_PublicIPEnrichedAndCachedUsingBothLookups(t *testing.T) {
	geo := &stubGeoIP{
		record: ipintel.GeoIPRecord{
			CountryCode: "US",
			Country:     "United States",
			Region:      "California",
			City:        "Los Angeles",
			Latitude:    34.05,
			Longitude:   -118.24,
			Timezone:    "America/Los_Angeles",
		},
	}
	whois := &stubWHOIS{
		record: ipintel.WHOISRecord{
			Organization: "EXAMPLE-ORG",
			Network:      "1.2.3.0/24",
		},
	}
	cache := ipintel.NewCache(openTestDB(t))
	svc := ipintel.NewService(cache, geo, whois)

	result, err := svc.Enrich(context.Background(), "1.2.3.4")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Status != ipintel.StatusEnriched {
		t.Errorf("Status = %q, want %q", result.Status, ipintel.StatusEnriched)
	}
	if result.CountryCode != "US" {
		t.Errorf("CountryCode = %q, want %q", result.CountryCode, "US")
	}
	if result.Country != "United States" {
		t.Errorf("Country = %q, want %q", result.Country, "United States")
	}
	if result.Organization != "EXAMPLE-ORG" {
		t.Errorf("Organization = %q, want %q", result.Organization, "EXAMPLE-ORG")
	}
	if result.Network != "1.2.3.0/24" {
		t.Errorf("Network = %q, want %q", result.Network, "1.2.3.0/24")
	}
	if result.CachedAt == "" {
		t.Error("CachedAt should be set after enrichment")
	}
	if result.Source == "" {
		t.Error("Source should be set after enrichment")
	}

	// Verify it was persisted to cache.
	cached, err := cache.Get(context.Background(), "1.2.3.4")
	if err != nil {
		t.Fatalf("cache.Get() after Enrich error = %v", err)
	}
	if cached.CountryCode != "US" {
		t.Errorf("cached.CountryCode = %q, want %q", cached.CountryCode, "US")
	}
}

func TestService_CacheHitAvoidsRepeatLookup(t *testing.T) {
	geo := &stubGeoIP{
		record: ipintel.GeoIPRecord{CountryCode: "DE", Country: "Germany"},
	}
	whois := &stubWHOIS{}
	cache := ipintel.NewCache(openTestDB(t))
	svc := ipintel.NewService(cache, geo, whois)

	// First call: performs lookup and caches.
	if _, err := svc.Enrich(context.Background(), "5.6.7.8"); err != nil {
		t.Fatalf("first Enrich() error = %v", err)
	}
	callsAfterFirst := geo.calls

	// Second call: should hit cache, not call geo/whois again.
	result, err := svc.Enrich(context.Background(), "5.6.7.8")
	if err != nil {
		t.Fatalf("second Enrich() error = %v", err)
	}
	if geo.calls != callsAfterFirst {
		t.Errorf("GeoIP called %d times total, expected no new calls after cache hit", geo.calls)
	}
	if result.CountryCode != "DE" {
		t.Errorf("CountryCode = %q, want %q", result.CountryCode, "DE")
	}
}

func TestService_StaleCacheTriggersFreshLookup(t *testing.T) {
	db := openTestDB(t)
	geo := &stubGeoIP{
		record: ipintel.GeoIPRecord{CountryCode: "DE", Country: "Germany"},
	}
	cache := ipintel.NewCache(db)
	svc := ipintel.NewService(cache, geo, &stubWHOIS{})

	if _, err := svc.Enrich(context.Background(), "5.6.7.8"); err != nil {
		t.Fatalf("first Enrich() error = %v", err)
	}
	callsAfterFirst := geo.calls

	if _, err := db.ExecContext(context.Background(), `
		UPDATE ip_intel_cache
		SET enriched_at = '2000-01-01T00:00:00Z', updated_at = '2000-01-01T00:00:00Z'
		WHERE ip = '5.6.7.8'
	`); err != nil {
		t.Fatalf("mark stale cache entry error = %v", err)
	}

	result, err := svc.Enrich(context.Background(), "5.6.7.8")
	if err != nil {
		t.Fatalf("second Enrich() error = %v", err)
	}
	if geo.calls != callsAfterFirst+1 {
		t.Fatalf("GeoIP calls = %d, want %d after stale cache refresh", geo.calls, callsAfterFirst+1)
	}
	if result.CountryCode != "DE" {
		t.Errorf("CountryCode = %q, want %q", result.CountryCode, "DE")
	}
}

func TestService_GeoIPFailureStillReturnsPartialResult(t *testing.T) {
	geo := &stubGeoIP{err: errors.New("mmdb unavailable")}
	whois := &stubWHOIS{
		record: ipintel.WHOISRecord{Organization: "PARTIAL-ORG"},
	}
	svc := ipintel.NewService(ipintel.NewCache(openTestDB(t)), geo, whois)

	result, err := svc.Enrich(context.Background(), "9.8.7.6")
	if err != nil {
		t.Fatalf("Enrich() error = %v (should not propagate lookup failure)", err)
	}
	// Even on geoip failure, we still get a result (not a hard error).
	if result.IP != "9.8.7.6" {
		t.Errorf("IP = %q, want %q", result.IP, "9.8.7.6")
	}
	// WHOIS partial data should still appear.
	if result.Organization != "PARTIAL-ORG" {
		t.Errorf("Organization = %q, want %q", result.Organization, "PARTIAL-ORG")
	}
}

func TestService_NilGeoIPAndWhoisProducesErrorStatus(t *testing.T) {
	svc := ipintel.NewService(ipintel.NewCache(openTestDB(t)), nil, nil)

	result, err := svc.Enrich(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}
	// Without any lookup backends, status should reflect that.
	if result.IP != "8.8.8.8" {
		t.Errorf("IP = %q, want %q", result.IP, "8.8.8.8")
	}
	if result.Status == ipintel.StatusPrivate {
		t.Error("8.8.8.8 is not private")
	}
}
