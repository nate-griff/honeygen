package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/events"
)

func eventsListHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		items, err := application.EventService.List(ctx, events.ListOptions{
			Limit:        parseOptionalInt(r.URL.Query().Get("limit"), 100),
			Offset:       parseOptionalInt(r.URL.Query().Get("offset"), 0),
			WorldModelID: strings.TrimSpace(r.URL.Query().Get("world_model_id")),
			Path:         strings.TrimSpace(r.URL.Query().Get("path")),
			SourceIP:     strings.TrimSpace(r.URL.Query().Get("source_ip")),
			StatusCode:   parseOptionalInt(r.URL.Query().Get("status_code"), 0),
		})
		if err != nil {
			application.Logger.Error("list events", "error", err)
			writeError(w, http.StatusInternalServerError, "events_unavailable", "events are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func eventsItemHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := eventIDFromPath(r.URL.Path)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "resource not found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		item, err := application.EventService.Get(ctx, id)
		if errors.Is(err, events.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err != nil {
			application.Logger.Error("get event", "error", err, "id", id)
			writeError(w, http.StatusInternalServerError, "events_unavailable", "events are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, item)
	}
}

func eventIDFromPath(path string) (string, error) {
	const prefix = "/api/events/"
	if !strings.HasPrefix(path, prefix) {
		return "", requestValidationError{message: "event id is required"}
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") {
		return "", requestValidationError{message: "event id is required"}
	}
	return id, nil
}

func decodeIngestRequest(w http.ResponseWriter, r *http.Request) (events.IngestRequest, error) {
	body, err := readJSONBody(w, r)
	if err != nil {
		return events.IngestRequest{}, err
	}

	var payload events.IngestRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return events.IngestRequest{}, requestValidationError{message: "request body must be valid JSON"}
	}
	return payload, nil
}
