package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/generation"
	"github.com/natet/honeygen/backend/internal/models"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/rendering"
)

func TestGenerationAndAssetsEndpointsRunBrowseAndPreview(t *testing.T) {
	application := newTestAPIAppWithConfig(t, testProviderConfig(t, "https://provider.example/v1"))
	application.Provider = generationStubProvider{}
	application.Renderers = rendering.NewRegistry(rendering.RegistryConfig{
		PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
	})

	runReq := httptest.NewRequest(http.MethodPost, "/api/generation/run", strings.NewReader(`{"world_model_id":"northbridge-financial"}`))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(runRec, runReq)

	if runRec.Code != http.StatusCreated {
		t.Fatalf("run status code = %d, want %d, body=%s", runRec.Code, http.StatusCreated, runRec.Body.String())
	}
	assertJobLogsAppearOnlyInSummary(t, runRec.Body.Bytes())

	var job struct {
		ID           string `json:"id"`
		WorldModelID string `json:"world_model_id"`
		Status       string `json:"status"`
		Summary      struct {
			AssetCount    int `json:"asset_count"`
			ManifestCount int `json:"manifest_count"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(runRec.Body.Bytes(), &job); err != nil {
		t.Fatalf("json.Unmarshal(run) error = %v", err)
	}
	if job.ID == "" || job.WorldModelID != "northbridge-financial" || job.Status != generation.StatusRunning {
		t.Fatalf("job = %+v, want running job for seeded world model", job)
	}
	if job.Summary.ManifestCount == 0 {
		t.Fatalf("job summary = %+v, want planned manifest count", job.Summary)
	}

	jobsReq := httptest.NewRequest(http.MethodGet, "/api/generation/jobs?world_model_id=northbridge-financial&limit=5", nil)
	jobsRec := httptest.NewRecorder()
	NewRouter(application).ServeHTTP(jobsRec, jobsReq)

	if jobsRec.Code != http.StatusOK {
		t.Fatalf("jobs status code = %d, want %d, body=%s", jobsRec.Code, http.StatusOK, jobsRec.Body.String())
	}

	var jobsResponse struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(jobsRec.Body.Bytes(), &jobsResponse); err != nil {
		t.Fatalf("json.Unmarshal(jobs) error = %v", err)
	}
	if len(jobsResponse.Items) == 0 || jobsResponse.Items[0].ID != job.ID {
		t.Fatalf("jobs response = %+v, want generated job", jobsResponse)
	}

	waitForJobStatus(t, application.JobStore, job.ID, generation.StatusCompleted)

	jobReq := httptest.NewRequest(http.MethodGet, "/api/generation/jobs/"+job.ID, nil)
	jobRec := httptest.NewRecorder()
	NewRouter(application).ServeHTTP(jobRec, jobReq)
	if jobRec.Code != http.StatusOK {
		t.Fatalf("job detail status code = %d, want %d, body=%s", jobRec.Code, http.StatusOK, jobRec.Body.String())
	}
	assertJobLogsAppearOnlyInSummary(t, jobRec.Body.Bytes())

	assetsReq := httptest.NewRequest(http.MethodGet, "/api/assets?generation_job_id="+job.ID+"&limit=200", nil)
	assetsRec := httptest.NewRecorder()
	NewRouter(application).ServeHTTP(assetsRec, assetsReq)
	if assetsRec.Code != http.StatusOK {
		t.Fatalf("assets status code = %d, want %d, body=%s", assetsRec.Code, http.StatusOK, assetsRec.Body.String())
	}

	var assetsResponse struct {
		Items []struct {
			ID           string `json:"id"`
			Path         string `json:"path"`
			RenderedType string `json:"rendered_type"`
			Previewable  bool   `json:"previewable"`
		} `json:"items"`
	}
	if err := json.Unmarshal(assetsRec.Body.Bytes(), &assetsResponse); err != nil {
		t.Fatalf("json.Unmarshal(assets) error = %v", err)
	}
	if len(assetsResponse.Items) == 0 {
		t.Fatal("assets response is empty")
	}

	var previewAssetID, pdfAssetID string
	expectedPrefix := "generated/northbridge-financial/" + job.ID + "/"
	for _, item := range assetsResponse.Items {
		if !strings.HasPrefix(item.Path, expectedPrefix) {
			t.Fatalf("asset path = %q, want prefix %q", item.Path, expectedPrefix)
		}
		if item.Previewable && previewAssetID == "" {
			previewAssetID = item.ID
		}
		if item.RenderedType == "pdf" {
			pdfAssetID = item.ID
		}
	}
	if previewAssetID == "" || pdfAssetID == "" {
		t.Fatalf("asset coverage missing previewAssetID=%q pdfAssetID=%q", previewAssetID, pdfAssetID)
	}

	treeReq := httptest.NewRequest(http.MethodGet, "/api/assets/tree?generation_job_id="+job.ID, nil)
	treeRec := httptest.NewRecorder()
	NewRouter(application).ServeHTTP(treeRec, treeReq)
	if treeRec.Code != http.StatusOK {
		t.Fatalf("tree status code = %d, want %d, body=%s", treeRec.Code, http.StatusOK, treeRec.Body.String())
	}

	var treeResponse struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(treeRec.Body.Bytes(), &treeResponse); err != nil {
		t.Fatalf("json.Unmarshal(tree) error = %v", err)
	}
	rootNames := make([]string, 0, len(treeResponse.Items))
	for _, item := range treeResponse.Items {
		rootNames = append(rootNames, item.Name)
	}
	for _, want := range []string{"public", "intranet", "shared", "users"} {
		if !containsString(rootNames, want) {
			t.Fatalf("tree roots = %+v, want %q", rootNames, want)
		}
	}

	previewReq := httptest.NewRequest(http.MethodGet, "/api/assets/"+previewAssetID+"/content", nil)
	previewRec := httptest.NewRecorder()
	NewRouter(application).ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status code = %d, want %d, body=%s", previewRec.Code, http.StatusOK, previewRec.Body.String())
	}

	var previewResponse struct {
		Previewable bool   `json:"previewable"`
		Content     string `json:"content"`
	}
	if err := json.Unmarshal(previewRec.Body.Bytes(), &previewResponse); err != nil {
		t.Fatalf("json.Unmarshal(preview) error = %v", err)
	}
	if !previewResponse.Previewable || strings.TrimSpace(previewResponse.Content) == "" {
		t.Fatalf("preview response = %+v, want inline content", previewResponse)
	}

	pdfReq := httptest.NewRequest(http.MethodGet, "/api/assets/"+pdfAssetID+"/content", nil)
	pdfRec := httptest.NewRecorder()
	NewRouter(application).ServeHTTP(pdfRec, pdfReq)
	if pdfRec.Code != http.StatusOK {
		t.Fatalf("pdf content status code = %d, want %d, body=%s", pdfRec.Code, http.StatusOK, pdfRec.Body.String())
	}

	var pdfResponse struct {
		Previewable bool   `json:"previewable"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal(pdfRec.Body.Bytes(), &pdfResponse); err != nil {
		t.Fatalf("json.Unmarshal(pdf preview) error = %v", err)
	}
	if pdfResponse.Previewable || pdfResponse.Message == "" {
		t.Fatalf("pdf response = %+v, want metadata-only binary response", pdfResponse)
	}
}

func TestGenerationRunReturnsImmediatelyWhileJobRunsInBackground(t *testing.T) {
	application := newTestAPIAppWithConfig(t, testProviderConfig(t, "https://provider.example/v1"))
	provider := blockingGenerationProvider{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	application.Provider = provider
	application.Renderers = rendering.NewRegistry(rendering.RegistryConfig{
		PDF: rendering.StaticPDFRenderer([]byte("%PDF-1.4\n%stub\n")),
	})

	runReq := httptest.NewRequest(http.MethodPost, "/api/generation/run", strings.NewReader(`{"world_model_id":"northbridge-financial"}`))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(runRec, runReq)

	if runRec.Code != http.StatusCreated {
		t.Fatalf("run status code = %d, want %d, body=%s", runRec.Code, http.StatusCreated, runRec.Body.String())
	}

	var job struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(runRec.Body.Bytes(), &job); err != nil {
		t.Fatalf("json.Unmarshal(run) error = %v", err)
	}
	if job.ID == "" || job.Status != generation.StatusRunning {
		t.Fatalf("job = %+v, want running job", job)
	}

	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background generation did not start")
	}

	runningJob, err := application.JobStore.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() job error = %v", err)
	}
	if runningJob.Status != generation.StatusRunning {
		t.Fatalf("runningJob.Status = %q, want %q", runningJob.Status, generation.StatusRunning)
	}

	close(provider.release)
	waitForJobStatus(t, application.JobStore, job.ID, generation.StatusCompleted)
}

func TestGenerationRunReturnsValidationErrorWithoutWorldModelID(t *testing.T) {
	application := newTestAPIAppWithConfig(t, testProviderConfig(t, "https://provider.example/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/generation/run", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "validation_error",
			Message: "world_model_id is required",
		},
	})
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

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func waitForJobStatus(t *testing.T, store *generation.JobStore, jobID, wantStatus string) generation.Job {
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
	return generation.Job{}
}

func assertJobLogsAppearOnlyInSummary(t *testing.T, body []byte) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(job payload) error = %v", err)
	}
	if _, ok := payload["logs"]; ok {
		t.Fatalf("job payload includes top-level logs: %s", string(body))
	}

	summary, ok := payload["summary"].(map[string]any)
	if !ok {
		t.Fatalf("job payload summary missing or wrong type: %s", string(body))
	}
	logs, ok := summary["logs"].([]any)
	if !ok || len(logs) == 0 {
		t.Fatalf("job payload summary.logs missing or empty: %s", string(body))
	}
}
