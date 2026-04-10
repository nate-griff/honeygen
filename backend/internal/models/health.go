package models

type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	Database struct {
		Ready bool `json:"ready"`
	} `json:"database"`
}
