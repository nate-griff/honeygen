package ipintel_test

import (
	"context"
	"database/sql"
	"testing"

	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/ipintel"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}

func TestCache_GetMissReturnsErrCacheMiss(t *testing.T) {
	cache := ipintel.NewCache(openTestDB(t))

	_, err := cache.Get(context.Background(), "1.2.3.4")
	if err != ipintel.ErrCacheMiss {
		t.Fatalf("Get() error = %v, want ErrCacheMiss", err)
	}
}

func TestCache_SetAndGetRoundTrip(t *testing.T) {
	cache := ipintel.NewCache(openTestDB(t))
	ctx := context.Background()

	want := ipintel.IPIntelResult{
		IP:           "1.2.3.4",
		Status:       ipintel.StatusEnriched,
		CountryCode:  "US",
		Country:      "United States",
		Region:       "California",
		City:         "Los Angeles",
		Latitude:     34.05,
		Longitude:    -118.24,
		Timezone:     "America/Los_Angeles",
		Organization: "EXAMPLE-ORG",
		Network:      "1.2.3.0/24",
		Source:       "geoip+whois",
	}

	if err := cache.Set(ctx, want); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := cache.Get(ctx, "1.2.3.4")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.IP != want.IP {
		t.Errorf("IP = %q, want %q", got.IP, want.IP)
	}
	if got.Status != want.Status {
		t.Errorf("Status = %q, want %q", got.Status, want.Status)
	}
	if got.CountryCode != want.CountryCode {
		t.Errorf("CountryCode = %q, want %q", got.CountryCode, want.CountryCode)
	}
	if got.Country != want.Country {
		t.Errorf("Country = %q, want %q", got.Country, want.Country)
	}
	if got.Region != want.Region {
		t.Errorf("Region = %q, want %q", got.Region, want.Region)
	}
	if got.City != want.City {
		t.Errorf("City = %q, want %q", got.City, want.City)
	}
	if got.Organization != want.Organization {
		t.Errorf("Organization = %q, want %q", got.Organization, want.Organization)
	}
	if got.Network != want.Network {
		t.Errorf("Network = %q, want %q", got.Network, want.Network)
	}
	if got.Source != want.Source {
		t.Errorf("Source = %q, want %q", got.Source, want.Source)
	}
	if got.CachedAt == "" {
		t.Error("CachedAt should be populated by Set()")
	}
}

func TestCache_SetOverwritesExistingEntry(t *testing.T) {
	cache := ipintel.NewCache(openTestDB(t))
	ctx := context.Background()

	first := ipintel.IPIntelResult{IP: "2.3.4.5", Status: ipintel.StatusError}
	if err := cache.Set(ctx, first); err != nil {
		t.Fatalf("Set(first) error = %v", err)
	}

	second := ipintel.IPIntelResult{IP: "2.3.4.5", Status: ipintel.StatusEnriched, Country: "Germany"}
	if err := cache.Set(ctx, second); err != nil {
		t.Fatalf("Set(second) error = %v", err)
	}

	got, err := cache.Get(ctx, "2.3.4.5")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != ipintel.StatusEnriched {
		t.Errorf("Status = %q, want %q", got.Status, ipintel.StatusEnriched)
	}
	if got.Country != "Germany" {
		t.Errorf("Country = %q, want %q", got.Country, "Germany")
	}
}

func TestCache_GetReturnsMissForStaleEntry(t *testing.T) {
	db := openTestDB(t)
	cache := ipintel.NewCache(db)
	ctx := context.Background()

	if err := cache.Set(ctx, ipintel.IPIntelResult{
		IP:      "7.7.7.7",
		Status:  ipintel.StatusEnriched,
		Country: "Old Country",
		Source:  "cache",
	}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	_, err := cache.Get(ctx, "7.7.7.7")
	if err != nil {
		t.Fatalf("Get() fresh error = %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE ip_intel_cache
		SET enriched_at = '2000-01-01T00:00:00Z', updated_at = '2000-01-01T00:00:00Z'
		WHERE ip = '7.7.7.7'
	`); err != nil {
		t.Fatalf("mark stale cache entry error = %v", err)
	}

	_, err = cache.Get(ctx, "7.7.7.7")
	if err != ipintel.ErrCacheMiss {
		t.Fatalf("Get() stale error = %v, want %v", err, ipintel.ErrCacheMiss)
	}
}
