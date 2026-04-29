package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/config"
	"github.com/natet/honeygen/backend/internal/models"
)

func TestAPIUnknownRouteReturnsCanonicalJSONError(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusNotFound, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "not_found",
			Message: "resource not found",
		},
	})
}

func TestAPIWrongMethodReturnsCanonicalJSONError(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodPost, "/api/status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusMethodNotAllowed, http.MethodGet, models.APIErrorResponse{
		Error: models.APIError{
			Code:    "method_not_allowed",
			Message: "method not allowed",
		},
	})
}

func newTestAPIApp(t *testing.T) *app.APIApp {
	t.Helper()

	cfg := config.Config{
		ServiceName:                 "honeygen-api",
		ServiceVersion:              "test",
		AppEnv:                      "test",
		HTTPAddr:                    ":0",
		AdminPassword:               "test-admin-password",
		InternalEventIngestToken:    "test-internal-event-token",
		ProviderConfigEncryptionKey: "test-provider-config-encryption-key",
		SQLitePath:                  filepath.Join(t.TempDir(), "api.db"),
		GeneratedAssetsDir:          filepath.Join(t.TempDir(), "generated"),
		StorageRoot:                 filepath.Join(t.TempDir(), "storage"),
	}

	application, err := app.NewAPIApp(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewAPIApp() error = %v", err)
	}

	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	return application
}

func authenticatedRequest(t *testing.T, router http.Handler, method, target string, body io.Reader) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, body)
	req.AddCookie(loginTestSessionCookie(t, router))
	return req
}

func loginTestSessionCookie(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"test-admin-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatal("login response did not set a session cookie")
	}

	return cookies[0]
}

func assertAPIErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantAllow string, want models.APIErrorResponse) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}
	if got := rec.Header().Get("Allow"); got != wantAllow {
		t.Fatalf("Allow = %q, want %q", got, wantAllow)
	}

	var got models.APIErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got != want {
		t.Fatalf("error response = %+v, want %+v", got, want)
	}
}
