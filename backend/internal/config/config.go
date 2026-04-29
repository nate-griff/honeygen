package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultServiceName        = "honeygen-api"
	defaultServiceVersion     = "dev"
	defaultHTTPAddr           = ":8080"
	defaultSQLitePath         = "/app/storage/sqlite/honeygen.db"
	defaultGeneratedAssetsDir = "/app/storage/generated"
	defaultInternalAPIBaseURL = "http://api:8080"

	DefaultMaxUploadSizeBytes  int64 = 25 * 1024 * 1024  // 25 MB
	AbsoluteMaxUploadSizeBytes int64 = 100 * 1024 * 1024 // 100 MB
)

type Config struct {
	ServiceName                 string         `json:"service_name"`
	ServiceVersion              string         `json:"service_version"`
	AppEnv                      string         `json:"app_env"`
	HTTPAddr                    string         `json:"http_addr"`
	FTPPublicHost               string         `json:"ftp_public_host"`
	FTPPassivePorts             string         `json:"ftp_passive_ports"`
	InternalEventIngestToken    string         `json:"internal_event_ingest_token"`
	AdminPassword               string         `json:"-"`
	ProviderConfigEncryptionKey string         `json:"-"`
	SQLitePath                  string         `json:"sqlite_path"`
	GeneratedAssetsDir          string         `json:"generated_assets_dir"`
	StorageRoot                 string         `json:"storage_root"`
	InternalAPIBaseURL          string         `json:"internal_api_base_url"`
	ConfigFilePath              string         `json:"-"`
	MaxUploadSizeBytes          int64          `json:"max_upload_size_bytes"`
	Provider                    ProviderConfig `json:"provider"`
	MaxMind                     MaxMindConfig  `json:"max_mind"`
}

// EffectiveMaxUploadSizeBytes returns the configured upload size limit,
// applying the 25 MB default when unset and clamping to the 100 MB hard cap.
func (c Config) EffectiveMaxUploadSizeBytes() int64 {
	if c.MaxUploadSizeBytes <= 0 {
		return DefaultMaxUploadSizeBytes
	}
	if c.MaxUploadSizeBytes > AbsoluteMaxUploadSizeBytes {
		return AbsoluteMaxUploadSizeBytes
	}
	return c.MaxUploadSizeBytes
}

type ProviderConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// MaxMindConfig holds credentials and paths for MaxMind GeoLite2 lookups.
// All fields are optional; missing credentials cause GeoIP enrichment to be
// skipped gracefully.
type MaxMindConfig struct {
	AccountID  string `json:"account_id"`
	LicenseKey string `json:"-"` // sensitive – env only
	DBPath     string `json:"db_path"`
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
	ServiceName              *string             `json:"service_name"`
	ServiceVersion           *string             `json:"service_version"`
	AppEnv                   *string             `json:"app_env"`
	HTTPAddr                 *string             `json:"http_addr"`
	FTPPublicHost            *string             `json:"ftp_public_host"`
	FTPPassivePorts          *string             `json:"ftp_passive_ports"`
	InternalEventIngestToken *string             `json:"internal_event_ingest_token"`
	SQLitePath               *string             `json:"sqlite_path"`
	GeneratedAssetsDir       *string             `json:"generated_assets_dir"`
	StorageRoot              *string             `json:"storage_root"`
	InternalAPIBaseURL       *string             `json:"internal_api_base_url"`
	MaxUploadSizeBytes       *int64              `json:"max_upload_size_bytes"`
	Provider                 *fileProviderConfig `json:"provider"`
}

type fileProviderConfig struct {
	BaseURL *string `json:"base_url"`
	APIKey  *string `json:"api_key"`
	Model   *string `json:"model"`
}

// Load builds runtime configuration from defaults, an optional JSON config file,
// and environment variables. JSON keeps the MVP simple and is easy to extend later.
func Load(configPath string) (Config, error) {
	return LoadWithDefaults(configPath, Config{})
}

