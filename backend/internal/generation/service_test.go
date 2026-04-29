package generation

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
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
	if job.Status != StatusRunning {
		t.Fatalf("job.Status = %q, want %q", job.Status, StatusRunning)
	}
	if job.CompletedAt != nil {
		t.Fatalf("job.CompletedAt = %v, want nil before background work finishes", job.CompletedAt)
	}
	if job.StartedAt == nil {
		t.Fatal("job.StartedAt is nil")
	}
	if len(job.Summary.Logs) == 0 {
		t.Fatal("job.Summary.Logs is empty")
	}

	storedJob := waitForJobStatus(t, NewJobStore(database), job.ID, StatusCompleted)
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
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if job.Status != StatusRunning {
		t.Fatalf("job.Status = %q, want %q", job.Status, StatusRunning)
	}

	failedJob := waitForJobStatus(t, NewJobStore(database), job.ID, StatusFailed)
	if failedJob.Status != StatusFailed {
		t.Fatalf("failedJob.Status = %q, want %q", failedJob.Status, StatusFailed)
	}
	if !strings.Contains(failedJob.ErrorMessage, "provider request failed") {
		t.Fatalf("failedJob.ErrorMessage = %q, want provider failure", failedJob.ErrorMessage)
	}
}

func TestServiceRunReturnsBeforeBackgroundGenerationCompletes(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	provider := blockingGenerationProvider{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    provider,
		Jobs:        NewJobStore(database),
		Assets:      assets.NewRepository(database),
		Storage:     storage.NewFilesystem(t.TempDir()),
		Renderers: rendering.NewRegistry(rendering.RegistryConfig{
			PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
		}),
	})

	job, err := service.Run(context.Background(), RunRequest{WorldModelID: "world-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if job.Status != StatusRunning {
		t.Fatalf("job.Status = %q, want %q", job.Status, StatusRunning)
	}

	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background generation did not start")
	}

	storedJob, err := NewJobStore(database).Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() job error = %v", err)
	}
	if storedJob.Status != StatusRunning {
		t.Fatalf("storedJob.Status = %q, want %q while provider is blocked", storedJob.Status, StatusRunning)
	}

	close(provider.release)

	completedJob := waitForJobStatus(t, NewJobStore(database), job.ID, StatusCompleted)
	if completedJob.CompletedAt == nil {
		t.Fatal("completedJob.CompletedAt is nil")
	}
}

