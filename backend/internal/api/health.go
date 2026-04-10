package api

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
)

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok")
}

func healthHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		writeJSON(w, http.StatusOK, application.Health(ctx))
	}
}
