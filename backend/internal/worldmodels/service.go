package worldmodels

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type Service struct {
	repo        Repository
	idGenerator func() string
	demoSeed    []byte
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:        repo,
		idGenerator: newWorldModelID,
		demoSeed:    []byte(demoWorldModelJSON),
	}
}

func (s *Service) List(ctx context.Context) ([]WorldModelSummary, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]WorldModelSummary, 0, len(items))
	for _, item := range items {
		summary, err := buildSummary(item)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (s *Service) Get(ctx context.Context, id string) (StoredWorldModel, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, payload []byte) (StoredWorldModel, error) {
	name, description, err := validatePayload(payload)
	if err != nil {
		return StoredWorldModel{}, err
	}

	return s.repo.Create(ctx, StoredWorldModel{
		ID:          s.idGenerator(),
		Name:        name,
		Description: description,
		JSONBlob:    string(payload),
	})
}

func (s *Service) Update(ctx context.Context, id string, payload []byte) (StoredWorldModel, error) {
	name, description, err := validatePayload(payload)
	if err != nil {
		return StoredWorldModel{}, err
	}

	return s.repo.Update(ctx, StoredWorldModel{
		ID:          id,
		Name:        name,
		Description: description,
		JSONBlob:    string(payload),
	})
}

func (s *Service) EnsureSeedData(ctx context.Context) error {
	if _, err := s.repo.Get(ctx, DemoWorldModelID); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}

	name, description, err := validatePayload(s.demoSeed)
	if err != nil {
		return fmt.Errorf("normalize demo world model: %w", err)
	}

	_, err = s.repo.Create(ctx, StoredWorldModel{
		ID:          DemoWorldModelID,
		Name:        name,
		Description: description,
		JSONBlob:    string(s.demoSeed),
	})
	if errors.Is(err, ErrAlreadyExists) {
		return nil
	}
	return err
}

func Expand(item StoredWorldModel) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(item.JSONBlob), &payload); err != nil {
		return nil, fmt.Errorf("decode world model payload: %w", err)
	}

	normalizeOptionalArrayFields(payload)
	payload["id"] = item.ID
	payload["name"] = item.Name
	payload["description"] = item.Description
	payload["createdAt"] = item.CreatedAt
	payload["updatedAt"] = item.UpdatedAt

	return payload, nil
}

func buildSummary(item StoredWorldModel) (WorldModelSummary, error) {
	var payload WorldModel
	if err := json.Unmarshal([]byte(item.JSONBlob), &payload); err != nil {
		return WorldModelSummary{}, fmt.Errorf("decode world model summary for %q: %w", item.ID, err)
	}
	normalizeOptionalSlices(&payload)

	return WorldModelSummary{
		ID:                 item.ID,
		Name:               item.Name,
		Description:        item.Description,
		DepartmentCount:    len(payload.Departments),
		EmployeeCount:      len(payload.Employees),
		ProjectCount:       len(payload.Projects),
		DocumentThemeCount: len(payload.DocumentThemes),
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}, nil
}

