package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	appdb "github.com/natet/honeygen/backend/internal/db"
	"github.com/natet/honeygen/backend/internal/provider"
)

type providerTestRequest struct {
	GenerationJobID string `json:"generation_job_id"`
}

func providerTestHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, err := readProviderTestRequest(w, r)
		if err != nil {
			writeValidationFailure(w, err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := application.Provider.Test(ctx); err != nil {
			recordProviderFailure(application, ctx, request.GenerationJobID, err)
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

func readProviderTestRequest(w http.ResponseWriter, r *http.Request) (providerTestRequest, error) {
	body, err := readOptionalJSONBody(w, r)
	if err != nil || len(body) == 0 {
		return providerTestRequest{}, err
	}

	var request providerTestRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return providerTestRequest{}, requestValidationError{message: "request body must be a JSON object"}
	}

	request.GenerationJobID = strings.TrimSpace(request.GenerationJobID)

	return request, nil
}

func recordProviderFailure(application *app.APIApp, ctx context.Context, generationJobID string, err error) {
	if generationJobID == "" || application == nil || application.DB == nil {
		return
	}

	message := provider.SafeErrorMessage(err)

	recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if recordErr := appdb.NewGenerationJobRecorder(application.DB).RecordProviderFailure(recordCtx, generationJobID, message); recordErr != nil {
		application.Logger.Error("record provider failure", "error", recordErr, "generation_job_id", generationJobID)
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
