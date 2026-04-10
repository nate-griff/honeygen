package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/events"
)

func internalEventsHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configuredToken := strings.TrimSpace(application.Config.InternalEventIngestToken)
		if configuredToken == "" {
			application.Logger.Error("internal event ingestion token is not configured")
			writeError(w, http.StatusServiceUnavailable, "events_unavailable", "events are temporarily unavailable")
			return
		}
		providedToken := strings.TrimSpace(r.Header.Get(events.InternalIngestTokenHeader))
		if subtle.ConstantTimeCompare([]byte(providedToken), []byte(configuredToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "internal event token is invalid")
			return
		}

		payload, err := decodeIngestRequest(w, r)
		if err != nil {
			var validationErr requestValidationError
			if errors.As(err, &validationErr) {
				writeError(w, http.StatusBadRequest, "invalid_request", validationErr.Error())
				return
			}
			application.Logger.Error("decode event ingestion request", "error", err)
			writeError(w, http.StatusInternalServerError, "events_unavailable", "events are temporarily unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		item, err := application.EventService.Ingest(ctx, payload)
		if err != nil {
			var validationErr events.ValidationError
			if errors.As(err, &validationErr) {
				writeError(w, http.StatusBadRequest, "invalid_request", validationErr.Error())
				return
			}
			application.Logger.Error("ingest event", "error", err)
			writeError(w, http.StatusInternalServerError, "events_unavailable", "events are temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusCreated, item)
	}
}
