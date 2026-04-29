package generation

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/assets"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/rendering"
	"github.com/natet/honeygen/backend/internal/storage"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func TestServiceRunStoresPlainTextWhenProviderReturnsHTMLForTextAssets(t *testing.T) {
	t.Parallel()

	database := newAssetPreviewGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), storedWorldModelForAssetPreviewTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	root := t.TempDir()
	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    htmlForTextGenerationProvider{},
		Jobs:        NewJobStore(database),
		Assets:      assets.NewRepository(database),
		Storage:     storage.NewFilesystem(root),
		Renderers: rendering.NewRegistry(rendering.RegistryConfig{
			PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
		}),
	})

	job, err := service.Run(context.Background(), RunRequest{WorldModelID: "world-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	waitForAssetPreviewJobStatus(t, NewJobStore(database), job.ID, StatusCompleted)

	items, err := assets.NewRepository(database).List(context.Background(), assets.ListOptions{GenerationJobID: job.ID, Limit: 200})
	if err != nil {
		t.Fatalf("List() assets error = %v", err)
	}

	foundTextAsset := false
	filesystem := storage.NewFilesystem(root)
	for _, item := range items {
		if item.RenderedType != "text" {
			continue
		}
		foundTextAsset = true

		content, err := filesystem.Read(context.Background(), item.Path)
		if err != nil {
			t.Fatalf("Read(%q) error = %v", item.Path, err)
		}
		got := string(content)
		if strings.Contains(got, "<h1>") || strings.Contains(got, "<p>") || strings.Contains(got, "<html") {
			t.Fatalf("text asset content = %q, want plain text without HTML markup", got)
		}
		if !strings.Contains(got, "Meeting Notes") {
			t.Fatalf("text asset content = %q, want extracted text content", got)
		}
	}

	if !foundTextAsset {
		t.Fatal("expected at least one text asset")
	}
}

func TestCleanProviderResponseStripsHTMLFromTextAssets(t *testing.T) {
	t.Parallel()

	got := cleanProviderResponse("<!doctype html><html><body><h1>Meeting Notes</h1><p>Action items</p><p>Follow up with IT</p></body></html>", "text")
	want := "Meeting Notes\n\nAction items\n\nFollow up with IT"
	if got != want {
		t.Fatalf("cleanProviderResponse() = %q, want %q", got, want)
	}
}

type htmlForTextGenerationProvider struct{}

func (htmlForTextGenerationProvider) Generate(_ context.Context, request provider.GenerateRequest) (provider.GenerateResponse, error) {
	if request.Metadata["rendered_type"] == "text" {
		return provider.GenerateResponse{
			Content: "<!doctype html><html><body><h1>Meeting Notes</h1><p>Action items</p><p>Follow up with IT</p></body></html>",
		}, nil
	}
	return assetPreviewGenerationStubProvider{}.Generate(context.Background(), request)
}

func (htmlForTextGenerationProvider) Test(context.Context) error { return nil }

type assetPreviewGenerationStubProvider struct{}

func (assetPreviewGenerationStubProvider) Generate(_ context.Context, request provider.GenerateRequest) (provider.GenerateResponse, error) {
	switch request.Metadata["rendered_type"] {
	case "html":
		return provider.GenerateResponse{Content: "# " + request.Metadata["title"] + "\n\nHTML body for " + request.Metadata["category"]}, nil
	case "markdown":
		return provider.GenerateResponse{Content: "# " + request.Metadata["title"] + "\n\nMarkdown body for " + request.Metadata["category"]}, nil
	case "csv":
		return provider.GenerateResponse{Content: "name,email\nAlex,alex@example.test\nJamie,jamie@example.test\n"}, nil
	case "text":
		return provider.GenerateResponse{Content: request.Metadata["title"] + "\n\nPlain text body"}, nil
	case "pdf":
		return provider.GenerateResponse{Content: "<h1>" + request.Metadata["title"] + "</h1><p>PDF body</p>"}, nil
	default:
		return provider.GenerateResponse{Content: request.Metadata["title"]}, nil
	}
}

func (assetPreviewGenerationStubProvider) Test(context.Context) error { return nil }

func newAssetPreviewGenerationTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "generation-test.db")
	database, err := appdb.OpenSQLite(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := appdb.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return database
}

func storedWorldModelForAssetPreviewTest(id string) worldmodels.StoredWorldModel {
	payload := map[string]any{
		"organization": map[string]any{
			"name":         "Northbridge Financial Advisory",
			"industry":     "Financial Services",
			"size":         "mid-size",
			"region":       "United States",
			"domain_theme": "northbridgefinancial.local",
		},
		"branding": map[string]any{
			"tone":   "formal",
			"colors": []string{"#123B5D", "#B58A3B"},
		},
		"departments": []string{"Finance", "Information Technology"},
		"employees": []map[string]any{
			{"name": "Lauren Chen", "role": "Managing Director", "department": "Finance"},
			{"name": "Dylan Brooks", "role": "IT Lead", "department": "Information Technology"},
		},
		"projects":        []string{"Quarterly Portfolio Review", "Endpoint Upgrade Initiative"},
		"document_themes": []string{"policies", "meeting notes", "vendor lists"},
	}
	blob, _ := json.Marshal(payload)

	return worldmodels.StoredWorldModel{
		ID:          id,
		Name:        "Northbridge Financial Advisory",
		Description: "A deterministic test world model",
		JSONBlob:    string(blob),
	}
}

func waitForAssetPreviewJobStatus(t *testing.T, store *JobStore, jobID, wantStatus string) Job {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.Get(context.Background(), jobID)
		if err == nil && job.Status == wantStatus {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}

	job, err := store.Get(context.Background(), jobID)
	if err != nil {
		t.Fatalf("Get() job error = %v", err)
	}
	t.Fatalf("job.Status = %q, want %q", job.Status, wantStatus)
	return Job{}
}
