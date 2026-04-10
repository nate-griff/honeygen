package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesDefaultsConfigFileAndEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":9099")
	t.Setenv("LLM_API_KEY", "env-key")

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
	if cfg.ConfigFilePath != configPath {
		t.Fatalf("ConfigFilePath = %q, want %q", cfg.ConfigFilePath, configPath)
	}
}

func TestLoadUsesDefaultsWhenConfigIsNotProvided(t *testing.T) {
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("APP_CONFIG_PATH", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ServiceName != "honeygen-api" {
		t.Fatalf("ServiceName = %q, want %q", cfg.ServiceName, "honeygen-api")
	}
	if cfg.ServiceVersion != "dev" {
		t.Fatalf("ServiceVersion = %q, want %q", cfg.ServiceVersion, "dev")
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.Provider.Mode() != "unconfigured" {
		t.Fatalf("Provider.Mode() = %q, want %q", cfg.Provider.Mode(), "unconfigured")
	}
}
