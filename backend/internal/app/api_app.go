package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/config"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/deployments"
	"github.com/natet/honeygen/backend/internal/events"
	"github.com/natet/honeygen/backend/internal/generation"
	"github.com/natet/honeygen/backend/internal/models"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/rendering"
	"github.com/natet/honeygen/backend/internal/storage"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

type APIApp struct {
	Config        config.Config
	Logger        *slog.Logger
	DB            *sql.DB
	StatusQueries appdb.StatusSummaryReader
	Settings      *appdb.SettingsStore
	WorldModels   *worldmodels.Service
	Provider      provider.Provider
	AssetRepo     *assets.Repository
	EventRepo     *events.Repository
	EventService  *events.Service
	JobStore      *generation.JobStore
	Planner       *generation.Planner
	Storage       *storage.Filesystem
	Renderers     rendering.Registry
	DeploymentRepo    *deployments.Repository
	DeploymentManager *deployments.Manager
}

func NewAPIApp(ctx context.Context, cfg config.Config, logger *slog.Logger) (*APIApp, error) {
	if logger == nil {
		logger = slog.Default()
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

	// Derive the API base URL for deployment event forwarding
	apiBaseURL := fmt.Sprintf("http://localhost%s", cfg.HTTPAddr)
	deploymentManager := deployments.NewManager(deploymentRepo, cfg.GeneratedAssetsDir, logger, cfg.InternalEventIngestToken, apiBaseURL)

	// Load saved provider settings (overlay on top of env/config file values)
	savedProvider, err := loadSavedProviderConfig(ctx, settingsStore)
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
		Config:        cfg,
		Logger:        logger,
		DB:            database,
		StatusQueries: appdb.NewStatusQueries(database),
		Settings:      settingsStore,
		WorldModels:   worldModelService,
		Provider:      provider.NewOpenAI(cfg.Provider, nil),
		AssetRepo:     assetRepo,
		EventRepo:     eventRepo,
		EventService:  events.NewService(eventRepo, assetRepo),
		JobStore:      jobStore,
		Planner:       generation.NewPlanner(),
		Storage:       filesystem,
		Renderers:     rendering.NewRegistry(rendering.RegistryConfig{}),
		DeploymentRepo:    deploymentRepo,
		DeploymentManager: deploymentManager,
	}

	deploymentManager.RestoreRunning(ctx)

	return result, nil
}

func (a *APIApp) UpdateProvider(cfg config.ProviderConfig) {
	a.Config.Provider = cfg
	a.Provider = provider.NewOpenAI(cfg, nil)
}

func (a *APIApp) GenerationService() *generation.Service {
	return generation.NewService(generation.ServiceConfig{
		WorldModels: worldmodels.NewRepository(a.DB),
		Planner:     a.Planner,
		Provider:    a.Provider,
		Jobs:        a.JobStore,
		Assets:      a.AssetRepo,
		Storage:     a.Storage,
		Renderers:   a.Renderers,
	})
}

func (a *APIApp) Close() error {
	if a == nil {
		return nil
	}
	if a.DeploymentManager != nil {
		a.DeploymentManager.StopAll(context.Background())
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
	response := models.StatusResponse{
		Service: models.ServiceStatus{
			Name:    a.Config.ServiceName,
			Version: a.Config.ServiceVersion,
		},
		Database: models.DatabaseStatus{
			Ready: a.databaseReady(ctx),
			Path:  a.Config.SQLitePath,
		},
		Storage: models.StorageStatus{
			Root:               a.Config.StorageRoot,
			GeneratedAssetsDir: a.Config.GeneratedAssetsDir,
		},
		Provider: models.ProviderStatus{
			Mode:    a.Config.Provider.Mode(),
			Ready:   a.Config.Provider.Ready(),
			BaseURL: a.Config.Provider.BaseURL,
			Model:   a.Config.Provider.Model,
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

func loadSavedProviderConfig(ctx context.Context, store *appdb.SettingsStore) (*config.ProviderConfig, error) {
	raw, err := store.Get(ctx, "provider")
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var cfg config.ProviderConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode saved provider config: %w", err)
	}
	return &cfg, nil
}
