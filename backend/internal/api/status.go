package api

import (
	"context"
	"net/http"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
)

func statusHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		status, err := application.Status(ctx)
		if err != nil {
			application.Logger.Error("build status response", "error", err)
			writeError(w, http.StatusInternalServerError, "status_unavailable", "status is temporarily unavailable")
			return
		}

		writeJSON(w, http.StatusOK, status)
	}
}
