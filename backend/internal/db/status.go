package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/natet/honeygen/backend/internal/models"
)

type StatusSummary struct {
	Counts    models.StatusCounts
	LatestJob *models.LatestJobSummary
}

type StatusSummaryReader interface {
	ReadStatusSummary(context.Context, time.Time) (StatusSummary, error)
}

type StatusQueries struct {
	db *sql.DB
}

func NewStatusQueries(database *sql.DB) *StatusQueries {
	return &StatusQueries{db: database}
}

func (q *StatusQueries) ReadStatusSummary(ctx context.Context, since time.Time) (StatusSummary, error) {
	assetsCount, err := q.countAssets(ctx)
	if err != nil {
		return StatusSummary{}, err
	}

	recentEventsCount, err := q.countRecentEvents(ctx, since)
	if err != nil {
		return StatusSummary{}, err
	}

	latestJob, err := q.latestJobSummary(ctx)
	if err != nil {
		return StatusSummary{}, err
	}

	return StatusSummary{
		Counts: models.StatusCounts{
			Assets:       assetsCount,
			RecentEvents: recentEventsCount,
		},
		LatestJob: latestJob,
	}, nil
}

func (q *StatusQueries) countAssets(ctx context.Context) (int, error) {
	var count int
	if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM assets`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count assets: %w", err)
	}
	return count, nil
}

func (q *StatusQueries) countRecentEvents(ctx context.Context, since time.Time) (int, error) {
	var count int
	if err := q.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM events WHERE datetime(COALESCE(NULLIF(timestamp, ''), created_at)) >= datetime(?)`,
		since.UTC().Format(time.RFC3339),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count recent events: %w", err)
	}
	return count, nil
}

func (q *StatusQueries) latestJobSummary(ctx context.Context) (*models.LatestJobSummary, error) {
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
	err := q.db.QueryRowContext(ctx, query).Scan(
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
