package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderSettingsEncryptsAPIKeyAtRest(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodPut, "/api/settings/provider", strings.NewReader(`{
		"base_url":"https://provider.example/v1",
		"api_key":"super-secret-api-key",
		"model":"gpt-4.1-mini"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var valueJSON string
	if err := application.DB.QueryRowContext(context.Background(), `SELECT value_json FROM settings WHERE key = 'provider'`).Scan(&valueJSON); err != nil {
		t.Fatalf("query value_json error = %v", err)
	}
	if strings.Contains(valueJSON, "super-secret-api-key") {
		t.Fatalf("stored settings unexpectedly contained plaintext API key: %s", valueJSON)
	}
	if !strings.Contains(valueJSON, `"encrypted_api_key"`) {
		t.Fatalf("stored settings did not include encrypted_api_key: %s", valueJSON)
	}
}
