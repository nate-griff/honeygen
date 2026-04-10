package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

const (
	defaultServiceName        = "honeygen-api"
	defaultServiceVersion     = "dev"
	defaultAppEnv             = "development"
	defaultHTTPAddr           = ":8080"
	defaultSQLitePath         = "/app/storage/sqlite/honeygen.db"
	defaultGeneratedAssetsDir = "/app/storage/generated"
	defaultInternalAPIBaseURL = "http://api:8080"
)

type Config struct {
	ServiceName        string         `json:"service_name"`
	ServiceVersion     string         `json:"service_version"`
	AppEnv             string         `json:"app_env"`
	HTTPAddr           string         `json:"http_addr"`
	SQLitePath         string         `json:"sqlite_path"`
	GeneratedAssetsDir string         `json:"generated_assets_dir"`
	StorageRoot        string         `json:"storage_root"`
	InternalAPIBaseURL string         `json:"internal_api_base_url"`
	ConfigFilePath     string         `json:"-"`
	Provider           ProviderConfig `json:"provider"`
}

type ProviderConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

func (p ProviderConfig) Ready() bool {
	return p.BaseURL != "" && p.APIKey != "" && p.Model != ""
}

func (p ProviderConfig) Mode() string {
	if p.Ready() {
		return "external"
	}
	return "unconfigured"
}

type fileConfig struct {
	ServiceName        *string             `json:"service_name"`
	ServiceVersion     *string             `json:"service_version"`
	AppEnv             *string             `json:"app_env"`
	HTTPAddr           *string             `json:"http_addr"`
	SQLitePath         *string             `json:"sqlite_path"`
	GeneratedAssetsDir *string             `json:"generated_assets_dir"`
	StorageRoot        *string             `json:"storage_root"`
	InternalAPIBaseURL *string             `json:"internal_api_base_url"`
	Provider           *fileProviderConfig `json:"provider"`
}

type fileProviderConfig struct {
	BaseURL *string `json:"base_url"`
	APIKey  *string `json:"api_key"`
	Model   *string `json:"model"`
}

// Load builds runtime configuration from defaults, an optional JSON config file,
// and environment variables. JSON keeps the MVP simple and is easy to extend later.
func Load(configPath string) (Config, error) {
	cfg := Config{
		ServiceName:        defaultServiceName,
		ServiceVersion:     defaultServiceVersion,
		AppEnv:             defaultAppEnv,
		HTTPAddr:           defaultHTTPAddr,
		SQLitePath:         defaultSQLitePath,
		GeneratedAssetsDir: defaultGeneratedAssetsDir,
		InternalAPIBaseURL: defaultInternalAPIBaseURL,
	}

	resolvedConfigPath := configPath
	if resolvedConfigPath == "" {
		resolvedConfigPath = EnvOrDefault("CONFIG_PATH", EnvOrDefault("APP_CONFIG_PATH", ""))
	}
	if resolvedConfigPath != "" {
		if err := applyFileConfig(&cfg, resolvedConfigPath); err != nil {
			return Config{}, err
		}
		cfg.ConfigFilePath = resolvedConfigPath
	}

	applyEnvOverride(&cfg.ServiceName, "SERVICE_NAME")
	applyEnvOverride(&cfg.ServiceVersion, "APP_VERSION")
	applyEnvOverride(&cfg.AppEnv, "APP_ENV")
	applyEnvOverride(&cfg.HTTPAddr, "HTTP_ADDR")
	applyEnvOverride(&cfg.SQLitePath, "SQLITE_PATH")
	applyEnvOverride(&cfg.GeneratedAssetsDir, "GENERATED_ASSETS_DIR")
	applyEnvOverride(&cfg.StorageRoot, "STORAGE_ROOT")
	applyEnvOverride(&cfg.InternalAPIBaseURL, "INTERNAL_API_BASE_URL")
	applyEnvOverride(&cfg.Provider.BaseURL, "LLM_BASE_URL")
	applyEnvOverride(&cfg.Provider.APIKey, "LLM_API_KEY")
	applyEnvOverride(&cfg.Provider.Model, "LLM_MODEL")

	if cfg.StorageRoot == "" {
		cfg.StorageRoot = deriveStorageRoot(cfg.SQLitePath, cfg.GeneratedAssetsDir)
	}

	return cfg, nil
}

func applyFileConfig(cfg *Config, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var fileCfg fileConfig
	if err := json.Unmarshal(content, &fileCfg); err != nil {
		return fmt.Errorf("parse config file %q as JSON: %w", path, err)
	}

	applyOptionalString(&cfg.ServiceName, fileCfg.ServiceName)
	applyOptionalString(&cfg.ServiceVersion, fileCfg.ServiceVersion)
	applyOptionalString(&cfg.AppEnv, fileCfg.AppEnv)
	applyOptionalString(&cfg.HTTPAddr, fileCfg.HTTPAddr)
	applyOptionalString(&cfg.SQLitePath, fileCfg.SQLitePath)
	applyOptionalString(&cfg.GeneratedAssetsDir, fileCfg.GeneratedAssetsDir)
	applyOptionalString(&cfg.StorageRoot, fileCfg.StorageRoot)
	applyOptionalString(&cfg.InternalAPIBaseURL, fileCfg.InternalAPIBaseURL)
	if fileCfg.Provider != nil {
		applyOptionalString(&cfg.Provider.BaseURL, fileCfg.Provider.BaseURL)
		applyOptionalString(&cfg.Provider.APIKey, fileCfg.Provider.APIKey)
		applyOptionalString(&cfg.Provider.Model, fileCfg.Provider.Model)
	}

	return nil
}

func applyOptionalString(dst *string, value *string) {
	if value != nil && *value != "" {
		*dst = *value
	}
}

func applyEnvOverride(dst *string, key string) {
	if value := os.Getenv(key); value != "" {
		*dst = value
	}
}

func deriveStorageRoot(sqlitePath, generatedAssetsDir string) string {
	if generatedAssetsDir != "" {
		if filepath.Clean(generatedAssetsDir) != generatedAssetsDir && filepath.ToSlash(generatedAssetsDir) == generatedAssetsDir {
			return path.Dir(generatedAssetsDir)
		}
		return filepath.Dir(generatedAssetsDir)
	}
	if sqlitePath != "" {
		if filepath.Clean(sqlitePath) != sqlitePath && filepath.ToSlash(sqlitePath) == sqlitePath {
			return path.Dir(sqlitePath)
		}
		return filepath.Dir(sqlitePath)
	}
	return ""
}
