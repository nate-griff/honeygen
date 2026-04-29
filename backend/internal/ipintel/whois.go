package ipintel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RDAPClient satisfies WHOISLookup using the ARIN RDAP REST API.
// It is best-effort; errors cause the caller to skip WHOIS data gracefully.
type RDAPClient struct {
	httpClient *http.Client
}

// NewRDAPClient returns an RDAPClient with the given http.Client.
// Pass nil to use a default client with a short timeout.
func NewRDAPClient(httpClient *http.Client) *RDAPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &RDAPClient{httpClient: httpClient}
}

// Lookup queries ARIN RDAP for the network owning ip. On failure or non-200
// responses it returns an error so the caller can degrade gracefully.
func (c *RDAPClient) Lookup(ctx context.Context, ip string) (WHOISRecord, error) {
	url := "https://rdap.arin.net/registry/ip/" + ip
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return WHOISRecord{}, fmt.Errorf("build rdap request: %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WHOISRecord{}, fmt.Errorf("rdap request for %q: %w", ip, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WHOISRecord{}, fmt.Errorf("rdap status %d for %q", resp.StatusCode, ip)
	}

	var payload struct {
		Name   string `json:"name"`
		Handle string `json:"handle"`
		CIDR0  []struct {
			V4Prefix string `json:"v4prefix"`
			V6Prefix string `json:"v6prefix"`
			Length   int    `json:"length"`
		} `json:"cidr0_cidrs"`
		Entities []struct {
			Handle  string   `json:"handle"`
			Roles   []string `json:"roles"`
			VCard   []any    `json:"vcardArray"`
		} `json:"entities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return WHOISRecord{}, fmt.Errorf("decode rdap response for %q: %w", ip, err)
	}

	rec := WHOISRecord{}

	// Build CIDR network string.
	if len(payload.CIDR0) > 0 {
		c0 := payload.CIDR0[0]
		prefix := c0.V4Prefix
		if prefix == "" {
			prefix = c0.V6Prefix
		}
		if prefix != "" {
			rec.Network = fmt.Sprintf("%s/%d", prefix, c0.Length)
		}
	}

	// Prefer the registrant/abuser contact name; fall back to network name/handle.
	org := extractOrganizationName(payload.Entities)
	if org == "" {
		org = payload.Name
	}
	if org == "" {
		org = payload.Handle
	}
	rec.Organization = strings.TrimSpace(org)

	return rec, nil
}

func extractOrganizationName(entities []struct {
	Handle string   `json:"handle"`
	Roles  []string `json:"roles"`
	VCard  []any    `json:"vcardArray"`
}) string {
	for _, e := range entities {
		for _, role := range e.Roles {
			if role == "registrant" || role == "technical" {
				if name := vcardFN(e.VCard); name != "" {
					return name
				}
				return e.Handle
			}
		}
	}
	return ""
}

// vcardFN extracts the "fn" (full name) property from a jCard array.
func vcardFN(vcardArray []any) string {
	if len(vcardArray) < 2 {
		return ""
	}
	items, ok := vcardArray[1].([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		prop, ok := item.([]any)
		if !ok || len(prop) < 4 {
			continue
		}
		propName, ok := prop[0].(string)
		if !ok || strings.ToLower(propName) != "fn" {
			continue
		}
		if val, ok := prop[3].(string); ok {
			return val
		}
	}
	return ""
}
