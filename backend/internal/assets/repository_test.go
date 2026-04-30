package assets

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	appdb "github.com/natet/honeygen/backend/internal/db"
)

func TestRepositoryTreeIncludesAllMatchingAssets(t *testing.T) {
	t.Parallel()

	repository := NewRepository(newAssetsTestDatabase(t))
	ctx := context.Background()

	const totalAssets = 10001
	for i := 0; i < totalAssets; i++ {
		if _, err := repository.Create(ctx, Asset{
			ID:              fmt.Sprintf("asset-%05d", i),
			GenerationJobID: "job-1",
			WorldModelID:    "world-1",
			SourceType:      "generated",
			RenderedType:    "text",
			Path:            fmt.Sprintf("generated/world-1/job-1/public/file-%05d.txt", i),
			MIMEType:        "text/plain",
			SizeBytes:       int64(i + 1),
			Previewable:     true,
			Checksum:        fmt.Sprintf("sum-%05d", i),
		}); err != nil {
			t.Fatalf("Create() asset %d error = %v", i, err)
		}
	}

	tree, err := repository.Tree(ctx, ListOptions{GenerationJobID: "job-1"})
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if len(tree) != 1 || tree[0].Name != "public" {
		t.Fatalf("tree roots = %+v, want single public root", tree)
	}
	if got := len(tree[0].Children); got != totalAssets {
		t.Fatalf("len(public children) = %d, want %d", got, totalAssets)
	}
}