func validatePayload(payload []byte) (string, string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return "", "", ValidationError{Message: "request body must be a JSON object"}
	}

	orgRaw, ok := raw["organization"]
	if !ok || isNullJSON(orgRaw) {
		return "", "", ValidationError{Message: "organization is required"}
	}
	var organization Organization
	if err := json.Unmarshal(orgRaw, &organization); err != nil {
		return "", "", ValidationError{Message: "organization must be an object"}
	}
	if trimmed(organization.Name) == "" {
		return "", "", ValidationError{Message: "organization.name is required"}
	}
	if trimmed(organization.Industry) == "" {
		return "", "", ValidationError{Message: "organization.industry is required"}
	}
	if trimmed(organization.Size) == "" {
		return "", "", ValidationError{Message: "organization.size is required"}
	}
	if trimmed(organization.Region) == "" {
		return "", "", ValidationError{Message: "organization.region is required"}
	}
	if trimmed(organization.DomainTheme) == "" {
		return "", "", ValidationError{Message: "organization.domain_theme is required"}
	}

	brandingRaw, ok := raw["branding"]
	if !ok || isNullJSON(brandingRaw) {
		return "", "", ValidationError{Message: "branding is required"}
	}
	var branding Branding
	if err := json.Unmarshal(brandingRaw, &branding); err != nil {
		return "", "", ValidationError{Message: "branding must be an object"}
	}
	if trimmed(branding.Tone) == "" {
		return "", "", ValidationError{Message: "branding.tone is required"}
	}

	if err := validateRequiredArray(raw, "departments", func(data []byte) error {
		var departments []string
		if err := json.Unmarshal(data, &departments); err != nil {
			return ValidationError{Message: "departments must be an array"}
		}
		for _, department := range departments {
			if trimmed(department) == "" {
				return ValidationError{Message: "departments[] must be a non-empty string"}
			}
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	if err := validateRequiredArray(raw, "employees", func(data []byte) error {
		var employees []Employee
		if err := json.Unmarshal(data, &employees); err != nil {
			return ValidationError{Message: "employees must be an array"}
		}
		for _, employee := range employees {
			if trimmed(employee.Name) == "" {
				return ValidationError{Message: "employees[].name is required"}
			}
			if trimmed(employee.Role) == "" {
				return ValidationError{Message: "employees[].role is required"}
			}
			if trimmed(employee.Department) == "" {
				return ValidationError{Message: "employees[].department is required"}
			}
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	if err := validateRequiredArray(raw, "projects", func(data []byte) error {
		var projects []string
		if err := json.Unmarshal(data, &projects); err != nil {
			return ValidationError{Message: "projects must be an array"}
		}
		for _, project := range projects {
			if trimmed(project) == "" {
				return ValidationError{Message: "projects[] must be a non-empty string"}
			}
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	if err := validateRequiredArray(raw, "document_themes", func(data []byte) error {
		var documentThemes []string
		if err := json.Unmarshal(data, &documentThemes); err != nil {
			return ValidationError{Message: "document_themes must be an array"}
		}
		for _, theme := range documentThemes {
			if trimmed(theme) == "" {
				return ValidationError{Message: "document_themes[] must be a non-empty string"}
			}
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	return trimmed(organization.Name), trimmed(organization.Description), nil
}

func normalizeOptionalSlices(worldModel *WorldModel) {
	if worldModel.Branding.Colors == nil {
		worldModel.Branding.Colors = []string{}
	}
}

func normalizeOptionalArrayFields(payload map[string]any) {
	nested, ok := payload["branding"].(map[string]any)
	if !ok {
		return
	}

	if colors, exists := nested["colors"]; !exists || colors == nil {
		nested["colors"] = []any{}
	}
}

func validateRequiredArray(raw map[string]json.RawMessage, key string, validate func([]byte) error) error {
	data, ok := raw[key]
	if !ok || isNullJSON(data) {
		return ValidationError{Message: key + " is required"}
	}

	return validate(data)
}

func isNullJSON(data []byte) bool {
	return string(data) == "null"
}

func newWorldModelID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return "wm_" + hex.EncodeToString(buf)
}

const demoWorldModelJSON = `{
  "organization": {
    "name": "Northbridge Financial Advisory",
    "industry": "Financial Services",
    "size": "mid-size",
    "region": "United States",
    "domain_theme": "northbridgefinancial.local"
  },
  "branding": {
    "tone": "formal",
    "colors": ["#123B5D", "#B58A3B"]
  },
  "departments": [
    "Finance",
    "Human Resources",
    "Information Technology",
    "Operations",
    "Compliance"
  ],
  "employees": [
    { "name": "Lauren Chen", "role": "Managing Director", "department": "Finance" },
    { "name": "Marcus Bell", "role": "Controller", "department": "Finance" },
    { "name": "Priya Nair", "role": "HR Manager", "department": "Human Resources" },
    { "name": "Dylan Brooks", "role": "IT Lead", "department": "Information Technology" },
    { "name": "Avery Patel", "role": "Operations Manager", "department": "Operations" },
    { "name": "Sofia Ramirez", "role": "Compliance Officer", "department": "Compliance" },
    { "name": "Ethan Cole", "role": "Financial Analyst", "department": "Finance" },
    { "name": "Grace Kim", "role": "Client Operations Specialist", "department": "Operations" },
    { "name": "Noah Foster", "role": "Systems Administrator", "department": "Information Technology" },
    { "name": "Maya Singh", "role": "Talent Coordinator", "department": "Human Resources" }
  ],
  "projects": [
    "Quarterly Portfolio Review",
    "SOX Control Refresh",
    "Endpoint Upgrade Initiative",
    "Benefits Renewal"
  ],
  "document_themes": [
    "budgets",
    "policies",
    "meeting notes",
    "vendor lists",
    "roadmaps"
  ]
}`
