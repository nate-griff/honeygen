package worldmodels

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceCreateDefaultsOptionalArraysAndBuildsSummary(t *testing.T) {
	database := newTestDatabase(t)
	service := NewService(NewRepository(database))

	model, err := service.Create(context.Background(), []byte(`{
		"organization": {
			"name": "Acme Advisory",
			"description": "Boutique advisory firm"
		},
		"branding": {
			"tone": "professional"
		}
	}`))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if model.Name != "Acme Advisory" {
		t.Fatalf("model.Name = %q, want %q", model.Name, "Acme Advisory")
	}
	if model.Description != "Boutique advisory firm" {
		t.Fatalf("model.Description = %q, want %q", model.Description, "Boutique advisory firm")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(model.JSONBlob), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{"departments", "employees", "projects", "documentThemes"} {
		values, ok := payload[key].([]any)
		if !ok {
			t.Fatalf("payload[%q] = %#v, want []any", key, payload[key])
		}
		if len(values) != 0 {
			t.Fatalf("len(payload[%q]) = %d, want 0", key, len(values))
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
			payload: `{"organization":{"name":"Acme Advisory"}}`,
			message: "branding is required",
		},
		{
			name:    "missing organization name",
			payload: `{"organization":{"description":"x"},"branding":{}}`,
			message: "organization.name is required",
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
