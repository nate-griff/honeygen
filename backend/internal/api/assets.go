package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/assets"
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
		default:
			w.Header().Set("Allow", http.MethodGet)
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
