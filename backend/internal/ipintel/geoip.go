package ipintel

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// GeoIP2Reader wraps a MaxMind GeoIP2/GeoLite2 MMDB file and satisfies GeoIPLookup.
type GeoIP2Reader struct {
	reader *geoip2.Reader
}

// OpenGeoIP2Reader opens an MMDB file at path and returns a GeoIP2Reader.
// The caller is responsible for calling Close() when done.
func OpenGeoIP2Reader(path string) (*GeoIP2Reader, error) {
	r, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open geoip2 db %q: %w", path, err)
	}
	return &GeoIP2Reader{reader: r}, nil
}

// LookupCity returns geographic information for ip from the MMDB.
func (r *GeoIP2Reader) LookupCity(ip net.IP) (GeoIPRecord, error) {
	record, err := r.reader.City(ip)
	if err != nil {
		return GeoIPRecord{}, fmt.Errorf("geoip2 city lookup for %s: %w", ip, err)
	}

	region := ""
	if len(record.Subdivisions) > 0 {
		region = record.Subdivisions[0].Names["en"]
	}

	return GeoIPRecord{
		CountryCode: record.Country.IsoCode,
		Country:     record.Country.Names["en"],
		Region:      region,
		City:        record.City.Names["en"],
		Latitude:    record.Location.Latitude,
		Longitude:   record.Location.Longitude,
		Timezone:    record.Location.TimeZone,
	}, nil
}

// Close releases the MMDB file handle.
func (r *GeoIP2Reader) Close() error {
	return r.reader.Close()
}
