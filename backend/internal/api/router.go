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
	mux.HandleFunc("/api/health", allowMethod(http.MethodGet, healthHandler(application)))
	mux.HandleFunc("/api/provider/test", allowMethod(http.MethodPost, providerTestHandler(application)))
	mux.HandleFunc("/api/status", allowMethod(http.MethodGet, statusHandler(application)))
	mux.HandleFunc("/api/generation/run", allowMethod(http.MethodPost, generationRunHandler(application)))
	mux.HandleFunc("/api/generation/jobs", allowMethod(http.MethodGet, generationJobsListHandler(application)))
	mux.HandleFunc("/api/generation/jobs/", allowMethod(http.MethodGet, generationJobsItemHandler(application)))
	mux.HandleFunc("/api/assets/tree", allowMethod(http.MethodGet, assetsTreeHandler(application)))
	mux.HandleFunc("/api/assets", allowMethod(http.MethodGet, assetsListHandler(application)))
	mux.HandleFunc("/api/assets/", assetsItemHandler(application))
	mux.HandleFunc("/api/world-models", worldModelsCollectionHandler(application))
	mux.HandleFunc("/api/world-models/", worldModelItemHandler(application))
	mux.HandleFunc("/api/", apiNotFoundHandler)
	return mux
}

func allowMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		next(w, r)
	}
}

func apiNotFoundHandler(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
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
