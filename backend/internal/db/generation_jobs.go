package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/natet/honeygen/backend/internal/provider"
)

type GenerationJobRecorder struct {
	db *sql.DB
}

func NewGenerationJobRecorder(database *sql.DB) *GenerationJobRecorder {
	return &GenerationJobRecorder{db: database}
}

func (r *GenerationJobRecorder) RecordProviderFailure(ctx context.Context, jobID string, err error) error {
	result, execErr := r.db.ExecContext(
		ctx,
		`UPDATE generation_jobs
		SET error_message = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		provider.SafeErrorMessage(err),
		jobID,
	)
	if execErr != nil {
		return fmt.Errorf("update generation job error message: %w", execErr)
	}

	rowsAffected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("inspect generation job update: %w", rowsErr)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("generation job %q not found: %w", jobID, sql.ErrNoRows)
	}

	return nil
}
