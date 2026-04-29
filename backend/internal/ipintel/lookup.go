package ipintel

import (
	"context"
	"net"
)

// GeoIPRecord holds geographic data for one IP address.
type GeoIPRecord struct {
	CountryCode string
	Country     string
	Region      string
	City        string
	Latitude    float64
	Longitude   float64
	Timezone    string
}

// GeoIPLookup is the interface satisfied by the real MaxMind reader and test stubs.
type GeoIPLookup interface {
	LookupCity(ip net.IP) (GeoIPRecord, error)
}

// WHOISRecord holds network registration data for one IP address.
type WHOISRecord struct {
	Organization string
	Network      string
}

// WHOISLookup is the interface satisfied by the real RDAP client and test stubs.
type WHOISLookup interface {
	Lookup(ctx context.Context, ip string) (WHOISRecord, error)
}
