package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/models"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func TestWorldModelsListReturnsSeededSummary(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodGet, "/api/world-models", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Items []struct {
			ID                 string `json:"id"`
			Name               string `json:"name"`
			EmployeeCount      int    `json:"employeeCount"`
			DocumentThemeCount int    `json:"documentThemeCount"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(response.Items) != 1 {
		t.Fatalf("len(items) = %d, want %d", len(response.Items), 1)
	}
	if response.Items[0].Name != "Northbridge Financial Advisory" {
		t.Fatalf("items[0].Name = %q, want %q", response.Items[0].Name, "Northbridge Financial Advisory")
	}
	if response.Items[0].EmployeeCount < 8 || response.Items[0].EmployeeCount > 12 {
		t.Fatalf("items[0].EmployeeCount = %d, want between 8 and 12", response.Items[0].EmployeeCount)
	}
	if response.Items[0].DocumentThemeCount != 5 {
		t.Fatalf("items[0].DocumentThemeCount = %d, want %d", response.Items[0].DocumentThemeCount, 5)
	}
}

func TestWorldModelsCreateFetchAndUpdate(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	createReq := authenticatedRequest(t, router, http.MethodPost, "/api/world-models", strings.NewReader(`{
		"organization": {
			"name": "Acme Advisory",
			"industry": "Financial Services",
			"size": "mid-size",
			"region": "United States",
			"domain_theme": "acmeadvisory.local"
		},
		"branding": {
			"tone": "professional"
		},
		"departments": ["Finance"],
		"employees": [{"name":"Alex Morgan","role":"Analyst","department":"Finance"}],
		"projects": ["Portfolio Refresh"],
		"document_themes": ["budgets"]
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()

	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status code = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Departments    []string `json:"departments"`
		Projects       []string `json:"projects"`
		DocumentThemes []string `json:"document_themes"`
		Employees      []struct {
			Name       string `json:"name"`
			Role       string `json:"role"`
			Department string `json:"department"`
		} `json:"employees"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("created.ID is empty")
	}
	if created.Name != "Acme Advisory" {
		t.Fatalf("created.Name = %q, want %q", created.Name, "Acme Advisory")
	}
	if len(created.Departments) != 1 || created.Departments[0] != "Finance" {
		t.Fatalf("created.Departments = %+v, want one Finance department", created.Departments)
	}
	if len(created.Employees) != 1 || created.Employees[0].Name != "Alex Morgan" || created.Employees[0].Role != "Analyst" || created.Employees[0].Department != "Finance" {
		t.Fatalf("created.Employees = %+v, want spec employee shape", created.Employees)
	}
	if len(created.Projects) != 1 || created.Projects[0] != "Portfolio Refresh" {
		t.Fatalf("created.Projects = %+v, want one project", created.Projects)
	}
	if len(created.DocumentThemes) != 1 || created.DocumentThemes[0] != "budgets" {
		t.Fatalf("created.DocumentThemes = %+v, want one document theme", created.DocumentThemes)
	}
	assertBrandingColorsEmptyArray(t, createRec.Body.Bytes())

	getReq := authenticatedRequest(t, router, http.MethodGet, "/api/world-models/"+created.ID, nil)
	getRec := httptest.NewRecorder()

	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status code = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	assertBrandingColorsEmptyArray(t, getRec.Body.Bytes())

	updateReq := authenticatedRequest(t, router, http.MethodPut, "/api/world-models/"+created.ID, strings.NewReader(`{
		"organization": {
			"name": "Acme Advisory Updated",
			"industry": "Financial Services",
			"size": "mid-size",
			"region": "Canada",
			"domain_theme": "acmeadvisory.ca"
		},
		"branding": {
			"tone": "modern"
		},
		"departments": ["Finance"],
		"employees": [{"name":"Alex Morgan","role":"Analyst","department":"Finance"}],
		"projects": ["Portfolio Refresh"],
		"document_themes": ["compliance"]
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()

	router.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status code = %d, want %d, body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	var updated struct {
		Name         string `json:"name"`
		Organization struct {
			Name        string `json:"name"`
			Region      string `json:"region"`
			DomainTheme string `json:"domain_theme"`
		} `json:"organization"`
		Departments    []string `json:"departments"`
		DocumentThemes []string `json:"document_themes"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v", err)
	}
	if updated.Name != "Acme Advisory Updated" || updated.Organization.Name != "Acme Advisory Updated" {
		t.Fatalf("updated = %+v, want updated names", updated)
	}
	if updated.Organization.Region != "Canada" || updated.Organization.DomainTheme != "acmeadvisory.ca" {
		t.Fatalf("updated.Organization = %+v, want updated spec fields", updated.Organization)
	}
	if len(updated.Departments) != 1 || updated.Departments[0] != "Finance" {
		t.Fatalf("updated.Departments = %+v, want one Finance department", updated.Departments)
	}
	if len(updated.DocumentThemes) != 1 || updated.DocumentThemes[0] != "compliance" {
		t.Fatalf("updated.DocumentThemes = %+v, want one compliance theme", updated.DocumentThemes)
	}
	assertBrandingColorsEmptyArray(t, updateRec.Body.Bytes())
}

func TestWorldModelsValidationErrorsUseCanonicalShape(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)

	req := authenticatedRequest(t, router, http.MethodPost, "/api/world-models", strings.NewReader(`{"branding":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "validation_error",
			Message: "organization is required",
		},
	})
}