func TestDisplayPathTrimsGeneratedPrefixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		item    Asset
		options ListOptions
		want    string
	}{
		{
			name: "generation tree trims generated world and job prefix",
			item: Asset{Path: "generated/world-1/job-1/public/report.txt", WorldModelID: "world-1"},
			options: ListOptions{
				GenerationJobID: "job-1",
			},
			want: "public/report.txt",
		},
		{
			name: "world tree trims generated world prefix",
			item: Asset{Path: "generated/world-1/job-1/public/report.txt", WorldModelID: "world-1"},
			options: ListOptions{
				WorldModelID: "world-1",
			},
			want: "job-1/public/report.txt",
		},
		{
			name: "world tree trims bare world prefix",
			item: Asset{Path: "world-1/job-1/public/report.txt", WorldModelID: "world-1"},
			options: ListOptions{
				WorldModelID: "world-1",
			},
			want: "job-1/public/report.txt",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := displayPath(tc.item, tc.options); got != tc.want {
				t.Fatalf("displayPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRepositoryDeleteByJobIDRemovesOnlyTargetJobAssets(t *testing.T) {
	t.Parallel()

	repository := NewRepository(newAssetsTestDatabase(t))
	ctx := context.Background()

	makeAsset := func(id, jobID, path string) Asset {
		return Asset{
			ID:              id,
			GenerationJobID: jobID,
			WorldModelID:    "world-1",
			SourceType:      "generated",
			RenderedType:    "text",
			Path:            path,
			MIMEType:        "text/plain",
			SizeBytes:       10,
			Previewable:     true,
			Checksum:        "abc",
		}
	}

	for _, a := range []Asset{
		makeAsset("a1", "job-1", "generated/world-1/job-1/public/file1.txt"),
		makeAsset("a2", "job-1", "generated/world-1/job-1/public/file2.txt"),
		makeAsset("a3", "job-2", "generated/world-1/job-2/public/file3.txt"),
	} {
		if _, err := repository.Create(ctx, a); err != nil {
			t.Fatalf("Create() asset %q error = %v", a.ID, err)
		}
	}

	if err := repository.DeleteByJobID(ctx, "job-1"); err != nil {
		t.Fatalf("DeleteByJobID() error = %v", err)
	}

	// job-1 assets must be gone.
	remaining, err := repository.List(ctx, ListOptions{GenerationJobID: "job-1", Limit: 100})
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("List() after delete = %d assets, want 0", len(remaining))
	}

	// job-2 assets must be untouched.
	other, err := repository.List(ctx, ListOptions{GenerationJobID: "job-2", Limit: 100})
	if err != nil {
		t.Fatalf("List() job-2 after delete error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("List() job-2 after delete = %d assets, want 1", len(other))
	}
}

func TestRepositoryDeleteByJobIDIsNoOpForUnknownJob(t *testing.T) {
	t.Parallel()

	repository := NewRepository(newAssetsTestDatabase(t))
	ctx := context.Background()

	if err := repository.DeleteByJobID(ctx, "nonexistent-job"); err != nil {
		t.Fatalf("DeleteByJobID() nonexistent job error = %v, want nil", err)
	}
}

func TestRepositoryDeleteByJobIDDeletesAllAssetsForLargeJob(t *testing.T) {
	t.Parallel()

	repository := NewRepository(newAssetsTestDatabase(t))
	ctx := context.Background()

	const count = 500
	for i := 0; i < count; i++ {
		if _, err := repository.Create(ctx, Asset{
			ID:              fmt.Sprintf("asset-%04d", i),
			GenerationJobID: "job-bulk",
			WorldModelID:    "world-1",
			SourceType:      "generated",
			RenderedType:    "text",
			Path:            fmt.Sprintf("generated/world-1/job-bulk/public/file-%04d.txt", i),
			MIMEType:        "text/plain",
			SizeBytes:       1,
			Checksum:        fmt.Sprintf("sum-%04d", i),
		}); err != nil {
			t.Fatalf("Create() asset %d error = %v", i, err)
		}
	}

	if err := repository.DeleteByJobID(ctx, "job-bulk"); err != nil {
		t.Fatalf("DeleteByJobID() error = %v", err)
	}

	remaining, err := repository.List(ctx, ListOptions{GenerationJobID: "job-bulk", Limit: 1000})
	if err != nil {
		t.Fatalf("List() after bulk delete error = %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("List() after bulk delete = %d assets, want 0", len(remaining))
	}
}

func TestRepositoryDeleteClearsEventReferencesBeforeDeletingAsset(t *testing.T) {
	t.Parallel()

	database := newAssetsTestDatabase(t)
	repository := NewRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO world_models (id, name, description, json_blob, created_at, updated_at)
		VALUES ('world-1', 'World 1', '', '{}', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`); err != nil {
		t.Fatalf("seed world model error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO generation_jobs (id, world_model_id, status, created_at, updated_at)
		VALUES ('job-1', 'world-1', 'completed', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`); err != nil {
		t.Fatalf("seed generation job error = %v", err)
	}

	created, err := repository.Create(ctx, Asset{
		ID:              "asset-1",
		GenerationJobID: "job-1",
		WorldModelID:    "world-1",
		SourceType:      "generated",
		RenderedType:    "text",
		Path:            "generated/world-1/job-1/public/file.txt",
		MIMEType:        "text/plain",
		SizeBytes:       5,
		Previewable:     true,
		Checksum:        "sum-1",
	})
	if err != nil {
		t.Fatalf("Create() asset error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO events (id, asset_id, world_model_id, event_type, path, source_ip, user_agent, metadata_json)
		VALUES ('event-1', ?, 'world-1', 'asset.requested', '/generated/world-1/job-1/public/file.txt', '127.0.0.1', 'test-agent', '{}')
	`, created.ID); err != nil {
		t.Fatalf("seed event error = %v", err)
	}

	if err := repository.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := repository.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want %v", err, ErrNotFound)
	}

	var eventAssetID sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT asset_id FROM events WHERE id = 'event-1'`).Scan(&eventAssetID); err != nil {
		t.Fatalf("query event asset_id error = %v", err)
	}
	if eventAssetID.Valid {
		t.Fatalf("event asset_id valid = %t, want false (value=%q)", eventAssetID.Valid, eventAssetID.String)
	}
}

func TestRepositoryDeleteByJobIDClearsEventReferencesBeforeDeletingAssets(t *testing.T) {
	t.Parallel()

	database := newAssetsTestDatabase(t)
	repository := NewRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO world_models (id, name, description, json_blob, created_at, updated_at)
		VALUES ('world-1', 'World 1', '', '{}', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`); err != nil {
		t.Fatalf("seed world model error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO generation_jobs (id, world_model_id, status, created_at, updated_at)
		VALUES
			('job-1', 'world-1', 'completed', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
			('job-2', 'world-1', 'completed', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`); err != nil {
		t.Fatalf("seed generation jobs error = %v", err)
	}

	for _, asset := range []Asset{
		{
			ID:              "asset-1",
			GenerationJobID: "job-1",
			WorldModelID:    "world-1",
			SourceType:      "generated",
			RenderedType:    "text",
			Path:            "generated/world-1/job-1/public/file-1.txt",
			MIMEType:        "text/plain",
			SizeBytes:       5,
			Previewable:     true,
			Checksum:        "sum-1",
		},
		{
			ID:              "asset-2",
			GenerationJobID: "job-2",
			WorldModelID:    "world-1",
			SourceType:      "generated",
			RenderedType:    "text",
			Path:            "generated/world-1/job-2/public/file-2.txt",
			MIMEType:        "text/plain",
			SizeBytes:       5,
			Previewable:     true,
			Checksum:        "sum-2",
		},
	} {
		if _, err := repository.Create(ctx, asset); err != nil {
			t.Fatalf("Create(%q) error = %v", asset.ID, err)
		}
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO events (id, asset_id, world_model_id, event_type, path, source_ip, user_agent, metadata_json)
		VALUES
			('event-1', 'asset-1', 'world-1', 'asset.requested', '/generated/world-1/job-1/public/file-1.txt', '127.0.0.1', 'test-agent', '{}'),
			('event-2', 'asset-2', 'world-1', 'asset.requested', '/generated/world-1/job-2/public/file-2.txt', '127.0.0.1', 'test-agent', '{}')
	`); err != nil {
		t.Fatalf("seed events error = %v", err)
	}

	if err := repository.DeleteByJobID(ctx, "job-1"); err != nil {
		t.Fatalf("DeleteByJobID() error = %v", err)
	}

	if _, err := repository.Get(ctx, "asset-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(asset-1) after DeleteByJobID() error = %v, want %v", err, ErrNotFound)
	}
	if _, err := repository.Get(ctx, "asset-2"); err != nil {
		t.Fatalf("Get(asset-2) after DeleteByJobID() error = %v, want asset to remain", err)
	}

	var deletedEventAssetID sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT asset_id FROM events WHERE id = 'event-1'`).Scan(&deletedEventAssetID); err != nil {
		t.Fatalf("query deleted event asset_id error = %v", err)
	}
	if deletedEventAssetID.Valid {
		t.Fatalf("deleted event asset_id valid = %t, want false (value=%q)", deletedEventAssetID.Valid, deletedEventAssetID.String)
	}

	var remainingEventAssetID sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT asset_id FROM events WHERE id = 'event-2'`).Scan(&remainingEventAssetID); err != nil {
		t.Fatalf("query remaining event asset_id error = %v", err)
	}
	if !remainingEventAssetID.Valid || remainingEventAssetID.String != "asset-2" {
		t.Fatalf("remaining event asset_id = %+v, want asset-2", remainingEventAssetID)
	}
}

func TestRepositoryCreateRejectsDuplicatePath(t *testing.T) {
	t.Parallel()

	repository := NewRepository(newAssetsTestDatabase(t))
	ctx := context.Background()

	first := Asset{
		ID:              "asset-1",
		GenerationJobID: "job-1",
		WorldModelID:    "world-1",
		SourceType:      "generated",
		RenderedType:    "text",
		Path:            "generated/world-1/job-1/public/duplicate.txt",
		MIMEType:        "text/plain",
		SizeBytes:       5,
		Previewable:     true,
		Checksum:        "sum-1",
	}
	if _, err := repository.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	_, err := repository.Create(ctx, Asset{
		ID:              "asset-2",
		GenerationJobID: "job-1",
		WorldModelID:    "world-1",
		SourceType:      "upload",
		RenderedType:    "text",
		Path:            first.Path,
		MIMEType:        "text/plain",
		SizeBytes:       6,
		Previewable:     true,
		Checksum:        "sum-2",
	})
	if err == nil {
		t.Fatal("Create(duplicate) error = nil, want ErrPathConflict")
	}
	if !errors.Is(err, ErrPathConflict) {
		t.Fatalf("Create(duplicate) error = %v, want %v", err, ErrPathConflict)
	}
}

func newAssetsTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := appdb.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return database
}
