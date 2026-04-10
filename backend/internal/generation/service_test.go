package generation

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/assets"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/rendering"
	"github.com/natet/honeygen/backend/internal/storage"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func TestServiceRunPersistsJobsAssetsAndFiles(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	root := t.TempDir()
	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    generationStubProvider{},
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
	if job.Status != StatusCompleted {
		t.Fatalf("job.Status = %q, want %q", job.Status, StatusCompleted)
	}
	if job.CompletedAt == nil {
		t.Fatal("job.CompletedAt is nil")
	}
	if len(job.Summary.Logs) == 0 {
		t.Fatal("job.Summary.Logs is empty")
	}

	storedJob, err := NewJobStore(database).Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() job error = %v", err)
	}
	if storedJob.Status != StatusCompleted {
		t.Fatalf("storedJob.Status = %q, want %q", storedJob.Status, StatusCompleted)
	}
	if storedJob.Summary.AssetCount == 0 || storedJob.Summary.ManifestCount == 0 {
		t.Fatalf("storedJob.Summary = %+v, want non-zero counts", storedJob.Summary)
	}

	items, err := assets.NewRepository(database).List(context.Background(), assets.ListOptions{GenerationJobID: job.ID, Limit: 200})
	if err != nil {
		t.Fatalf("List() assets error = %v", err)
	}
	if len(items) == 0 {
		t.Fatal("List() returned no assets")
	}

	var sawHTML, sawMarkdown, sawCSV, sawText, sawPDF bool
	expectedPrefix := "generated/world-1/" + job.ID + "/"
	for _, item := range items {
		if !strings.HasPrefix(item.Path, expectedPrefix) {
			t.Fatalf("asset path = %q, want prefix %q", item.Path, expectedPrefix)
		}
		fullPath := filepath.Join(root, filepath.FromSlash(item.Path))
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("generated file %q missing: %v", fullPath, err)
		}
		switch item.RenderedType {
		case "html":
			sawHTML = true
		case "markdown":
			sawMarkdown = true
		case "csv":
			sawCSV = true
		case "text":
			sawText = true
		case "pdf":
			sawPDF = true
			if item.Previewable {
				t.Fatal("pdf asset must not be previewable")
			}
		}
	}
	if !sawHTML || !sawMarkdown || !sawCSV || !sawText || !sawPDF {
		t.Fatalf("rendered types coverage html=%t markdown=%t csv=%t text=%t pdf=%t", sawHTML, sawMarkdown, sawCSV, sawText, sawPDF)
	}
}

func TestServiceRunMarksJobFailedWhenProviderErrors(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    failingGenerationProvider{},
		Jobs:        NewJobStore(database),
		Assets:      assets.NewRepository(database),
		Storage:     storage.NewFilesystem(t.TempDir()),
		Renderers: rendering.NewRegistry(rendering.RegistryConfig{
			PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
		}),
	})

	job, err := service.Run(context.Background(), RunRequest{WorldModelID: "world-1"})
	if err == nil {
		t.Fatal("Run() error = nil, want provider failure")
	}
	if job.Status != StatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, StatusFailed)
	}
	if !strings.Contains(job.ErrorMessage, "provider request failed") {
		t.Fatalf("job.ErrorMessage = %q, want provider failure", job.ErrorMessage)
	}
}

func newGenerationTestDatabase(t *testing.T) *sql.DB {
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

func StoredWorldModelForTest(id string) worldmodels.StoredWorldModel {
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

type generationStubProvider struct{}

func (generationStubProvider) Generate(_ context.Context, request provider.GenerateRequest) (provider.GenerateResponse, error) {
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

func (generationStubProvider) Test(context.Context) error { return nil }

type failingGenerationProvider struct{}

func (failingGenerationProvider) Generate(context.Context, provider.GenerateRequest) (provider.GenerateResponse, error) {
	return provider.GenerateResponse{}, &provider.Error{
		Kind:    provider.KindConnectivity,
		Message: "provider request failed",
	}
}

func (failingGenerationProvider) Test(context.Context) error { return nil }