func TestWriteWorldModelErrorReturnsConflictForAlreadyExists(t *testing.T) {
	application := newTestAPIApp(t)
	rec := httptest.NewRecorder()

	writeWorldModelError(application, rec, "create world model", worldmodels.ErrAlreadyExists)

	assertAPIErrorResponse(t, rec, http.StatusConflict, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "already_exists",
			Message: "world model already exists",
		},
	})
}

func TestWorldModelGenerateAllowsExtendedInferenceTimeout(t *testing.T) {
	t.Parallel()

	application := newTestAPIApp(t)
	router := NewRouter(application)
	application.Provider = minimumDeadlineProvider{minimumRemaining: 2 * time.Minute}

	req := authenticatedRequest(t, router, http.MethodPost, "/api/world-models/generate", strings.NewReader(`{"description":"Generate a plausible financial-services organization."}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Generated bool            `json:"generated"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !response.Generated {
		t.Fatalf("generated = %v, want true", response.Generated)
	}
	if string(response.Payload) != "{}" {
		t.Fatalf("payload = %s, want {}", response.Payload)
	}
}

type minimumDeadlineProvider struct {
	minimumRemaining time.Duration
}

func (p minimumDeadlineProvider) Generate(ctx context.Context, _ provider.GenerateRequest) (provider.GenerateResponse, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return provider.GenerateResponse{}, &provider.Error{Kind: provider.KindConnectivity, Message: "missing request deadline"}
	}
	if remaining := time.Until(deadline); remaining < p.minimumRemaining {
		return provider.GenerateResponse{}, &provider.Error{
			Kind:    provider.KindConnectivity,
			Message: fmt.Sprintf("request deadline too short: %s", remaining),
		}
	}

	return provider.GenerateResponse{Content: "{}"}, nil
}

func (minimumDeadlineProvider) Test(context.Context) error { return nil }

func assertBrandingColorsEmptyArray(t *testing.T, body []byte) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	branding, ok := payload["branding"].(map[string]any)
	if !ok {
		t.Fatalf("payload[branding] = %#v, want object", payload["branding"])
	}

	colors, ok := branding["colors"].([]any)
	if !ok {
		t.Fatalf("branding[colors] = %#v, want empty array", branding["colors"])
	}
	if len(colors) != 0 {
		t.Fatalf("branding[colors] = %#v, want empty array", branding["colors"])
	}
}
