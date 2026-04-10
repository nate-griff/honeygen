package api

import (
	"encoding/json"
	"net/http"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/models"
)

func NewRouter(application *app.APIApp) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/health", healthHandler(application))
	mux.HandleFunc("/api/status", statusHandler(application))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, models.APIErrorResponse{
		Error: models.APIError{
			Code:    code,
			Message: message,
		},
	})
}
