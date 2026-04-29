package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/models"
)

func TestProtectedAPIRoutesRequireAuthenticatedSession(t *testing.T) {
	application := newTestAPIApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusUnauthorized, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "unauthorized",
			Message: "authentication required",
		},
	})
}

func TestHealthzRemainsUnauthenticated(t *testing.T) {
	application := newTestAPIApp(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Fatalf("body = %q, want %q", got, "ok")
	}
}

func TestLoginSessionAndLogoutFlow(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"test-admin-password"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status code = %d, want %d, body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var loginBody struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("json.Unmarshal(login) error = %v", err)
	}
	if !loginBody.Authenticated {
		t.Fatalf("login body = %+v, want authenticated session", loginBody)
	}

	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login response did not set a session cookie")
	}
	sessionCookie := cookies[0]
	if sessionCookie.Path != "/api" {
		t.Fatalf("session cookie path = %q, want %q", sessionCookie.Path, "/api")
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionReq.AddCookie(sessionCookie)
	sessionRec := httptest.NewRecorder()
	router.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status code = %d, want %d, body=%s", sessionRec.Code, http.StatusOK, sessionRec.Body.String())
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	protectedReq.AddCookie(sessionCookie)
	protectedRec := httptest.NewRecorder()
	router.ServeHTTP(protectedRec, protectedReq)
	if protectedRec.Code != http.StatusOK {
		t.Fatalf("protected status code = %d, want %d, body=%s", protectedRec.Code, http.StatusOK, protectedRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status code = %d, want %d, body=%s", logoutRec.Code, http.StatusNoContent, logoutRec.Body.String())
	}

	loggedOutReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	loggedOutReq.AddCookie(sessionCookie)
	loggedOutRec := httptest.NewRecorder()
	router.ServeHTTP(loggedOutRec, loggedOutReq)

	assertAPIErrorResponse(t, loggedOutRec, http.StatusUnauthorized, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "unauthorized",
			Message: "authentication required",
		},
	})
}

func TestLoginRejectsInvalidPassword(t *testing.T) {
	application := newTestAPIApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"wrong-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusUnauthorized, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "invalid_credentials",
			Message: "invalid credentials",
		},
	})
}
