package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/config"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/deployments"
	"github.com/natet/honeygen/backend/internal/events"
	"github.com/natet/honeygen/backend/internal/generation"
	"github.com/natet/honeygen/backend/internal/ipintel"
	"github.com/natet/honeygen/backend/internal/models"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/rendering"
	"github.com/natet/honeygen/backend/internal/storage"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

type APIApp struct {
	providerMu          sync.RWMutex
	Config              config.Config
	Logger              *slog.Logger
	DB                  *sql.DB
	StatusQueries       appdb.StatusSummaryReader
	Settings            *appdb.SettingsStore
	WorldModels         *worldmodels.Service
	Provider            provider.Provider
	Generation          *generation.Service
	AssetRepo           *assets.Repository
	EventRepo           *events.Repository
	EventService        *events.Service
	JobStore            *generation.JobStore
	Planner             *generation.Planner
	Storage             *storage.Filesystem
	Renderers           rendering.Registry
	AdminSessions       *AdminSessionManager
	ProviderConfigCodec *providerConfigCodec
	DeploymentRepo      *deployments.Repository
	DeploymentManager   *deployments.Manager
	IPIntelService      *ipintel.Service
	MMDBUpdater         *ipintel.Updater
}

func NewAPIApp(ctx context.Context, cfg config.Config, logger *slog.Logger) (*APIApp, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("admin password must be set")
	}

	providerConfigCodec, err := newProviderConfigCodec(cfg.ProviderConfigEncryptionKey)
	if err != nil {
		return nil, err
	}

	if cfg.StorageRoot != "" {
		if err := os.MkdirAll(cfg.StorageRoot, 0o755); err != nil {
			return nil, fmt.Errorf("create storage root: %w", err)
		}
	}
	if cfg.GeneratedAssetsDir != "" {
		if err := os.MkdirAll(cfg.GeneratedAssetsDir, 0o755); err != nil {
			return nil, fmt.Errorf("create generated assets directory: %w", err)
		}
	}

	database, err := appdb.OpenSQLite(ctx, cfg.SQLitePath)
	if err != nil {
		return nil, err
	}

	if err := appdb.Migrate(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}

	worldModelService := worldmodels.NewService(worldmodels.NewRepository(database))
	if err := worldModelService.EnsureSeedData(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("seed world models: %w", err)
	}

	assetRepo := assets.NewRepository(database)
	eventRepo := events.NewRepository(database)
	jobStore := generation.NewJobStore(database)
	settingsStore := appdb.NewSettingsStore(database)
	filesystem := storage.NewFilesystem(cfg.StorageRoot)
	deploymentRepo := deployments.NewRepository(database)

	apiBaseURL := fmt.Sprintf("http://localhost%s", cfg.HTTPAddr)
	deploymentManager := deployments.NewManager(
		deploymentRepo,
		cfg.StorageRoot,
		logger,
		cfg.InternalEventIngestToken,
		apiBaseURL,
		cfg.FTPPublicHost,
		cfg.FTPPassivePorts,
	)

	savedProvider, err := loadSavedProviderConfig(ctx, settingsStore, providerConfigCodec)
	if err != nil {
		logger.Warn("load saved provider settings", "error", err)
	} else if savedProvider != nil {
		if savedProvider.BaseURL != "" {
			cfg.Provider.BaseURL = savedProvider.BaseURL
		}
		if savedProvider.APIKey != "" {
			cfg.Provider.APIKey = savedProvider.APIKey
		}
		if savedProvider.Model != "" {
			cfg.Provider.Model = savedProvider.Model
		}
	}

	result := &APIApp{
		Config:              cfg,
		Logger:              logger,
		DB:                  database,
		StatusQueries:       appdb.NewStatusQueries(database),
		Settings:            settingsStore,
		WorldModels:         worldModelService,
		Provider:            provider.NewOpenAI(cfg.Provider, nil),
		AssetRepo:           assetRepo,
		EventRepo:           eventRepo,
		EventService:        events.NewService(eventRepo, assetRepo),
		JobStore:            jobStore,
		Planner:             generation.NewPlanner(),
		Storage:             filesystem,
		Renderers:           rendering.NewRegistry(rendering.RegistryConfig{}),
		AdminSessions:       NewAdminSessionManager(),
		ProviderConfigCodec: providerConfigCodec,
		DeploymentRepo:      deploymentRepo,
		DeploymentManager:   deploymentManager,
	}

	// Wire up IP intelligence enrichment (best-effort; failures do not block startup).
	ipIntelCache := ipintel.NewCache(database)
	mmdbUpdater := ipintel.NewUpdater(
		cfg.MaxMind.AccountID,
		cfg.MaxMind.LicenseKey,
		cfg.MaxMind.DBPath,
		logger,
	)
	result.MMDBUpdater = mmdbUpdater

	var geoIPReader ipintel.GeoIPLookup
	if err := mmdbUpdater.EnsureFresh(ctx); err != nil {
		logger.Info("GeoIP MMDB not available; GeoIP enrichment disabled", "reason", err.Error())
	} else {
		if r, err := ipintel.OpenGeoIP2Reader(cfg.MaxMind.DBPath); err != nil {
			logger.Warn("failed to open GeoIP2 MMDB; GeoIP enrichment disabled", "error", err)
		} else {
			geoIPReader = r
		}
	}

	rdapClient := ipintel.NewRDAPClient(nil)
	ipIntelService := ipintel.NewService(ipIntelCache, geoIPReader, rdapClient)
	result.IPIntelService = ipIntelService
	result.EventService.SetIPEnricher(ipIntelService)
	result.Generation = generation.NewService(generation.ServiceConfig{
		WorldModels: worldmodels.NewRepository(database),
		Planner:     result.Planner,
		ProviderProvider: func() provider.Provider {
			return result.CurrentProvider()
		},
		RenderersProvider: func() rendering.Registry {
			return result.Renderers
		},
		Jobs:             result.JobStore,
		Assets:           result.AssetRepo,
		Storage:          result.Storage,
		LifecycleContext: ctx,
	})

	deploymentManager.RestoreRunning(ctx)

	return result, nil
}

