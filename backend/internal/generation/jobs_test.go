package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func TestJobStoreDeleteRemovesTerminalJob(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	store := NewJobStore(database)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	job, err := store.Create(context.Background(), "world-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.SetRunning(context.Background(), job.ID, Summary{}); err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}
	if _, err := store.SetCompleted(context.Background(), job.ID, Summary{}); err != nil {
		t.Fatalf("SetCompleted() error = %v", err)
	}

	if err := store.Delete(context.Background(), job.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get(context.Background(), job.ID)
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestJobStoreDeleteReturnsNotFoundForMissingJob(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	store := NewJobStore(database)

	err := store.Delete(context.Background(), "job_nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("Delete() error = %v, want %v", err, ErrJobNotFound)
	}
}

func TestJobStoreSetCanceledReturnsNotCancelableWhenJobAlreadyCompleted(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	store := NewJobStore(database)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	job, err := store.Create(context.Background(), "world-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	job, err = store.SetRunning(context.Background(), job.ID, Summary{ManifestCount: 1})
	if err != nil {
		t.Fatalf("SetRunning() error = %v", err)
	}

	if _, err := store.SetCompleted(context.Background(), job.ID, Summary{ManifestCount: 1, AssetCount: 1}); err != nil {
		t.Fatalf("SetCompleted() error = %v", err)
	}

	canceledJob, err := store.SetCanceled(context.Background(), job.ID, Summary{ManifestCount: 1, AssetCount: 1}, "generation canceled")
	if !errors.Is(err, ErrJobNotCancelable) {
		t.Fatalf("SetCanceled() error = %v, want %v", err, ErrJobNotCancelable)
	}
	if canceledJob.Status != StatusCompleted {
		t.Fatalf("canceledJob.Status = %q, want %q", canceledJob.Status, StatusCompleted)
	}
}
