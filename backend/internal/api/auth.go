package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/natet/honeygen/backend/internal/app"
)

const adminSessionCookieName = "honeygen_admin_session"
const adminSessionCookiePath = "/api"

func loginHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := readJSONBody(w, r)
		if err != nil {
			writeValidationFailure(w, err)
			return
		}

		var request struct {
			Password string `json:"password"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			writeValidationFailure(w, requestValidationError{message: "request body must be a JSON object"})
			return
		}

		if !constantTimePasswordMatch(request.Password, application.Config.AdminPassword) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}

		sessionToken, err := application.AdminSessions.Create()
		if err != nil {
			application.Logger.Error("create admin session", "error", err)
			writeError(w, http.StatusInternalServerError, "session_error", "unable to create session")
			return
		}

		http.SetCookie(w, sessionCookie(application, sessionToken))
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
	}
}

func logoutHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(adminSessionCookieName); err == nil {
			application.AdminSessions.Delete(cookie.Value)
		}

		expired := sessionCookie(application, "")
		expired.MaxAge = -1
		expired.Expires = time.Unix(0, 0)
		http.SetCookie(w, expired)
		w.WriteHeader(http.StatusNoContent)
	}
}

func sessionHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasValidAdminSession(application, r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
	}
}

func requireAdminSession(application *app.APIApp, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasValidAdminSession(application, r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		next(w, r)
	}
}

func hasValidAdminSession(application *app.APIApp, r *http.Request) bool {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return application.AdminSessions.Valid(cookie.Value)
}

func sessionCookie(application *app.APIApp, value string) *http.Cookie {
	return &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    value,
		Path:     adminSessionCookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   application.Config.AppEnv != "development" && application.Config.AppEnv != "test",
	}
}

func constantTimePasswordMatch(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}
