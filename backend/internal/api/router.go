package api

import (
	"encoding/json"
	"net/http"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/models"
)

func NewRouter(application *app.APIApp) http.Handler {
	mux := http.NewServeMux()
	protected := func(next http.HandlerFunc) http.HandlerFunc {
		return requireAdminSession(application, next)
	}

	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/auth/login", allowMethod(http.MethodPost, loginHandler(application)))
	mux.HandleFunc("/api/auth/logout", allowMethod(http.MethodPost, logoutHandler(application)))
	mux.HandleFunc("/api/auth/session", allowMethod(http.MethodGet, sessionHandler(application)))
	mux.HandleFunc("/api/health", protected(allowMethod(http.MethodGet, healthHandler(application))))
	mux.HandleFunc("/api/provider/test", protected(allowMethod(http.MethodPost, providerTestHandler(application))))
	mux.HandleFunc("/api/status", protected(allowMethod(http.MethodGet, statusHandler(application))))
	mux.HandleFunc("/api/generation/run", protected(allowMethod(http.MethodPost, generationRunHandler(application))))
	mux.HandleFunc("/api/generation/jobs", protected(allowMethod(http.MethodGet, generationJobsListHandler(application))))
	mux.HandleFunc("/api/generation/jobs/", protected(generationJobsRoutingHandler(application)))
	mux.HandleFunc("/api/assets/tree", protected(allowMethod(http.MethodGet, assetsTreeHandler(application))))
	mux.HandleFunc("/api/assets", protected(allowMethod(http.MethodGet, assetsListHandler(application))))
	mux.HandleFunc("/api/assets/", protected(assetsItemHandler(application)))
	mux.HandleFunc("/api/events", protected(allowMethod(http.MethodGet, eventsListHandler(application))))
	mux.HandleFunc("/api/events/", protected(allowMethod(http.MethodGet, eventsItemHandler(application))))
	mux.HandleFunc("/internal/events", allowMethod(http.MethodPost, internalEventsHandler(application)))
	mux.HandleFunc("/api/world-models/generate", protected(worldModelGenerateHandler(application)))
	mux.HandleFunc("/api/world-models", protected(worldModelsCollectionHandler(application)))
	mux.HandleFunc("/api/world-models/", protected(worldModelItemHandler(application)))
	mux.HandleFunc("/api/settings/provider", protected(settingsProviderHandler(application)))
	mux.HandleFunc("/api/deployments", protected(deploymentsCollectionHandler(application)))
	mux.HandleFunc("/api/deployments/", protected(deploymentRoutingHandler(application)))
	mux.HandleFunc("/api/", protected(apiNotFoundHandler))
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
