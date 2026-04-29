package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/generation"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func generationRunHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := readJSONBody(w, r)
		if err != nil {
			writeValidationFailure(w, err)
			return
		}

		var request generation.RunRequest
		if err := json.Unmarshal(body, &request); err != nil {
			writeValidationFailure(w, requestValidationError{message: "request body must be a JSON object"})
			return
		}
		request.WorldModelID = strings.TrimSpace(request.WorldModelID)
		if request.WorldModelID == "" {
			writeValidationFailure(w, requestValidationError{message: "world_model_id is required"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()

		job, err := application.GenerationService().Run(ctx, request)
		if err != nil {
			if job.ID != "" && job.Status == generation.StatusFailed {
				writeJSON(w, http.StatusBadGateway, job)
				return
			}
			writeGenerationError(application, w, err)
			return
		}

		writeJSON(w, http.StatusCreated, job)
	}
}

func generationJobsListHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		items, err := application.JobStore.List(ctx, generation.ListOptions{
			WorldModelID: strings.TrimSpace(r.URL.Query().Get("world_model_id")),
			Limit:        parseOptionalInt(r.URL.Query().Get("limit"), 100),
			Offset:       parseOptionalInt(r.URL.Query().Get("offset"), 0),
		})
		if err != nil {
			application.Logger.Error("list generation jobs", "error", err)
			writeError(w, http.StatusInternalServerError, "generation_jobs_unavailable", "generation jobs are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func generationJobsItemHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := generationJobIDFromPath(r.URL.Path)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "resource not found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		job, err := application.JobStore.Get(ctx, id)
		if errors.Is(err, generation.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "generation job not found")
			return
		}
		if err != nil {
			application.Logger.Error("get generation job", "error", err, "id", id)
			writeError(w, http.StatusInternalServerError, "generation_jobs_unavailable", "generation jobs are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, job)
	}
}

func generationJobCancelHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := generationJobCancelIDFromPath(r.URL.Path)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "resource not found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		job, err := application.GenerationService().Cancel(ctx, id)
		if errors.Is(err, generation.ErrJobNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "generation job not found")
			return
		}
		if errors.Is(err, generation.ErrJobNotCancelable) {
			writeError(w, http.StatusConflict, "generation_job_not_cancelable", "generation job is not running")
			return
		}
		if err != nil {
			application.Logger.Error("cancel generation job", "error", err, "id", id)
			writeError(w, http.StatusInternalServerError, "generation_cancel_failed", "generation job could not be canceled")
			return
		}

		writeJSON(w, http.StatusOK, job)
	}
}

func generationJobsRoutingHandler(application *app.APIApp) http.HandlerFunc {
	itemHandler := generationJobsItemHandler(application)
	cancelHandler := generationJobCancelHandler(application)

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cancel") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			cancelHandler(w, r)
			return
		}

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		itemHandler(w, r)
	}
}

func writeGenerationError(application *app.APIApp, w http.ResponseWriter, err error) {
	var providerErr *provider.Error
	switch {
	case errors.As(err, &providerErr):
		writeProviderError(application, w, err)
	case errors.Is(err, generation.ErrJobNotFound):
		writeError(w, http.StatusNotFound, "not_found", "generation job not found")
	case errors.Is(err, worldmodels.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "world model not found")
	default:
		application.Logger.Error("run generation job", "error", err)
		writeError(w, http.StatusInternalServerError, "generation_failed", "generation failed")
	}
}

func generationJobIDFromPath(path string) (string, error) {
	const prefix = "/api/generation/jobs/"
	if !strings.HasPrefix(path, prefix) {
		return "", requestValidationError{message: "generation job id is required"}
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") {
		return "", requestValidationError{message: "generation job id is required"}
	}
	return id, nil
}

func generationJobCancelIDFromPath(path string) (string, error) {
	if !strings.HasSuffix(path, "/cancel") {
		return "", requestValidationError{message: "generation job id is required"}
	}
	return generationJobIDFromPath(strings.TrimSuffix(path, "/cancel"))
}

func parseOptionalInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
