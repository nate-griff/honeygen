package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func worldModelsCollectionHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleWorldModelsList(application, w, r)
		case http.MethodPost:
			handleWorldModelsCreate(application, w, r)
		default:
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func worldModelItemHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleWorldModelGet(application, w, r)
		case http.MethodPut:
			handleWorldModelUpdate(application, w, r)
		default:
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func handleWorldModelsList(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	items, err := application.WorldModels.List(ctx)
	if err != nil {
		application.Logger.Error("list world models", "error", err)
		writeError(w, http.StatusInternalServerError, "world_models_unavailable", "world models are temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleWorldModelGet(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := worldModelIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.WorldModels.Get(ctx, id)
	if err != nil {
		writeWorldModelError(application, w, "get world model", err)
		return
	}

	response, err := worldmodels.Expand(item)
	if err != nil {
		application.Logger.Error("expand world model", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func handleWorldModelsCreate(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(w, r)
	if err != nil {
		writeValidationFailure(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.WorldModels.Create(ctx, body)
	if err != nil {
		writeWorldModelError(application, w, "create world model", err)
		return
	}

	response, err := worldmodels.Expand(item)
	if err != nil {
		application.Logger.Error("expand created world model", "error", err, "id", item.ID)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func handleWorldModelUpdate(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := worldModelIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	body, err := readJSONBody(w, r)
	if err != nil {
		writeValidationFailure(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.WorldModels.Update(ctx, id, body)
	if err != nil {
		writeWorldModelError(application, w, "update world model", err)
		return
	}

	response, err := worldmodels.Expand(item)
	if err != nil {
		application.Logger.Error("expand updated world model", "error", err, "id", item.ID)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func writeWorldModelError(application *app.APIApp, w http.ResponseWriter, action string, err error) {
	var validationErr worldmodels.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, "validation_error", validationErr.Message)
	case isRequestValidationError(err):
		writeValidationFailure(w, err)
	case errors.Is(err, worldmodels.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "world model not found")
	default:
		application.Logger.Error(action, "error", err)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
	}
}

func writeValidationFailure(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, "validation_error", err.Error())
}
