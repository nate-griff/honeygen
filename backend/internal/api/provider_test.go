package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/config"
	"github.com/natet/honeygen/backend/internal/models"
	"github.com/natet/honeygen/backend/internal/provider"
)

func TestProviderTestEndpointReportsReadyProvider(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/v1/models")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	cfg := testProviderConfig(t, server.URL+"/v1")
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Ready   bool   `json:"ready"`
		Mode    string `json:"mode"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !response.Ready || response.Mode != "external" || response.BaseURL != cfg.Provider.BaseURL || response.Model != cfg.Provider.Model {
		t.Fatalf("response = %+v, want ready external with provider config", response)
	}
}

func TestProviderTestEndpointReportsConfigurationFailures(t *testing.T) {
	t.Parallel()

	cfg := testProviderConfig(t, "")
	cfg.Provider.APIKey = ""

	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_invalid",
			Message: "provider base URL is required",
		},
	})
}

func TestProviderTestEndpointReportsUpstreamAuthFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := testProviderConfig(t, server.URL)
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadGateway, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_auth_failed",
			Message: "provider authentication failed",
		},
	})
}

func TestProviderTestEndpointPersistsFailureMessageWhenGenerationJobIDProvided(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := testProviderConfig(t, server.URL)
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())
	seedGenerationJob(t, application, "world-1", "job-1")

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", strings.NewReader(`{"generation_job_id":"job-1"}`))
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadGateway, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_auth_failed",
			Message: "provider authentication failed",
		},
	})

	assertGenerationJobErrorMessage(t, application, "job-1", "provider authentication failed")
}

func TestProviderTestEndpointSkipsFailurePersistenceWithoutGenerationJobID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := testProviderConfig(t, server.URL)
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())
	seedGenerationJob(t, application, "world-1", "job-1")

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadGateway, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_auth_failed",
			Message: "provider authentication failed",
		},
	})

	assertGenerationJobErrorMessage(t, application, "job-1", "")
}

func TestProviderTestEndpointReportsConnectivityFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := server.URL
	server.Close()

	cfg := testProviderConfig(t, baseURL)
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadGateway, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_unreachable",
			Message: "provider request failed",
		},
	})
}

func TestProviderTestEndpointReportsInvalidResponses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list"}`))
	}))
	defer server.Close()

	cfg := testProviderConfig(t, server.URL)
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadGateway, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_invalid_response",
			Message: "provider returned an invalid response",
		},
	})
}

func TestProviderTestEndpointReportsUpstreamFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	cfg := testProviderConfig(t, server.URL)
	application := newTestAPIAppWithConfig(t, cfg)
	application.Provider = provider.NewOpenAI(cfg.Provider, server.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/provider/test", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadGateway, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "provider_unavailable",
			Message: "provider returned status 502",
		},
	})
}

func newTestAPIAppWithConfig(t *testing.T, cfg config.Config) *app.APIApp {
	t.Helper()

	application, err := app.NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp() error = %v", err)
	}

	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	return application
}

func testProviderConfig(t *testing.T, baseURL string) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		ServiceName:        "honeygen-api",
		ServiceVersion:     "test",
		AppEnv:             "test",
		HTTPAddr:           ":0",
		SQLitePath:         filepath.Join(root, "api.db"),
		GeneratedAssetsDir: filepath.Join(root, "generated"),
		StorageRoot:        filepath.Join(root, "storage"),
		Provider: config.ProviderConfig{
			BaseURL: baseURL,
			APIKey:  "test-key",
			Model:   "gpt-4.1-mini",
		},
	}
}

func seedGenerationJob(t *testing.T, application *app.APIApp, worldModelID, jobID string) {
	t.Helper()

	if _, err := application.DB.ExecContext(context.Background(), `
		INSERT INTO world_models (id, name, description, json_blob, created_at, updated_at)
		VALUES (?, 'World 1', '', '{}', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`, worldModelID); err != nil {
		t.Fatalf("seed world model error = %v", err)
	}

	if _, err := application.DB.ExecContext(context.Background(), `
		INSERT INTO generation_jobs (id, world_model_id, status, error_message, created_at, updated_at)
		VALUES (?, ?, 'failed', '', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')
	`, jobID, worldModelID); err != nil {
		t.Fatalf("seed generation job error = %v", err)
	}
}

func assertGenerationJobErrorMessage(t *testing.T, application *app.APIApp, jobID, want string) {
	t.Helper()

	var message string
	if err := application.DB.QueryRowContext(context.Background(), `SELECT error_message FROM generation_jobs WHERE id = ?`, jobID).Scan(&message); err != nil {
		t.Fatalf("query error_message error = %v", err)
	}
	if message != want {
		t.Fatalf("error_message = %q, want %q", message, want)
	}
}
