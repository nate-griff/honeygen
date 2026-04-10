package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/provider"
)

func providerTestHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := application.Provider.Test(ctx); err != nil {
			writeProviderError(application, w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ready":    true,
			"mode":     application.Config.Provider.Mode(),
			"base_url": application.Config.Provider.BaseURL,
			"model":    application.Config.Provider.Model,
		})
	}
}

func writeProviderError(application *app.APIApp, w http.ResponseWriter, err error) {
	var providerErr *provider.Error
	if !errors.As(err, &providerErr) {
		application.Logger.Error("test provider", "error", err)
		writeError(w, http.StatusBadGateway, "provider_unavailable", provider.SafeErrorMessage(err))
		return
	}

	message := provider.SafeErrorMessage(err)

	switch providerErr.Kind {
	case provider.KindConfig:
		writeError(w, http.StatusBadRequest, "provider_invalid", message)
	case provider.KindUnauthorized:
		application.Logger.Warn("provider authentication failed", "error", err)
		writeError(w, http.StatusBadGateway, "provider_auth_failed", message)
	case provider.KindConnectivity:
		application.Logger.Warn("provider connectivity failed", "error", err)
		writeError(w, http.StatusBadGateway, "provider_unreachable", message)
	case provider.KindInvalidResponse:
		application.Logger.Warn("provider returned invalid response", "error", err)
		writeError(w, http.StatusBadGateway, "provider_invalid_response", message)
	default:
		application.Logger.Warn("provider upstream error", "error", err)
		writeError(w, http.StatusBadGateway, "provider_unavailable", message)
	}
}
