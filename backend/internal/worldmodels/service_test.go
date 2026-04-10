package worldmodels

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceCreatePreservesSpecShape(t *testing.T) {
	database := newTestDatabase(t)
	service := NewService(NewRepository(database))

	model, err := service.Create(context.Background(), []byte(`{
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
		"employees": [
			{"name":"Alex Morgan","role":"Analyst","department":"Finance"}
		],
		"projects": ["Portfolio Refresh"],
		"document_themes": ["budgets"]
	}`))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if model.Name != "Acme Advisory" {
		t.Fatalf("model.Name = %q, want %q", model.Name, "Acme Advisory")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(model.JSONBlob), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload["document_themes"] == nil {
		t.Fatalf("payload = %#v, want document_themes key", payload)
	}
	if payload["documentThemes"] != nil {
		t.Fatalf("payload = %#v, want documentThemes key removed", payload)
	}

	organization, ok := payload["organization"].(map[string]any)
	if !ok {
		t.Fatalf("payload[organization] = %#v, want object", payload["organization"])
	}
	for _, key := range []string{"name", "industry", "size", "region", "domain_theme"} {
		if organization[key] == nil || organization[key] == "" {
			t.Fatalf("organization[%q] = %#v, want non-empty value", key, organization[key])
		}
	}

	employees, ok := payload["employees"].([]any)
	if !ok || len(employees) != 1 {
		t.Fatalf("payload[employees] = %#v, want one employee", payload["employees"])
	}
	employee, ok := employees[0].(map[string]any)
	if !ok {
		t.Fatalf("employees[0] = %#v, want object", employees[0])
	}
	for _, key := range []string{"name", "role", "department"} {
		if employee[key] == nil || employee[key] == "" {
			t.Fatalf("employee[%q] = %#v, want non-empty value", key, employee[key])
		}
	}
}

func TestServiceCreateValidatesRequiredFields(t *testing.T) {
	database := newTestDatabase(t)
	service := NewService(NewRepository(database))

	testCases := []struct {
		name    string
		payload string
		message string
	}{
		{
			name:    "missing organization",
			payload: `{"branding":{}}`,
			message: "organization is required",
		},
		{
			name:    "missing branding",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"}}`,
			message: "branding is required",
		},
		{
			name:    "missing organization name",
			payload: `{"organization":{"industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
			message: "organization.name is required",
		},
		{
			name:    "missing organization industry",
			payload: `{"organization":{"name":"Acme Advisory","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
			message: "organization.industry is required",
		},
		{
			name:    "missing organization size",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
			message: "organization.size is required",
		},
		{
			name:    "missing organization region",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
			message: "organization.region is required",
		},
		{
			name:    "missing organization domain theme",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
			message: "organization.domain_theme is required",
		},
		{
			name:    "missing branding tone",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
			message: "branding.tone is required",
		},
		{
			name:    "missing departments",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"employees":[],"projects":[],"document_themes":[]}`,
			message: "departments is required",
		},
		{
			name:    "missing employees",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"projects":[],"document_themes":[]}`,
			message: "employees is required",
		},
		{
			name:    "missing projects",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"document_themes":[]}`,
			message: "projects is required",
		},
		{
			name:    "missing document themes",
			payload: `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[]}`,
			message: "document_themes is required",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), []byte(testCase.payload))
			var validationErr ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Create() error = %v, want ValidationError", err)
			}
			if validationErr.Message != testCase.message {
				t.Fatalf("validationErr.Message = %q, want %q", validationErr.Message, testCase.message)
			}
		})
	}
}

func TestServiceEnsureSeedIsIdempotent(t *testing.T) {
	database := newTestDatabase(t)
	service := NewService(NewRepository(database))

	if err := service.EnsureSeedData(context.Background()); err != nil {
		t.Fatalf("EnsureSeedData() first call error = %v", err)
	}
	if err := service.EnsureSeedData(context.Background()); err != nil {
		t.Fatalf("EnsureSeedData() second call error = %v", err)
	}

	items, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(List()) = %d, want %d", len(items), 1)
	}
	if items[0].ID != DemoWorldModelID {
		t.Fatalf("items[0].ID = %q, want %q", items[0].ID, DemoWorldModelID)
	}
}

func TestDemoSampleDataMatchesBootstrapSeed(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "..", "sample-data", "world-models", "northbridge-financial.json"))
	fileContents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var samplePayload any
	if err := json.Unmarshal(fileContents, &samplePayload); err != nil {
		t.Fatalf("json.Unmarshal(sample file) error = %v", err)
	}

	var embeddedPayload any
	if err := json.Unmarshal([]byte(demoWorldModelJSON), &embeddedPayload); err != nil {
		t.Fatalf("json.Unmarshal(embedded seed) error = %v", err)
	}

	sampleJSON, err := json.Marshal(samplePayload)
	if err != nil {
		t.Fatalf("json.Marshal(sample file) error = %v", err)
	}
	embeddedJSON, err := json.Marshal(embeddedPayload)
	if err != nil {
		t.Fatalf("json.Marshal(embedded seed) error = %v", err)
	}

	if string(sampleJSON) != string(embeddedJSON) {
		t.Fatal("sample file does not match embedded seed")
	}
}
