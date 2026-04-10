package models

type StatusResponse struct {
	Service      ServiceStatus        `json:"service"`
	Database     DatabaseStatus       `json:"database"`
	Storage      StorageStatus        `json:"storage"`
	Provider     ProviderStatus       `json:"provider"`
	Counts       StatusCounts         `json:"counts"`
	RecentEvents []RecentEventSummary `json:"recent_events"`
	LatestJob    *LatestJobSummary    `json:"latest_job,omitempty"`
}

type ServiceStatus struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type DatabaseStatus struct {
	Ready bool   `json:"ready"`
	Path  string `json:"path"`
}

type StorageStatus struct {
	Root               string `json:"root"`
	GeneratedAssetsDir string `json:"generated_assets_dir"`
}

type ProviderStatus struct {
	Mode    string `json:"mode"`
	Ready   bool   `json:"ready"`
	BaseURL string `json:"base_url,omitempty"`
	Model   string `json:"model,omitempty"`
}

type StatusCounts struct {
	Assets       int `json:"assets"`
	RecentEvents int `json:"recent_events"`
}

type LatestJobSummary struct {
	ID           string `json:"id"`
	WorldModelID string `json:"world_model_id"`
	Status       string `json:"status"`
	CompletedAt  string `json:"completed_at,omitempty"`
	AssetCount   int    `json:"asset_count"`
}

type RecentEventSummary struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	Path      string `json:"path"`
	SourceIP  string `json:"source_ip"`
	Timestamp string `json:"timestamp"`
}
