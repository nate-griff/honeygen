package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesDefaultsConfigFileAndEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":9099")
	t.Setenv("FTP_PUBLIC_HOST", "127.0.0.1")
	t.Setenv("FTP_PASSIVE_PORTS", "9100-9199")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("ADMIN_PASSWORD", "env-admin-password")
	t.Setenv("PROVIDER_CONFIG_ENCRYPTION_KEY", "env-provider-config-encryption-key")
	t.Setenv("INTERNAL_API_BASE_URL", "http://api.internal:8080")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "env-internal-token")

	configPath := filepath.Join(t.TempDir(), "config.json")
	configJSON := `{
		"sqlite_path": "/tmp/from-file.db",
		"generated_assets_dir": "/tmp/generated",
		"provider": {
			"base_url": "https://provider.example/v1",
			"api_key": "file-key",
			"model": "gpt-4.1-mini"
		}
	}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "production" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "production")
	}
	if cfg.HTTPAddr != ":9099" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":9099")
	}
	if cfg.SQLitePath != "/tmp/from-file.db" {
		t.Fatalf("SQLitePath = %q, want %q", cfg.SQLitePath, "/tmp/from-file.db")
	}
	if cfg.GeneratedAssetsDir != "/tmp/generated" {
		t.Fatalf("GeneratedAssetsDir = %q, want %q", cfg.GeneratedAssetsDir, "/tmp/generated")
	}
	if cfg.StorageRoot != "/tmp" {
		t.Fatalf("StorageRoot = %q, want %q", cfg.StorageRoot, "/tmp")
	}
	if cfg.Provider.Mode() != "external" {
		t.Fatalf("Provider.Mode() = %q, want %q", cfg.Provider.Mode(), "external")
	}
	if cfg.Provider.APIKey != "env-key" {
		t.Fatalf("Provider.APIKey = %q, want %q", cfg.Provider.APIKey, "env-key")
	}
	if cfg.Provider.BaseURL != "https://provider.example/v1" {
		t.Fatalf("Provider.BaseURL = %q, want %q", cfg.Provider.BaseURL, "https://provider.example/v1")
	}
	if cfg.InternalAPIBaseURL != "http://api.internal:8080" {
		t.Fatalf("InternalAPIBaseURL = %q, want %q", cfg.InternalAPIBaseURL, "http://api.internal:8080")
	}
	if cfg.FTPPublicHost != "127.0.0.1" {
		t.Fatalf("FTPPublicHost = %q, want %q", cfg.FTPPublicHost, "127.0.0.1")
	}
	if cfg.FTPPassivePorts != "9100-9199" {
		t.Fatalf("FTPPassivePorts = %q, want %q", cfg.FTPPassivePorts, "9100-9199")
	}
	if cfg.InternalEventIngestToken != "env-internal-token" {
		t.Fatalf("InternalEventIngestToken = %q, want %q", cfg.InternalEventIngestToken, "env-internal-token")
	}
	if cfg.AdminPassword != "env-admin-password" {
		t.Fatalf("AdminPassword = %q, want %q", cfg.AdminPassword, "env-admin-password")
	}
	if cfg.ProviderConfigEncryptionKey != "env-provider-config-encryption-key" {
		t.Fatalf("ProviderConfigEncryptionKey = %q, want %q", cfg.ProviderConfigEncryptionKey, "env-provider-config-encryption-key")
	}
	if cfg.ConfigFilePath != configPath {
		t.Fatalf("ConfigFilePath = %q, want %q", cfg.ConfigFilePath, configPath)
	}
}

func TestLoadReadsMaxMindConfigFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "test-token")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("PROVIDER_CONFIG_ENCRYPTION_KEY", "test-key")
	t.Setenv("MM_ACCOUNT_ID", "123456")
	t.Setenv("MM_LICENSE_KEY", "secret-license-key")
	t.Setenv("MM_DB_PATH", "/custom/path/GeoLite2-City.mmdb")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MaxMind.AccountID != "123456" {
		t.Errorf("MaxMind.AccountID = %q, want %q", cfg.MaxMind.AccountID, "123456")
	}
	if cfg.MaxMind.LicenseKey != "secret-license-key" {
		t.Errorf("MaxMind.LicenseKey = %q, want %q", cfg.MaxMind.LicenseKey, "secret-license-key")
	}
	if cfg.MaxMind.DBPath != "/custom/path/GeoLite2-City.mmdb" {
		t.Errorf("MaxMind.DBPath = %q, want %q", cfg.MaxMind.DBPath, "/custom/path/GeoLite2-City.mmdb")
	}
}

func TestLoadMaxMindConfigDefaultsToStorageRootSubdir(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "test-token")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("PROVIDER_CONFIG_ENCRYPTION_KEY", "test-key")
	t.Setenv("MM_ACCOUNT_ID", "")
	t.Setenv("MM_LICENSE_KEY", "")
	t.Setenv("MM_DB_PATH", "")
	t.Setenv("STORAGE_ROOT", "/app/storage")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MaxMind.AccountID != "" {
		t.Errorf("MaxMind.AccountID = %q, want empty", cfg.MaxMind.AccountID)
	}
	// When MM_DB_PATH is not set, it should default based on StorageRoot.
	if cfg.MaxMind.DBPath == "" {
		t.Error("MaxMind.DBPath should have a default value when StorageRoot is set")
	}
}

func TestLoadRequiresExplicitAppEnvAndInternalEventToken(t *testing.T) {
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("APP_CONFIG_PATH", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "")

	_, err := Load("")
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if got := err.Error(); got != "APP_ENV must be set; INTERNAL_EVENT_INGEST_TOKEN must be set" {
		t.Fatalf("Load() error = %q, want %q", got, "APP_ENV must be set; INTERNAL_EVENT_INGEST_TOKEN must be set")
	}
}

func TestEffectiveMaxUploadSizeBytesAppliesDefaultAndHardCap(t *testing.T) {
	const (
		wantDefault int64 = 25 * 1024 * 1024
		wantHardCap int64 = 100 * 1024 * 1024
	)
	tests := []struct {
		name       string
		configured int64
		want       int64
	}{
		{"zero uses default", 0, wantDefault},
		{"negative uses default", -1, wantDefault},
		{"custom value respected", 5 * 1024 * 1024, 5 * 1024 * 1024},
		{"value above hard cap clamped", 101 * 1024 * 1024, wantHardCap},
		{"hard cap exactly", wantHardCap, wantHardCap},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{MaxUploadSizeBytes: tc.configured}
			if got := cfg.EffectiveMaxUploadSizeBytes(); got != tc.want {
				t.Errorf("EffectiveMaxUploadSizeBytes() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestLoadReadsMaxUploadSizeBytesFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("INTERNAL_EVENT_INGEST_TOKEN", "test-token")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("PROVIDER_CONFIG_ENCRYPTION_KEY", "test-key")
	t.Setenv("MAX_UPLOAD_SIZE_BYTES", "10485760") // 10 MB

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MaxUploadSizeBytes != 10485760 {
		t.Fatalf("MaxUploadSizeBytes = %d, want %d", cfg.MaxUploadSizeBytes, 10485760)
	}
}

