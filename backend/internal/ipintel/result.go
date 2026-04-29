package ipintel

// Status values for IPIntelResult.
const (
	StatusEnriched = "enriched"
	StatusPrivate  = "private"
	StatusSkipped  = "skipped"
	StatusError    = "error"
)

// IPIntelResult is the enrichment snapshot stored in event metadata
// under the key "ip_intelligence".
type IPIntelResult struct {
	IP           string  `json:"ip"`
	Status       string  `json:"status"`
	CountryCode  string  `json:"country_code,omitempty"`
	Country      string  `json:"country,omitempty"`
	Region       string  `json:"region,omitempty"`
	City         string  `json:"city,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	Timezone     string  `json:"timezone,omitempty"`
	Organization string  `json:"organization,omitempty"`
	Network      string  `json:"network,omitempty"`
	CachedAt     string  `json:"cached_at,omitempty"`
	Source       string  `json:"source,omitempty"`
}
