package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/config"
	"github.com/natet/honeygen/backend/internal/generation"
	"github.com/natet/honeygen/backend/internal/models"
	"github.com/natet/honeygen/backend/internal/storage"
)

func TestAssetContentEndpointDoesNotInlineDOCXPreviewEvenIfMarkedPreviewable(t *testing.T) {
	application := newAssetPreviewTestAPIApp(t)
	router := NewRouter(application)
	seedAssetPreviewGenerationJob(t, application, "world-1", "job-1")

	storedFile, err := application.Storage.Write(context.Background(), "generated/world-1/job-1/intranet/policies/acceptable-use-policy.docx", []byte("<?xml version=\"1.0\"?><w:document>raw xml</w:document>"))
	if err != nil {
		t.Fatalf("Storage.Write() error = %v", err)
	}

	if _, err := application.AssetRepo.Create(context.Background(), assets.Asset{
		ID:              "asset-docx-1",
		GenerationJobID: "job-1",
		WorldModelID:    "world-1",
		SourceType:      "generated",
		RenderedType:    "docx",
		Path:            storedFile.Path,
		MIMEType:        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		SizeBytes:       storedFile.SizeBytes,
		Previewable:     true,
		Checksum:        storedFile.Checksum,
	}); err != nil {
		t.Fatalf("AssetRepo.Create() error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodGet, "/api/assets/asset-docx-1/content", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Previewable bool   `json:"previewable"`
		Content     string `json:"content"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Previewable {
		t.Fatalf("response = %+v, want non-previewable DOCX response", response)
	}
	if response.Content != "" {
		t.Fatalf("response.Content = %q, want empty content for DOCX preview", response.Content)
	}
	if response.Message == "" {
		t.Fatalf("response = %+v, want binary preview message", response)
	}
}

func newAssetPreviewTestAPIApp(t *testing.T) *app.APIApp {
	t.Helper()

	cfg := config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    ":0",
		AdminPassword:               "test-admin-password",
		InternalEventIngestToken:    "test-internal-event-token",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		SQLitePath:                  filepath.Join(t.TempDir(), "api.db"),
		GeneratedAssetsDir:          filepath.Join(t.TempDir(), "generated"),
		StorageRoot:                 filepath.Join(t.TempDir(), "storage"),
	}

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

func seedAssetPreviewGenerationJob(t *testing.T, application *app.APIApp, worldModelID, jobID string) {
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

// ─── Upload endpoint tests ────────────────────────────────────────────────────

func TestAssetUploadEndpointRequiresAuth(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req, _ := buildUploadMultipartRequest(t, "northbridge-financial-job", "public/file.txt", "file.txt", []byte("hello"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusUnauthorized, "", models.APIErrorResponse{
		Error: models.APIError{Code: "unauthorized", Message: "authentication required"},
	})
}

func TestAssetDeleteEndpointRequiresAuth(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/asset-1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusUnauthorized, "", models.APIErrorResponse{
		Error: models.APIError{Code: "unauthorized", Message: "authentication required"},
	})
}

func TestAssetDeleteEndpointRemovesAssetFromCompletedJob(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	seedCompletedGenerationJob(t, application, "northbridge-financial", "delete-job-1")
	created := seedStoredAsset(t, application, assets.Asset{
		ID:              "asset-delete-1",
		GenerationJobID: "delete-job-1",
		WorldModelID:    "northbridge-financial",
		SourceType:      "generated",
		RenderedType:    "text",
		Path:            "generated/northbridge-financial/delete-job-1/public/delete-me.txt",
		MIMEType:        "text/plain",
		Previewable:     true,
	}, []byte("delete me"))

	req := authenticatedRequest(t, router, http.MethodDelete, "/api/assets/"+created.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("body = %q, want empty body", body)
	}

	if _, err := application.AssetRepo.Get(context.Background(), created.ID); !errors.Is(err, assets.ErrNotFound) {
		t.Fatalf("AssetRepo.Get() after delete error = %v, want %v", err, assets.ErrNotFound)
	}

	fullPath := filepath.Join(application.Config.StorageRoot, filepath.FromSlash(created.Path))
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("stored file still exists after delete: %v", err)
	}
}

func TestAssetDeleteEndpointReturnsNotFoundWhenAssetDoesNotExist(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodDelete, "/api/assets/missing-asset", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusNotFound, "", models.APIErrorResponse{
		Error: models.APIError{Code: "not_found", Message: "asset not found"},
	})
}

func TestAssetDeleteEndpointReturnsConflictForNonCompletedJob(t *testing.T) {
	statuses := []string{"pending", "running", "failed", "canceled"}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			application := newTestAPIApp(t)
			router := NewRouter(application)
			seedJobWithStatus(t, application, "northbridge-financial", "delete-job-status", status)
			created := seedStoredAsset(t, application, assets.Asset{
				ID:              "asset-delete-status",
				GenerationJobID: "delete-job-status",
				WorldModelID:    "northbridge-financial",
				SourceType:      "generated",
				RenderedType:    "text",
				Path:            "generated/northbridge-financial/delete-job-status/public/file.txt",
				MIMEType:        "text/plain",
				Previewable:     true,
			}, []byte("delete me"))

			req := authenticatedRequest(t, router, http.MethodDelete, "/api/assets/"+created.ID, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertAPIErrorResponse(t, rec, http.StatusConflict, "", models.APIErrorResponse{
				Error: models.APIError{Code: "asset_not_deletable", Message: "assets can only be deleted from completed generation jobs"},
			})
		})
	}
}

func TestAssetDeleteEndpointReturnsInternalServerErrorWhenDeleteFails(t *testing.T) {
	application := newTestAPIApp(t)
	originalService := application.Generation
	application.Generation = generation.NewService(generation.ServiceConfig{
		WorldModels: application.WorldModels,
		Planner:     application.Planner,
		Provider:    application.CurrentProvider(),
		Jobs:        application.JobStore,
		Assets:      application.AssetRepo,
		Storage: failingDeleteAssetStorage{
			Filesystem: application.Storage,
			err:        errors.New("delete file failed"),
		},
		Renderers: application.Renderers,
	})
	t.Cleanup(func() {
		_ = originalService.Close()
	})
	router := NewRouter(application)
	seedCompletedGenerationJob(t, application, "northbridge-financial", "delete-job-fail")
	created := seedStoredAsset(t, application, assets.Asset{
		ID:              "asset-delete-fail",
		GenerationJobID: "delete-job-fail",
		WorldModelID:    "northbridge-financial",
		SourceType:      "generated",
		RenderedType:    "text",
		Path:            "generated/northbridge-financial/delete-job-fail/public/file.txt",
		MIMEType:        "text/plain",
		Previewable:     true,
	}, []byte("delete me"))

	req := authenticatedRequest(t, router, http.MethodDelete, "/api/assets/"+created.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusInternalServerError, "", models.APIErrorResponse{
		Error: models.APIError{Code: "asset_delete_failed", Message: "asset could not be deleted"},
	})

	if _, err := application.AssetRepo.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("AssetRepo.Get() after failed delete error = %v, want asset to remain", err)
	}
}

func TestAssetEndpointRejectsUnsupportedMethod(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodPost, "/api/assets/asset-1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusMethodNotAllowed, "GET, DELETE", models.APIErrorResponse{
		Error: models.APIError{Code: "method_not_allowed", Message: "method not allowed"},
	})
}

func TestAssetUploadEndpointCreatesAssetInCompletedJob(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	seedCompletedGenerationJob(t, application, "northbridge-financial", "upload-job-1")

	req, contentType := buildUploadMultipartRequest(t, "upload-job-1", "public/hello.txt", "hello.txt", []byte("hello world"))
	req.Header.Set("Content-Type", contentType)
	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", req.Body)
	authReq.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var created assets.Asset
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, body=%s", err, rec.Body.String())
	}
	if created.ID == "" {
		t.Fatalf("created.ID is empty")
	}
	if created.GenerationJobID != "upload-job-1" {
		t.Fatalf("GenerationJobID = %q, want %q", created.GenerationJobID, "upload-job-1")
	}
	if created.WorldModelID != "northbridge-financial" {
		t.Fatalf("WorldModelID = %q, want %q", created.WorldModelID, "northbridge-financial")
	}
	if created.SourceType != "upload" {
		t.Fatalf("SourceType = %q, want %q", created.SourceType, "upload")
	}
	if created.SizeBytes != int64(len("hello world")) {
		t.Fatalf("SizeBytes = %d, want %d", created.SizeBytes, len("hello world"))
	}
	if created.Path == "" {
		t.Fatalf("Path is empty")
	}
}

func TestAssetUploadEndpointRejectsNonCompletedJob(t *testing.T) {
	statuses := []string{"pending", "running", "failed", "canceled"}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			application := newTestAPIApp(t)
			router := NewRouter(application)
			seedJobWithStatus(t, application, "northbridge-financial", "upload-job-status", status)

			req, contentType := buildUploadMultipartRequest(t, "upload-job-status", "public/file.txt", "file.txt", []byte("data"))
			authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", req.Body)
			authReq.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authReq)

			assertAPIErrorResponse(t, rec, http.StatusConflict, "", models.APIErrorResponse{
				Error: models.APIError{Code: "job_not_completed", Message: "uploads are only allowed for completed generation jobs"},
			})
		})
	}
}

func TestAssetUploadEndpointRejectsNonExistentJob(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req, contentType := buildUploadMultipartRequest(t, "job_nonexistent", "public/file.txt", "file.txt", []byte("data"))
	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", req.Body)
	authReq.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusNotFound, "", models.APIErrorResponse{
		Error: models.APIError{Code: "not_found", Message: "generation job not found"},
	})
}

func TestAssetUploadEndpointRejectsPathConflict(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	seedCompletedGenerationJob(t, application, "northbridge-financial", "upload-job-conflict")

	// Seed an existing asset at the target path
	existingPath := "generated/northbridge-financial/upload-job-conflict/public/conflict.txt"
	if _, err := application.AssetRepo.Create(context.Background(), assets.Asset{
		ID:              "existing-asset-conflict",
		GenerationJobID: "upload-job-conflict",
		WorldModelID:    "northbridge-financial",
		SourceType:      "generated",
		RenderedType:    "text",
		Path:            existingPath,
		MIMEType:        "text/plain",
		SizeBytes:       5,
		Checksum:        "abc123",
	}); err != nil {
		t.Fatalf("AssetRepo.Create() error = %v", err)
	}

	req, contentType := buildUploadMultipartRequest(t, "upload-job-conflict", "public/conflict.txt", "conflict.txt", []byte("new content"))
	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", req.Body)
	authReq.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusConflict, "", models.APIErrorResponse{
		Error: models.APIError{Code: "path_conflict", Message: "an asset already exists at the target path"},
	})
}

func TestAssetUploadEndpointEnforcesMaxUploadSize(t *testing.T) {
	cfg := baseTestConfig(t)
	cfg.MaxUploadSizeBytes = 100 // 100 bytes — very small for testing
	application := newTestAPIAppWithConfig(t, cfg)
	router := NewRouter(application)
	seedCompletedGenerationJob(t, application, "northbridge-financial", "upload-job-size")

	oversizedContent := bytes.Repeat([]byte("x"), 200)
	req, contentType := buildUploadMultipartRequest(t, "upload-job-size", "public/big.txt", "big.txt", oversizedContent)
	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", req.Body)
	authReq.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusRequestEntityTooLarge, "", models.APIErrorResponse{
		Error: models.APIError{Code: "upload_too_large", Message: "file exceeds the maximum upload size"},
	})
}

func TestAssetUploadEndpointCleansUpAssetRecordWhenStorageWriteFails(t *testing.T) {
	application := newTestAPIApp(t)
	application.Storage = storage.NewFilesystem("")
	router := NewRouter(application)
	seedCompletedGenerationJob(t, application, "northbridge-financial", "upload-job-write-fail")

	req, contentType := buildUploadMultipartRequest(t, "upload-job-write-fail", "public/file.txt", "file.txt", []byte("data"))
	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", req.Body)
	authReq.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusInternalServerError, "", models.APIErrorResponse{
		Error: models.APIError{Code: "upload_failed", Message: "failed to store uploaded file"},
	})

	items, err := application.AssetRepo.List(context.Background(), assets.ListOptions{GenerationJobID: "upload-job-write-fail", Limit: 10})
	if err != nil {
		t.Fatalf("AssetRepo.List() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("AssetRepo.List() returned %d items, want 0 after failed upload", len(items))
	}
}

func TestStoreUploadedAssetContentUsesFreshContextForRollback(t *testing.T) {
	expiredCtx, cancel := context.WithCancel(context.Background())
	cancel()

	deleteCalled := false
	err := storeUploadedAssetContent(
		expiredCtx,
		"generated/world/job/public/file.txt",
		[]byte("data"),
		"asset-timeout",
		func(ctx context.Context, relativePath string, data []byte) (storage.StoredFile, error) {
			return storage.StoredFile{}, ctx.Err()
		},
		func(ctx context.Context, assetID string) error {
			deleteCalled = true
			if assetID != "asset-timeout" {
				t.Fatalf("assetID = %q, want %q", assetID, "asset-timeout")
			}
			if err := ctx.Err(); err != nil {
				t.Fatalf("rollback ctx err = %v, want live context", err)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("rollback ctx missing deadline")
			}
			return nil
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("storeUploadedAssetContent() error = %v, want %v", err, context.Canceled)
	}
	if !deleteCalled {
		t.Fatal("delete callback was not called")
	}
}

func TestAssetUploadEndpointRequiresGenerationJobID(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("target_path", "public/file.txt")
	fw, _ := mw.CreateFormFile("file", "file.txt")
	_, _ = fw.Write([]byte("content"))
	mw.Close()

	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", &buf)
	authReq.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{Code: "validation_error", Message: "generation_job_id is required"},
	})
}

func TestAssetUploadEndpointRequiresTargetPath(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("generation_job_id", "some-job")
	fw, _ := mw.CreateFormFile("file", "file.txt")
	_, _ = fw.Write([]byte("content"))
	mw.Close()

	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", &buf)
	authReq.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{Code: "validation_error", Message: "target_path is required"},
	})
}

func TestAssetUploadEndpointRequiresFile(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("generation_job_id", "some-job")
	_ = mw.WriteField("target_path", "public/file.txt")
	mw.Close()

	authReq := authenticatedRequest(t, router, http.MethodPost, "/api/assets/upload", &buf)
	authReq.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{Code: "validation_error", Message: "file is required"},
	})
}

func TestAssetUploadEndpointRejectsWrongMethod(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	authReq := authenticatedRequest(t, router, http.MethodGet, "/api/assets/upload", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authReq)

	assertAPIErrorResponse(t, rec, http.StatusMethodNotAllowed, http.MethodPost, models.APIErrorResponse{
		Error: models.APIError{Code: "method_not_allowed", Message: "method not allowed"},
	})
}

// ─── Upload test helpers ──────────────────────────────────────────────────────

func buildUploadMultipartRequest(t *testing.T, jobID, targetPath, filename string, content []byte) (*http.Request, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if jobID != "" {
		_ = mw.WriteField("generation_job_id", jobID)
	}
	if targetPath != "" {
		_ = mw.WriteField("target_path", targetPath)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	return req, mw.FormDataContentType()
}

func seedCompletedGenerationJob(t *testing.T, application *app.APIApp, worldModelID, jobID string) {
	t.Helper()
	if _, err := application.DB.ExecContext(context.Background(), `
		INSERT OR IGNORE INTO generation_jobs (id, world_model_id, status, summary_json, error_message, created_at, updated_at, completed_at)
		VALUES (?, ?, 'completed', '{}', '', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	`, jobID, worldModelID); err != nil {
		t.Fatalf("seed completed generation job error = %v", err)
	}
}

func seedJobWithStatus(t *testing.T, application *app.APIApp, worldModelID, jobID, status string) {
	t.Helper()
	if _, err := application.DB.ExecContext(context.Background(), `
		INSERT OR IGNORE INTO generation_jobs (id, world_model_id, status, summary_json, error_message, created_at, updated_at)
		VALUES (?, ?, ?, '{}', '', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	`, jobID, worldModelID, status); err != nil {
		t.Fatalf("seed generation job with status %q error = %v", status, err)
	}
}

func seedStoredAsset(t *testing.T, application *app.APIApp, asset assets.Asset, content []byte) assets.Asset {
	t.Helper()

	storedFile, err := application.Storage.Write(context.Background(), asset.Path, content)
	if err != nil {
		t.Fatalf("Storage.Write() error = %v", err)
	}

	asset.SizeBytes = storedFile.SizeBytes
	asset.Checksum = storedFile.Checksum
	if asset.Tags == nil {
		asset.Tags = []string{}
	}

	created, err := application.AssetRepo.Create(context.Background(), asset)
	if err != nil {
		t.Fatalf("AssetRepo.Create() error = %v", err)
	}

	return created
}

type failingDeleteAssetStorage struct {
	*storage.Filesystem
	err error
}

func (s failingDeleteAssetStorage) Delete(context.Context, string) error {
	return s.err
}

func baseTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    ":0",
		AdminPassword:               "test-admin-password",
		InternalEventIngestToken:    "test-internal-event-token",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		SQLitePath:                  filepath.Join(t.TempDir(), "api.db"),
		GeneratedAssetsDir:          filepath.Join(t.TempDir(), "generated"),
		StorageRoot:                 filepath.Join(t.TempDir(), "storage"),
	}
}

func newUploadTestAPIApp(t *testing.T) (*app.APIApp, http.Handler) {
	t.Helper()
	application := newTestAPIApp(t)
	return application, NewRouter(application)
}
