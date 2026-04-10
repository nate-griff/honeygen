package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/config"
	appdb "github.com/natet/honeygen/backend/internal/db"
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
	WorldModels   *worldmodels.Service
	Provider      provider.Provider
	AssetRepo     *assets.Repository
	EventRepo     *events.Repository
	EventService  *events.Service
	JobStore      *generation.JobStore
	Planner       *generation.Planner
	Storage       *storage.Filesystem
	Renderers     rendering.Registry
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
	filesystem := storage.NewFilesystem(cfg.StorageRoot)

	return &APIApp{
		Config:        cfg,
		Logger:        logger,
		DB:            database,
		StatusQueries: appdb.NewStatusQueries(database),
		WorldModels:   worldModelService,
		Provider:      provider.NewOpenAI(cfg.Provider, nil),
		AssetRepo:     assetRepo,
		EventRepo:     eventRepo,
		EventService:  events.NewService(eventRepo, assetRepo),
		JobStore:      jobStore,
		Planner:       generation.NewPlanner(),
		Storage:       filesystem,
		Renderers:     rendering.NewRegistry(rendering.RegistryConfig{}),
	}, nil
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
	if a == nil || a.DB == nil {
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
	}

	if !response.Database.Ready {
		return response, nil
	}

	summary, err := a.StatusQueries.ReadStatusSummary(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return response, err
	}
	response.Counts = summary.Counts
	response.LatestJob = summary.LatestJob

	return response, nil
}

func (a *APIApp) databaseReady(ctx context.Context) bool {
	return a != nil && a.DB != nil && a.DB.PingContext(ctx) == nil
}