func (a *APIApp) UpdateProvider(cfg config.ProviderConfig) {
	a.providerMu.Lock()
	defer a.providerMu.Unlock()
	a.Config.Provider = cfg
	a.Provider = provider.NewOpenAI(cfg, nil)
}

func (a *APIApp) ProviderState() (config.ProviderConfig, provider.Provider) {
	if a == nil {
		return config.ProviderConfig{}, nil
	}

	a.providerMu.RLock()
	defer a.providerMu.RUnlock()

	return a.Config.Provider, a.Provider
}

func (a *APIApp) ProviderConfig() config.ProviderConfig {
	cfg, _ := a.ProviderState()
	return cfg
}

func (a *APIApp) CurrentProvider() provider.Provider {
	_, currentProvider := a.ProviderState()
	return currentProvider
}

func (a *APIApp) GenerationService() *generation.Service {
	return a.Generation
}

func (a *APIApp) Close() error {
	if a == nil {
		return nil
	}
	if a.DeploymentManager != nil {
		a.DeploymentManager.StopAll(context.Background())
	}
	if a.Generation != nil {
		if err := a.Generation.Close(); err != nil {
			return err
		}
	}
	if a.DB == nil {
		return nil
	}
	return a.DB.Close()
}

func (a *APIApp) Health(ctx context.Context) models.HealthResponse {
	health := models.HealthResponse{
		Status:  "ok",
		Service: a.Config.ServiceName,
		Version: a.Config.ServiceVersion,
	}
	health.Database.Ready = a.databaseReady(ctx)
	return health
}

func (a *APIApp) Status(ctx context.Context) (models.StatusResponse, error) {
	providerConfig := a.ProviderConfig()
	response := models.StatusResponse{
		Service: models.ServiceStatus{
			Name:    a.Config.ServiceName,
			Version: a.Config.ServiceVersion,
		},
		Database: models.DatabaseStatus{
			Ready: a.databaseReady(ctx),
		},
		Provider: models.ProviderStatus{
			Mode:  providerConfig.Mode(),
			Ready: providerConfig.Ready(),
			Model: providerConfig.Model,
		},
		RecentEvents: []models.RecentEventSummary{},
	}

	if !response.Database.Ready {
		return response, nil
	}

	summary, err := a.StatusQueries.ReadStatusSummary(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return response, err
	}
	response.Counts = summary.Counts
	response.RecentEvents = summary.RecentEvents
	response.LatestJob = summary.LatestJob

	return response, nil
}

func (a *APIApp) databaseReady(ctx context.Context) bool {
	return a != nil && a.DB != nil && a.DB.PingContext(ctx) == nil
}

func loadSavedProviderConfig(ctx context.Context, store *appdb.SettingsStore, codec *providerConfigCodec) (*config.ProviderConfig, error) {
	raw, err := store.Get(ctx, "provider")
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var stored struct {
		BaseURL         string `json:"base_url"`
		APIKey          string `json:"api_key,omitempty"`
		EncryptedAPIKey string `json:"encrypted_api_key,omitempty"`
		Model           string `json:"model"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("decode saved provider config: %w", err)
	}

	apiKey := stored.APIKey
	if stored.EncryptedAPIKey != "" {
		apiKey, err = codec.DecryptString(stored.EncryptedAPIKey)
		if err != nil {
			return nil, err
		}
	}

	return &config.ProviderConfig{
		BaseURL: stored.BaseURL,
		APIKey:  apiKey,
		Model:   stored.Model,
	}, nil
}
