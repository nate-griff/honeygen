package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

const (
	codeBlockFence            = "```"
	worldModelGenerateTimeout = 5 * time.Minute
)

func worldModelsCollectionHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleWorldModelsList(application, w, r)
		case http.MethodPost:
			handleWorldModelsCreate(application, w, r)
		default:
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func worldModelItemHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleWorldModelGet(application, w, r)
		case http.MethodPut:
			handleWorldModelUpdate(application, w, r)
		default:
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func handleWorldModelsList(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	items, err := application.WorldModels.List(ctx)
	if err != nil {
		application.Logger.Error("list world models", "error", err)
		writeError(w, http.StatusInternalServerError, "world_models_unavailable", "world models are temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleWorldModelGet(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := worldModelIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.WorldModels.Get(ctx, id)
	if err != nil {
		writeWorldModelError(application, w, "get world model", err)
		return
	}

	response, err := worldmodels.Expand(item)
	if err != nil {
		application.Logger.Error("expand world model", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func handleWorldModelsCreate(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(w, r)
	if err != nil {
		writeValidationFailure(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.WorldModels.Create(ctx, body)
	if err != nil {
		writeWorldModelError(application, w, "create world model", err)
		return
	}

	response, err := worldmodels.Expand(item)
	if err != nil {
		application.Logger.Error("expand created world model", "error", err, "id", item.ID)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func handleWorldModelUpdate(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	id, err := worldModelIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	body, err := readJSONBody(w, r)
	if err != nil {
		writeValidationFailure(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := application.WorldModels.Update(ctx, id, body)
	if err != nil {
		writeWorldModelError(application, w, "update world model", err)
		return
	}

	response, err := worldmodels.Expand(item)
	if err != nil {
		application.Logger.Error("expand updated world model", "error", err, "id", item.ID)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func writeWorldModelError(application *app.APIApp, w http.ResponseWriter, action string, err error) {
	var validationErr worldmodels.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, "validation_error", validationErr.Message)
	case errors.Is(err, worldmodels.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "world model already exists")
	case errors.Is(err, worldmodels.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "world model not found")
	default:
		application.Logger.Error(action, "error", err)
		writeError(w, http.StatusInternalServerError, "world_model_unavailable", "world model is temporarily unavailable")
	}
}

func writeValidationFailure(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, "validation_error", err.Error())
}

func worldModelGenerateHandler(application *app.APIApp) http.HandlerFunc {
	return allowMethod(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		handleWorldModelGenerate(application, w, r)
	})
}

func handleWorldModelGenerate(application *app.APIApp, w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(w, r)
	if err != nil {
		writeValidationFailure(w, err)
		return
	}

	var request struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeValidationFailure(w, requestValidationError{message: "request body must be a JSON object"})
		return
	}
	request.Description = strings.TrimSpace(request.Description)
	if request.Description == "" {
		writeValidationFailure(w, requestValidationError{message: "description is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), worldModelGenerateTimeout)
	defer cancel()

	result, err := application.Provider.Generate(ctx, provider.GenerateRequest{
		SystemPrompt: worldModelSystemPrompt(),
		Prompt:       request.Description,
		Metadata:     map[string]string{"task": "world_model_generation"},
	})
	if err != nil {
		writeProviderError(application, w, err)
		return
	}

	jsonContent := extractJSON(result.Content)

	if !json.Valid([]byte(jsonContent)) {
		application.Logger.Warn("world model generation returned invalid JSON", "content_length", len(result.Content))
		writeError(w, http.StatusBadGateway, "generation_invalid", "the LLM returned content that is not valid JSON — try again or rephrase the description")
		return
	}

	var payload json.RawMessage = json.RawMessage(jsonContent)
	writeJSON(w, http.StatusOK, map[string]any{
		"generated": true,
		"payload":   payload,
	})
}

func worldModelSystemPrompt() string {
	return `You are a world model generator for Honeygen, a honeypot content generation system.
Given a natural language description, generate a complete world model as a JSON object.

The JSON must follow this exact schema:
{
  "organization": {
    "name": "string (required)",
    "description": "string (optional)",
    "industry": "string (required)",
    "size": "string (required, e.g. small, mid-size, large, enterprise)",
    "region": "string (required, e.g. United States, Europe, Asia-Pacific)",
    "domain_theme": "string (required, a plausible internal domain like companyname.local)"
  },
  "branding": {
    "tone": "string (required, e.g. formal, casual, technical)",
    "colors": ["string array of hex colors, at least 2"]
  },
  "departments": ["string array, 3-8 department names"],
  "employees": [
    {"name": "string", "role": "string", "department": "string (must match a department)"}
  ],
  "projects": ["string array, 2-6 project names"],
  "document_themes": ["string array, 3-7 themes like budgets, policies, roadmaps"]
}

Rules:
- Generate 5-12 employees across the departments
- Every employee's department must appear in the departments array
- Use realistic, diverse names
- The domain_theme should be a plausible .local or .internal domain
- Return ONLY the JSON object, no markdown, no explanation, no code fences`
}

func extractJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, codeBlockFence) {
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		if idx := strings.LastIndex(trimmed, codeBlockFence); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}