// LoadWithDefaults builds runtime configuration from built-in defaults, service-specific
// defaults, an optional JSON config file, and environment variables.
func LoadWithDefaults(configPath string, defaults Config) (Config, error) {
	cfg := Config{
		ServiceName:        defaultServiceName,
		ServiceVersion:     defaultServiceVersion,
		HTTPAddr:           defaultHTTPAddr,
		SQLitePath:         defaultSQLitePath,
		GeneratedAssetsDir: defaultGeneratedAssetsDir,
		InternalAPIBaseURL: defaultInternalAPIBaseURL,
	}
	applyDefaultString(&cfg.ServiceName, defaults.ServiceName)
	applyDefaultString(&cfg.ServiceVersion, defaults.ServiceVersion)
	applyDefaultString(&cfg.AppEnv, defaults.AppEnv)
	applyDefaultString(&cfg.HTTPAddr, defaults.HTTPAddr)
	applyDefaultString(&cfg.FTPPublicHost, defaults.FTPPublicHost)
	applyDefaultString(&cfg.FTPPassivePorts, defaults.FTPPassivePorts)
	applyDefaultString(&cfg.InternalEventIngestToken, defaults.InternalEventIngestToken)
	applyDefaultString(&cfg.AdminPassword, defaults.AdminPassword)
	applyDefaultString(&cfg.ProviderConfigEncryptionKey, defaults.ProviderConfigEncryptionKey)
	applyDefaultString(&cfg.SQLitePath, defaults.SQLitePath)
	applyDefaultString(&cfg.GeneratedAssetsDir, defaults.GeneratedAssetsDir)
	applyDefaultString(&cfg.StorageRoot, defaults.StorageRoot)
	applyDefaultString(&cfg.InternalAPIBaseURL, defaults.InternalAPIBaseURL)
	applyDefaultString(&cfg.Provider.BaseURL, defaults.Provider.BaseURL)
	applyDefaultString(&cfg.Provider.APIKey, defaults.Provider.APIKey)
	applyDefaultString(&cfg.Provider.Model, defaults.Provider.Model)

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
	applyEnvOverride(&cfg.FTPPublicHost, "FTP_PUBLIC_HOST")
	applyEnvOverride(&cfg.FTPPassivePorts, "FTP_PASSIVE_PORTS")
	applyEnvOverride(&cfg.InternalEventIngestToken, "INTERNAL_EVENT_INGEST_TOKEN")
	applyEnvOverride(&cfg.AdminPassword, "ADMIN_PASSWORD")
	applyEnvOverride(&cfg.ProviderConfigEncryptionKey, "PROVIDER_CONFIG_ENCRYPTION_KEY")
	applyEnvOverride(&cfg.SQLitePath, "SQLITE_PATH")
	applyEnvOverride(&cfg.GeneratedAssetsDir, "GENERATED_ASSETS_DIR")
	applyEnvOverride(&cfg.StorageRoot, "STORAGE_ROOT")
	applyEnvOverride(&cfg.InternalAPIBaseURL, "INTERNAL_API_BASE_URL")
	applyEnvOverride(&cfg.Provider.BaseURL, "LLM_BASE_URL")
	applyEnvOverride(&cfg.Provider.APIKey, "LLM_API_KEY")
	applyEnvOverride(&cfg.Provider.Model, "LLM_MODEL")
	applyEnvOverrideInt64(&cfg.MaxUploadSizeBytes, "MAX_UPLOAD_SIZE_BYTES")
	applyEnvOverride(&cfg.MaxMind.AccountID, "MM_ACCOUNT_ID")
	applyEnvOverride(&cfg.MaxMind.LicenseKey, "MM_LICENSE_KEY")
	applyEnvOverride(&cfg.MaxMind.DBPath, "MM_DB_PATH")

	if cfg.StorageRoot == "" {
		cfg.StorageRoot = deriveStorageRoot(cfg.SQLitePath, cfg.GeneratedAssetsDir)
	}
	if cfg.MaxMind.DBPath == "" && cfg.StorageRoot != "" {
		cfg.MaxMind.DBPath = cfg.StorageRoot + "/geoip/GeoLite2-City.mmdb"
	}

	var validationErrors []string
	if strings.TrimSpace(cfg.AppEnv) == "" {
		validationErrors = append(validationErrors, "APP_ENV must be set")
	}
	if strings.TrimSpace(cfg.InternalEventIngestToken) == "" {
		validationErrors = append(validationErrors, "INTERNAL_EVENT_INGEST_TOKEN must be set")
	}
	if len(validationErrors) > 0 {
		return Config{}, errors.New(strings.Join(validationErrors, "; "))
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
	applyOptionalString(&cfg.FTPPublicHost, fileCfg.FTPPublicHost)
	applyOptionalString(&cfg.FTPPassivePorts, fileCfg.FTPPassivePorts)
	applyOptionalString(&cfg.InternalEventIngestToken, fileCfg.InternalEventIngestToken)
	applyOptionalString(&cfg.SQLitePath, fileCfg.SQLitePath)
	applyOptionalString(&cfg.GeneratedAssetsDir, fileCfg.GeneratedAssetsDir)
	applyOptionalString(&cfg.StorageRoot, fileCfg.StorageRoot)
	applyOptionalString(&cfg.InternalAPIBaseURL, fileCfg.InternalAPIBaseURL)
	applyOptionalInt64(&cfg.MaxUploadSizeBytes, fileCfg.MaxUploadSizeBytes)
	if fileCfg.Provider != nil {
		applyOptionalString(&cfg.Provider.BaseURL, fileCfg.Provider.BaseURL)
		applyOptionalString(&cfg.Provider.APIKey, fileCfg.Provider.APIKey)
		applyOptionalString(&cfg.Provider.Model, fileCfg.Provider.Model)
	}

	return nil
}

func applyDefaultString(dst *string, value string) {
	if value != "" {
		*dst = value
	}
}

func applyOptionalString(dst *string, value *string) {
	if value != nil && *value != "" {
		*dst = *value
	}
}

func applyOptionalInt64(dst *int64, value *int64) {
	if value != nil && *value > 0 {
		*dst = *value
	}
}

func applyEnvOverride(dst *string, key string) {
	if value := os.Getenv(key); value != "" {
		*dst = value
	}
}

func applyEnvOverrideInt64(dst *int64, key string) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
			*dst = n
		}
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