func TestServiceCancelMarksRunningJobCanceled(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	provider := cancelableBlockingGenerationProvider{
		started:  make(chan struct{}, 1),
		canceled: make(chan struct{}, 1),
	}
	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    provider,
		Jobs:        NewJobStore(database),
		Assets:      assets.NewRepository(database),
		Storage:     storage.NewFilesystem(t.TempDir()),
		Renderers: rendering.NewRegistry(rendering.RegistryConfig{
			PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
		}),
	})
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	job, err := service.Run(context.Background(), RunRequest{WorldModelID: "world-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	waitForSignal(t, provider.started, "background generation did not start")

	canceledJob, err := service.Cancel(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if canceledJob.Status != StatusCanceled {
		t.Fatalf("canceledJob.Status = %q, want %q", canceledJob.Status, StatusCanceled)
	}

	waitForSignal(t, provider.canceled, "provider context was not canceled")

	storedJob := waitForJobStatus(t, NewJobStore(database), job.ID, StatusCanceled)
	if storedJob.CompletedAt == nil {
		t.Fatal("storedJob.CompletedAt is nil")
	}
	if !strings.Contains(strings.ToLower(storedJob.ErrorMessage), "canceled") {
		t.Fatalf("storedJob.ErrorMessage = %q, want cancellation message", storedJob.ErrorMessage)
	}
	if len(storedJob.Summary.Logs) == 0 {
		t.Fatal("storedJob.Summary.Logs is empty")
	}
	if !strings.Contains(strings.ToLower(storedJob.Summary.Logs[len(storedJob.Summary.Logs)-1].Message), "canceled") {
		t.Fatalf("last log message = %q, want cancellation entry", storedJob.Summary.Logs[len(storedJob.Summary.Logs)-1].Message)
	}
}

func TestServiceCancelDuringPersistenceKeepsPersistedAssetAndSummaryConsistent(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	root := t.TempDir()
	assetsRepo := assets.NewRepository(database)
	assetsSpy := &blockingAssetWriter{
		repo:      assetsRepo,
		attempted: make(chan struct{}, 1),
		release:   make(chan struct{}),
	}

	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    generationStubProvider{},
		Jobs:        NewJobStore(database),
		Assets:      assetsSpy,
		Storage:     storage.NewFilesystem(root),
		Renderers: rendering.NewRegistry(rendering.RegistryConfig{
			PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
		}),
	})
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	job, err := service.Run(context.Background(), RunRequest{WorldModelID: "world-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	waitForSignal(t, assetsSpy.attempted, "asset persistence did not start")

	canceledJob, err := service.Cancel(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if canceledJob.Status != StatusCanceled {
		t.Fatalf("canceledJob.Status = %q, want %q", canceledJob.Status, StatusCanceled)
	}
	close(assetsSpy.release)

	storedJob, items := waitForCanceledJobAssetCount(t, NewJobStore(database), assetsRepo, job.ID, 1)
	if storedJob.Summary.AssetCount != len(items) {
		t.Fatalf("storedJob.Summary.AssetCount = %d, want %d", storedJob.Summary.AssetCount, len(items))
	}
	fullPath := filepath.Join(root, filepath.FromSlash(items[0].Path))
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("persisted file %q missing: %v", fullPath, err)
	}
}

func TestServiceCloseCancelsRunningJobs(t *testing.T) {
	t.Parallel()

	database := newGenerationTestDatabase(t)
	repo := worldmodels.NewRepository(database)
	if _, err := repo.Create(context.Background(), StoredWorldModelForTest("world-1")); err != nil {
		t.Fatalf("Create() world model error = %v", err)
	}

	provider := cancelableBlockingGenerationProvider{
		started:  make(chan struct{}, 1),
		canceled: make(chan struct{}, 1),
	}
	service := NewService(ServiceConfig{
		WorldModels: repo,
		Planner:     NewPlanner(),
		Provider:    provider,
		Jobs:        NewJobStore(database),
		Assets:      assets.NewRepository(database),
		Storage:     storage.NewFilesystem(t.TempDir()),
		Renderers: rendering.NewRegistry(rendering.RegistryConfig{
			PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
		}),
	})

	job, err := service.Run(context.Background(), RunRequest{WorldModelID: "world-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	waitForSignal(t, provider.started, "background generation did not start")

	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	waitForSignal(t, provider.canceled, "provider context was not canceled during shutdown")

	storedJob := waitForJobStatus(t, NewJobStore(database), job.ID, StatusCanceled)
	if storedJob.CompletedAt == nil {
		t.Fatal("storedJob.CompletedAt is nil")
	}
}

func newGenerationTestDatabase(t *testing.T) *sql.DB {
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

type blockingGenerationProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p blockingGenerationProvider) Generate(_ context.Context, request provider.GenerateRequest) (provider.GenerateResponse, error) {
	select {
	case p.started <- struct{}{}:
	default:
	}

	<-p.release

	return generationStubProvider{}.Generate(context.Background(), request)
}

func (blockingGenerationProvider) Test(context.Context) error { return nil }

type cancelableBlockingGenerationProvider struct {
	started  chan struct{}
	canceled chan struct{}
}

func (p cancelableBlockingGenerationProvider) Generate(ctx context.Context, request provider.GenerateRequest) (provider.GenerateResponse, error) {
	select {
	case p.started <- struct{}{}:
	default:
	}

	<-ctx.Done()

	select {
	case p.canceled <- struct{}{}:
	default:
	}

	return provider.GenerateResponse{}, ctx.Err()
}

func (cancelableBlockingGenerationProvider) Test(context.Context) error { return nil }

type blockingAssetWriter struct {
	repo      *assets.Repository
	attempted chan struct{}
	release   chan struct{}
}

func (w *blockingAssetWriter) Create(ctx context.Context, asset assets.Asset) (assets.Asset, error) {
	select {
	case w.attempted <- struct{}{}:
	default:
	}

	<-w.release
	return w.repo.Create(ctx, asset)
}

func waitForJobStatus(t *testing.T, store *JobStore, jobID, wantStatus string) Job {
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

func waitForCanceledJobAssetCount(t *testing.T, store *JobStore, repo *assets.Repository, jobID string, wantCount int) (Job, []assets.Asset) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, jobErr := store.Get(context.Background(), jobID)
		items, assetsErr := repo.List(context.Background(), assets.ListOptions{GenerationJobID: jobID, Limit: 10})
		if jobErr == nil && assetsErr == nil && job.Status == StatusCanceled && job.Summary.AssetCount == wantCount && len(items) == wantCount {
			return job, items
		}
		time.Sleep(10 * time.Millisecond)
	}

	job, err := store.Get(context.Background(), jobID)
	if err != nil {
		t.Fatalf("Get() job error = %v", err)
	}
	items, err := repo.List(context.Background(), assets.ListOptions{GenerationJobID: jobID, Limit: 10})
	if err != nil {
		t.Fatalf("List() assets error = %v", err)
	}
	t.Fatalf("job.Status = %q asset_count=%d listed_assets=%d, want canceled and %d assets", job.Status, job.Summary.AssetCount, len(items), wantCount)
	return Job{}, nil
}

func waitForSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(message)
	}
}

func TestCleanProviderResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		renderedType string
		want         string
	}{
		{
			name:         "no fences",
			content:      "<h1>Hello</h1>\n<p>World</p>",
			renderedType: "html",
			want:         "<h1>Hello</h1>\n<p>World</p>",
		},
		{
			name:         "html fences",
			content:      "```html\n<!DOCTYPE html>\n<html><body>test</body></html>\n```",
			renderedType: "html",
			want:         "<!DOCTYPE html>\n<html><body>test</body></html>",
		},
		{
			name:         "markdown fences",
			content:      "```markdown\n# Title\n\nSome content\n```",
			renderedType: "markdown",
			want:         "# Title\n\nSome content",
		},
		{
			name:         "csv fences",
			content:      "```csv\nname,email\nAlice,alice@example.com\n```",
			renderedType: "csv",
			want:         "name,email\nAlice,alice@example.com",
		},
		{
			name:         "generic fence without language",
			content:      "```\nplain content\n```",
			renderedType: "text",
			want:         "plain content",
		},
		{
			name:         "empty content",
			content:      "",
			renderedType: "html",
			want:         "",
		},
		{
			name:         "whitespace around fences",
			content:      "  \n```html\n<p>Content</p>\n```\n  ",
			renderedType: "html",
			want:         "<p>Content</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanProviderResponse(tt.content, tt.renderedType)
			if got != tt.want {
				t.Errorf("cleanProviderResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildPromptIncludesColors(t *testing.T) {
	t.Parallel()

	model := worldmodels.WorldModel{
		Organization: worldmodels.Organization{
			Name:     "Test Corp",
			Industry: "Technology",
			Region:   "US",
		},
		Branding: worldmodels.Branding{
			Tone:   "formal",
			Colors: []string{"#123B5D", "#B58A3B"},
		},
		Departments: []string{"Engineering"},
		Projects:    []string{"Alpha"},
	}
	entry := ManifestEntry{
		Title:        "About Page",
		Category:     "public-about-page",
		Path:         "public/about.html",
		RenderedType: "html",
		PromptHint:   "Generate an about page",
	}

	prompt := buildPrompt(model, entry)

	if !strings.Contains(prompt, "#123B5D") {
		t.Error("prompt should contain brand color #123B5D")
	}
	if !strings.Contains(prompt, "#B58A3B") {
		t.Error("prompt should contain brand color #B58A3B")
	}
	if !strings.Contains(prompt, "Brand colors:") {
		t.Error("prompt should contain 'Brand colors:' label")
	}
}

func TestBuildPromptOmitsColorsWhenEmpty(t *testing.T) {
	t.Parallel()

	model := worldmodels.WorldModel{
		Organization: worldmodels.Organization{
			Name:     "Test Corp",
			Industry: "Technology",
			Region:   "US",
		},
		Branding: worldmodels.Branding{
			Tone:   "formal",
			Colors: []string{},
		},
	}
	entry := ManifestEntry{
		Title:        "About Page",
		Category:     "public-about-page",
		Path:         "public/about.html",
		RenderedType: "html",
		PromptHint:   "Generate an about page",
	}

	prompt := buildPrompt(model, entry)

	if strings.Contains(prompt, "Brand colors:") {
		t.Error("prompt should not contain 'Brand colors:' when colors are empty")
	}
}
