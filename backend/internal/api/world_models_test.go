package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/models"
)

func TestWorldModelsListReturnsSeededSummary(t *testing.T) {
	application := newTestAPIApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/world-models", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

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

	createReq := httptest.NewRequest(http.MethodPost, "/api/world-models", strings.NewReader(`{
		"organization": {
			"name": "Acme Advisory",
			"description": "Boutique advisory firm"
		},
		"branding": {
			"tone": "professional"
		}
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status code = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created struct {
		ID             string           `json:"id"`
		Name           string           `json:"name"`
		Departments    []map[string]any `json:"departments"`
		Employees      []map[string]any `json:"employees"`
		Projects       []map[string]any `json:"projects"`
		DocumentThemes []map[string]any `json:"documentThemes"`
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
	if created.Departments == nil || created.Employees == nil || created.Projects == nil || created.DocumentThemes == nil {
		t.Fatalf("expected empty arrays in create response, got %+v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/world-models/"+created.ID, nil)
	getRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status code = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/world-models/"+created.ID, strings.NewReader(`{
		"organization": {
			"name": "Acme Advisory Updated",
			"description": "Updated boutique advisory firm"
		},
		"branding": {
			"tone": "modern"
		},
		"departments": [{"name":"Finance"}],
		"employees": [{"fullName":"Alex Morgan","title":"Analyst","department":"Finance"}],
		"projects": [{"name":"Portfolio Refresh","status":"active"}],
		"documentThemes": [{"name":"Compliance","description":"Regulatory summaries"}]
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status code = %d, want %d, body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	var updated struct {
		Name         string `json:"name"`
		Organization struct {
			Name string `json:"name"`
		} `json:"organization"`
		Departments []struct {
			Name string `json:"name"`
		} `json:"departments"`
	}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v", err)
	}
	if updated.Name != "Acme Advisory Updated" || updated.Organization.Name != "Acme Advisory Updated" {
		t.Fatalf("updated = %+v, want updated names", updated)
	}
	if len(updated.Departments) != 1 || updated.Departments[0].Name != "Finance" {
		t.Fatalf("updated.Departments = %+v, want one Finance department", updated.Departments)
	}
}

func TestWorldModelsValidationErrorsUseCanonicalShape(t *testing.T) {
	application := newTestAPIApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/world-models", strings.NewReader(`{"branding":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "validation_error",
			Message: "organization is required",
		},
	})
}
