package ipintel

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// Service performs best-effort IP enrichment using a SQLite cache, an optional
// GeoIP MMDB reader, and an optional WHOIS/RDAP lookup.
type Service struct {
	cache *Cache
	geoip GeoIPLookup
	whois WHOISLookup
}

// NewService constructs a Service. geoip and whois may be nil; missing
// lookups degrade gracefully.
func NewService(cache *Cache, geoip GeoIPLookup, whois WHOISLookup) *Service {
	return &Service{cache: cache, geoip: geoip, whois: whois}
}

// Enrich returns an IPIntelResult for ip. It returns a cached result when
// available. For private/loopback/blank IPs it returns immediately without
// external lookups. Lookup failures are silently absorbed so the caller can
// always persist partial data.
func (s *Service) Enrich(ctx context.Context, ip string) (IPIntelResult, error) {
	normalizedIP := strings.TrimSpace(ip)
	if normalizedIP == "" {
		return IPIntelResult{IP: ip, Status: StatusSkipped}, nil
	}

	parsed := net.ParseIP(normalizedIP)
	if parsed == nil {
		return IPIntelResult{IP: normalizedIP, Status: StatusSkipped}, nil
	}

	if isPrivateOrReserved(parsed) {
		return IPIntelResult{IP: normalizedIP, Status: StatusPrivate}, nil
	}

	// Check cache first.
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, normalizedIP)
		if err == nil {
			return cached, nil
		}
		if !errors.Is(err, ErrCacheMiss) {
			// Cache read error – fall through to fresh lookup.
		}
	}

	result := IPIntelResult{IP: normalizedIP}
	var sources []string

	if s.geoip != nil {
		rec, err := s.geoip.LookupCity(parsed)
		if err == nil {
			result.CountryCode = rec.CountryCode
			result.Country = rec.Country
			result.Region = rec.Region
			result.City = rec.City
			result.Latitude = rec.Latitude
			result.Longitude = rec.Longitude
			result.Timezone = rec.Timezone
			sources = append(sources, "geoip")
		}
	}

	if s.whois != nil {
		rec, err := s.whois.Lookup(ctx, normalizedIP)
		if err == nil {
			result.Organization = rec.Organization
			result.Network = rec.Network
			sources = append(sources, "whois")
		}
	}

	if len(sources) > 0 {
		result.Status = StatusEnriched
		result.Source = strings.Join(sources, "+")
	} else {
		result.Status = StatusError
		result.Source = "no_backends"
	}

	if s.cache != nil {
		result.CachedAt = time.Now().UTC().Format(time.RFC3339)
		_ = s.cache.Set(ctx, result)
	}

	return result, nil
}

// isPrivateOrReserved reports whether ip is a loopback, private, or otherwise
// non-routable address that should not trigger external lookups.
func isPrivateOrReserved(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	return false
}
