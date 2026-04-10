package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/models"
)

type APIApp struct {
	Config          config.Config
	Logger          *slog.Logger
	DB              *sql.DB
	MigrationsReady bool
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

	return &APIApp{
		Config:          cfg,
		Logger:          logger,
		DB:              database,
		MigrationsReady: true,
	}, nil
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

	assetsCount, err := a.countAssets(ctx)
	if err != nil {
		return response, err
	}
	recentEventsCount, err := a.countRecentEvents(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return response, err
	}
	response.Counts = models.StatusCounts{
		Assets:       assetsCount,
		RecentEvents: recentEventsCount,
	}

	latestJob, err := a.latestJobSummary(ctx)
	if err != nil {
		return response, err
	}
	response.LatestJob = latestJob

	return response, nil
}

func (a *APIApp) databaseReady(ctx context.Context) bool {
	return a != nil && a.DB != nil && a.MigrationsReady && a.DB.PingContext(ctx) == nil
}

func (a *APIApp) countAssets(ctx context.Context) (int, error) {
	var count int
	if err := a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM assets`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count assets: %w", err)
	}
	return count, nil
}

func (a *APIApp) countRecentEvents(ctx context.Context, since time.Time) (int, error) {
	var count int
	if err := a.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM events WHERE datetime(timestamp) >= datetime(?)`,
		since.UTC().Format(time.RFC3339),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count recent events: %w", err)
	}
	return count, nil
}

func (a *APIApp) latestJobSummary(ctx context.Context) (*models.LatestJobSummary, error) {
	const query = `
SELECT
	j.id,
	j.world_model_id,
	j.status,
	COALESCE(j.completed_at, ''),
	(
		SELECT COUNT(*)
		FROM assets AS a
		WHERE a.generation_job_id = j.id
	) AS asset_count
FROM generation_jobs AS j
ORDER BY j.created_at DESC
LIMIT 1
`

	var job models.LatestJobSummary
	err := a.DB.QueryRowContext(ctx, query).Scan(
		&job.ID,
		&job.WorldModelID,
		&job.Status,
		&job.CompletedAt,
		&job.AssetCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest job: %w", err)
	}
	return &job, nil
}
