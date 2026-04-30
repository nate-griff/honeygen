package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/generation"
	"github.com/natet/honeygen/backend/internal/storage"
)

func assetsListHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		items, err := application.AssetRepo.List(ctx, readAssetListOptions(r))
		if err != nil {
			application.Logger.Error("list assets", "error", err)
			writeError(w, http.StatusInternalServerError, "assets_unavailable", "assets are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func assetsTreeHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		items, err := application.AssetRepo.Tree(ctx, readAssetListOptions(r))
		if err != nil {
			application.Logger.Error("build asset tree", "error", err)
			writeError(w, http.StatusInternalServerError, "assets_unavailable", "assets are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func assetsItemHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(r.URL.Path, "/content") {
				handleAssetContent(application, w, r)
				return
			}
			handleAssetGet(application, w, r)
		case http.MethodDelete:
			handleAssetDelete(application, w, r)
		default:
			w.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodDelete}, ", "))
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func handleAssetGet(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := assetIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.AssetRepo.Get(ctx, id)
	if errors.Is(err, assets.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	if err != nil {
		application.Logger.Error("get asset", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "assets_unavailable", "assets are temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func handleAssetContent(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := assetContentIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.AssetRepo.Get(ctx, id)
	if errors.Is(err, assets.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	if err != nil {
		application.Logger.Error("get asset content", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "assets_unavailable", "assets are temporarily unavailable")
		return
	}

	if !assetSupportsInlinePreview(item) {
		writeJSON(w, http.StatusOK, map[string]any{
			"asset":       item,
			"previewable": false,
			"message":     "asset content is not previewable inline",
		})
		return
	}

	content, err := application.Storage.Read(ctx, item.Path)
	if err != nil {
		application.Logger.Error("read asset content", "error", err, "id", id, "path", item.Path)
		writeError(w, http.StatusInternalServerError, "asset_content_unavailable", "asset content is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"asset":       item,
		"previewable": true,
		"content":     string(content),
	})
}

func handleAssetDelete(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := assetIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := application.GenerationService().DeleteAsset(ctx, id); err != nil {
		if errors.Is(err, assets.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "asset not found")
			return
		}
		if errors.Is(err, generation.ErrAssetNotDeletable) {
			writeError(w, http.StatusConflict, "asset_not_deletable", "assets can only be deleted from completed generation jobs")
			return
		}
		application.Logger.Error("delete asset", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "asset_delete_failed", "asset could not be deleted")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func assetSupportsInlinePreview(item assets.Asset) bool {
	if !item.Previewable {
		return false
	}

	switch item.RenderedType {
	case "html", "markdown", "text", "csv":
		return true
	}

	return strings.HasPrefix(strings.ToLower(item.MIMEType), "text/")
}

func readAssetListOptions(r *http.Request) assets.ListOptions {
	return assets.ListOptions{
		WorldModelID:    strings.TrimSpace(r.URL.Query().Get("world_model_id")),
		GenerationJobID: strings.TrimSpace(r.URL.Query().Get("generation_job_id")),
		Limit:           parseOptionalInt(r.URL.Query().Get("limit"), 100),
		Offset:          parseOptionalInt(r.URL.Query().Get("offset"), 0),
	}
}

func assetIDFromPath(path string) (string, error) {
	const prefix = "/api/assets/"
	if !strings.HasPrefix(path, prefix) {
		return "", requestValidationError{message: "asset id is required"}
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") {
		return "", requestValidationError{message: "asset id is required"}
	}
	return id, nil
}

func assetContentIDFromPath(path string) (string, error) {
	const prefix = "/api/assets/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/content") {
		return "", requestValidationError{message: "asset id is required"}
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/content")
	id = strings.TrimSpace(strings.TrimSuffix(id, "/"))
	if id == "" || strings.Contains(id, "/") {
		return "", requestValidationError{message: "asset id is required"}
	}
	return id, nil
}

func assetsUploadHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxBytes := application.Config.EffectiveMaxUploadSizeBytes()
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

		if err := r.ParseMultipartForm(maxBytes); err != nil {
			if strings.Contains(err.Error(), "http: request body too large") {
				writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "file exceeds the maximum upload size")
				return
			}
			writeValidationFailure(w, requestValidationError{message: "request must be multipart/form-data"})
			return
		}

		jobID := strings.TrimSpace(r.FormValue("generation_job_id"))
		if jobID == "" {
			writeValidationFailure(w, requestValidationError{message: "generation_job_id is required"})
			return
		}

		targetPath := strings.TrimSpace(r.FormValue("target_path"))
		if targetPath == "" {
			writeValidationFailure(w, requestValidationError{message: "target_path is required"})
			return
		}

		file, fh, err := r.FormFile("file")
		if err != nil {
			writeValidationFailure(w, requestValidationError{message: "file is required"})
			return
		}
		defer file.Close()

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		job, err := application.JobStore.Get(ctx, jobID)
		if errors.Is(err, generation.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "generation job not found")
			return
		}
		if err != nil {
			application.Logger.Error("upload: get job", "error", err, "job_id", jobID)
			writeError(w, http.StatusInternalServerError, "assets_unavailable", "assets are temporarily unavailable")
			return
		}
		if job.Status != generation.StatusCompleted {
			writeError(w, http.StatusConflict, "job_not_completed", "uploads are only allowed for completed generation jobs")
			return
		}

		storagePath, err := storage.JoinRelative("generated", job.WorldModelID, job.ID, targetPath)
		if err != nil {
			writeValidationFailure(w, requestValidationError{message: "target_path is invalid"})
			return
		}

		_, findErr := application.AssetRepo.FindByPath(ctx, storagePath)
		if findErr == nil {
			writeError(w, http.StatusConflict, "path_conflict", "an asset already exists at the target path")
			return
		}
		if !errors.Is(findErr, assets.ErrNotFound) {
			application.Logger.Error("upload: check path conflict", "error", findErr, "path", storagePath)
			writeError(w, http.StatusInternalServerError, "assets_unavailable", "assets are temporarily unavailable")
			return
		}

		data, err := io.ReadAll(file)
		if err != nil {
			application.Logger.Error("upload: read file", "error", err)
			writeError(w, http.StatusInternalServerError, "upload_failed", "failed to read uploaded file")
			return
		}

		mimeType := uploadMIMEType(fh, data)
		renderedType := uploadRenderedType(fh.Filename)
		checksum := uploadChecksum(data)

		created, err := application.AssetRepo.Create(ctx, assets.Asset{
			ID:              newUploadAssetID(),
			GenerationJobID: job.ID,
			WorldModelID:    job.WorldModelID,
			SourceType:      "upload",
			RenderedType:    renderedType,
			Path:            storagePath,
			MIMEType:        mimeType,
			SizeBytes:       int64(len(data)),
			Tags:            []string{"source:upload"},
			Previewable:     isUploadPreviewable(renderedType, mimeType),
			Checksum:        checksum,
		})
		if err != nil {
			if errors.Is(err, assets.ErrPathConflict) {
				writeError(w, http.StatusConflict, "path_conflict", "an asset already exists at the target path")
				return
			}
			application.Logger.Error("upload: create asset record", "error", err, "path", storagePath)
			writeError(w, http.StatusInternalServerError, "upload_failed", "failed to record uploaded asset")
			return
		}

		if err := storeUploadedAssetContent(ctx, storagePath, data, created.ID, application.Storage.Write, application.AssetRepo.Delete, application.Logger); err != nil {
			application.Logger.Error("upload: write file", "error", err, "path", storagePath)
			writeError(w, http.StatusInternalServerError, "upload_failed", "failed to store uploaded file")
			return
		}

		writeJSON(w, http.StatusCreated, created)
	}
}

func uploadMIMEType(fh *multipart.FileHeader, data []byte) string {
	if ct := fh.Header.Get("Content-Type"); ct != "" && ct != "application/octet-stream" {
		return ct
	}
	ext := uploadExtension(fh.Filename)
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
}

func uploadRenderedType(filename string) string {
	switch uploadExtension(filename) {
	case ".html", ".htm":
		return "html"
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "text"
	case ".csv":
		return "csv"
	case ".pdf":
		return "pdf"
	}
	return "binary"
}

func isUploadPreviewable(renderedType, mimeType string) bool {
	switch renderedType {
	case "html", "markdown", "text", "csv":
		return true
	}
	return strings.HasPrefix(strings.ToLower(mimeType), "text/")
}

func uploadExtension(filename string) string {
	if i := strings.LastIndex(filename, "."); i >= 0 {
		return strings.ToLower(filename[i:])
	}
	return ""
}

func newUploadAssetID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return "asset_" + hex.EncodeToString(buf)
}

func uploadChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func storeUploadedAssetContent(
	ctx context.Context,
	storagePath string,
	data []byte,
	assetID string,
	writeFile func(context.Context, string, []byte) (storage.StoredFile, error),
	deleteAsset func(context.Context, string) error,
	logger *slog.Logger,
) error {
	if _, err := writeFile(ctx, storagePath, data); err != nil {
		rollbackUploadedAssetRecord(assetID, storagePath, deleteAsset, logger)
		return err
	}
	return nil
}

func rollbackUploadedAssetRecord(
	assetID string,
	storagePath string,
	deleteAsset func(context.Context, string) error,
	logger *slog.Logger,
) {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()

	if deleteErr := deleteAsset(cleanupCtx, assetID); deleteErr != nil && !errors.Is(deleteErr, assets.ErrNotFound) {
		logger.Error("upload: rollback asset record", "error", deleteErr, "asset_id", assetID, "path", storagePath)
	}
}
