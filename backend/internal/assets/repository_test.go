package assets

import (
	"context"
	"database/sql"
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
