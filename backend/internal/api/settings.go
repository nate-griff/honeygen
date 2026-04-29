package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/config"
)

type providerSettingsResponse struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	Ready   bool   `json:"ready"`
	Mode    string `json:"mode"`
}

func settingsProviderHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetProviderSettings(application, w, r)
		case http.MethodPut:
			handlePutProviderSettings(application, w, r)
		default:
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func handleGetProviderSettings(application *app.APIApp, w http.ResponseWriter, _ *http.Request) {
	cfg := application.ProviderConfig()
	writeJSON(w, http.StatusOK, providerSettingsResponse{
		BaseURL: cfg.BaseURL,
		APIKey:  maskAPIKey(cfg.APIKey),
		Model:   cfg.Model,
		Ready:   cfg.Ready(),
		Mode:    cfg.Mode(),
	})
}

func handlePutProviderSettings(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(w, r)
	if err != nil {
		writeValidationFailure(w, err)
		return
	}

	var request struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeValidationFailure(w, requestValidationError{message: "request body must be a JSON object"})
		return
	}

	request.BaseURL = strings.TrimSpace(request.BaseURL)
	request.APIKey = strings.TrimSpace(request.APIKey)
	request.Model = strings.TrimSpace(request.Model)

	current := application.ProviderConfig()
	if request.APIKey == maskAPIKey(current.APIKey) || request.APIKey == "" {
		request.APIKey = current.APIKey
	}

	newCfg := config.ProviderConfig{
		BaseURL: request.BaseURL,
		APIKey:  request.APIKey,
		Model:   request.Model,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	encryptedAPIKey, err := application.ProviderConfigCodec.EncryptString(newCfg.APIKey)
	if err != nil {
		application.Logger.Error("encrypt provider settings", "error", err)
		writeError(w, http.StatusInternalServerError, "settings_error", "unable to save provider settings")
		return
	}

	valueJSON, err := json.Marshal(struct {
		BaseURL         string `json:"base_url"`
		EncryptedAPIKey string `json:"encrypted_api_key,omitempty"`
		Model           string `json:"model"`
	}{
		BaseURL:         newCfg.BaseURL,
		EncryptedAPIKey: encryptedAPIKey,
		Model:           newCfg.Model,
	})
	if err != nil {
		application.Logger.Error("encode provider settings", "error", err)
		writeError(w, http.StatusInternalServerError, "settings_error", "unable to save provider settings")
		return
	}
	if err := application.Settings.Put(ctx, "provider", valueJSON); err != nil {
		application.Logger.Error("save provider settings", "error", err)
		writeError(w, http.StatusInternalServerError, "settings_error", "unable to save provider settings")
		return
	}

	application.UpdateProvider(newCfg)

	application.Logger.Info("provider settings updated", "ready", newCfg.Ready(), "mode", newCfg.Mode(), "base_url", newCfg.BaseURL, "model", newCfg.Model)

	writeJSON(w, http.StatusOK, providerSettingsResponse{
		BaseURL: newCfg.BaseURL,
		APIKey:  maskAPIKey(newCfg.APIKey),
		Model:   newCfg.Model,
		Ready:   newCfg.Ready(),
		Mode:    newCfg.Mode(),
	})
}

func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return strings.Repeat("•", len(key))
	}
	return strings.Repeat("•", len(key)-4) + key[len(key)-4:]
}
