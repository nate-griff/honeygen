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
	normalizedPayload, name, description, err := normalizePayload(payload)
	if err != nil {
		return StoredWorldModel{}, err
	}

	return s.repo.Create(ctx, StoredWorldModel{
		ID:          s.idGenerator(),
		Name:        name,
		Description: description,
		JSONBlob:    string(normalizedPayload),
	})
}

func (s *Service) Update(ctx context.Context, id string, payload []byte) (StoredWorldModel, error) {
	normalizedPayload, name, description, err := normalizePayload(payload)
	if err != nil {
		return StoredWorldModel{}, err
	}

	return s.repo.Update(ctx, StoredWorldModel{
		ID:          id,
		Name:        name,
		Description: description,
		JSONBlob:    string(normalizedPayload),
	})
}

func (s *Service) EnsureSeedData(ctx context.Context) error {
	if _, err := s.repo.Get(ctx, DemoWorldModelID); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}

	normalizedPayload, name, description, err := normalizePayload(s.demoSeed)
	if err != nil {
		return fmt.Errorf("normalize demo world model: %w", err)
	}

	_, err = s.repo.Create(ctx, StoredWorldModel{
		ID:          DemoWorldModelID,
		Name:        name,
		Description: description,
		JSONBlob:    string(normalizedPayload),
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

	payload["id"] = item.ID
	payload["name"] = item.Name
	payload["description"] = item.Description
	payload["createdAt"] = item.CreatedAt
	payload["updatedAt"] = item.UpdatedAt

	return payload, nil
}

func buildSummary(item StoredWorldModel) (WorldModelSummary, error) {
	var payload struct {
		Departments    []json.RawMessage `json:"departments"`
		Employees      []json.RawMessage `json:"employees"`
		Projects       []json.RawMessage `json:"projects"`
		DocumentThemes []json.RawMessage `json:"documentThemes"`
	}
	if err := json.Unmarshal([]byte(item.JSONBlob), &payload); err != nil {
		return WorldModelSummary{}, fmt.Errorf("decode world model summary for %q: %w", item.ID, err)
	}

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

func normalizePayload(payload []byte) ([]byte, string, string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, "", "", ValidationError{Message: "request body must be a JSON object"}
	}

	orgRaw, ok := raw["organization"]
	if !ok || isNullJSON(orgRaw) {
		return nil, "", "", ValidationError{Message: "organization is required"}
	}
	var organization Organization
	if err := json.Unmarshal(orgRaw, &organization); err != nil {
		return nil, "", "", ValidationError{Message: "organization must be an object"}
	}
	if trimmed(organization.Name) == "" {
		return nil, "", "", ValidationError{Message: "organization.name is required"}
	}

	brandingRaw, ok := raw["branding"]
	if !ok || isNullJSON(brandingRaw) {
		return nil, "", "", ValidationError{Message: "branding is required"}
	}
	var branding map[string]any
	if err := json.Unmarshal(brandingRaw, &branding); err != nil {
		return nil, "", "", ValidationError{Message: "branding must be an object"}
	}

	if err := validateOptionalArray(raw, "departments", func(data []byte) error {
		var departments []Department
		if err := json.Unmarshal(data, &departments); err != nil {
			return ValidationError{Message: "departments must be an array"}
		}
		for _, department := range departments {
			if trimmed(department.Name) == "" {
				return ValidationError{Message: "departments[].name is required"}
			}
		}
		return nil
	}); err != nil {
		return nil, "", "", err
	}

	if err := validateOptionalArray(raw, "employees", func(data []byte) error {
		var employees []Employee
		if err := json.Unmarshal(data, &employees); err != nil {
			return ValidationError{Message: "employees must be an array"}
		}
		for _, employee := range employees {
			if trimmed(employee.FullName) == "" {
				return ValidationError{Message: "employees[].fullName is required"}
			}
			if trimmed(employee.Title) == "" {
				return ValidationError{Message: "employees[].title is required"}
			}
		}
		return nil
	}); err != nil {
		return nil, "", "", err
	}

	if err := validateOptionalArray(raw, "projects", func(data []byte) error {
		var projects []Project
		if err := json.Unmarshal(data, &projects); err != nil {
			return ValidationError{Message: "projects must be an array"}
		}
		for _, project := range projects {
			if trimmed(project.Name) == "" {
				return ValidationError{Message: "projects[].name is required"}
			}
		}
		return nil
	}); err != nil {
		return nil, "", "", err
	}

	if err := validateOptionalArray(raw, "documentThemes", func(data []byte) error {
		var documentThemes []DocumentTheme
		if err := json.Unmarshal(data, &documentThemes); err != nil {
			return ValidationError{Message: "documentThemes must be an array"}
		}
		for _, theme := range documentThemes {
			if trimmed(theme.Name) == "" {
				return ValidationError{Message: "documentThemes[].name is required"}
			}
		}
		return nil
	}); err != nil {
		return nil, "", "", err
	}

	normalizedPayload, err := json.Marshal(raw)
	if err != nil {
		return nil, "", "", fmt.Errorf("marshal normalized world model: %w", err)
	}

	return normalizedPayload, trimmed(organization.Name), trimmed(organization.Description), nil
}

func validateOptionalArray(raw map[string]json.RawMessage, key string, validate func([]byte) error) error {
	data, ok := raw[key]
	if !ok || isNullJSON(data) {
		raw[key] = json.RawMessage("[]")
		return nil
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
    "description": "Mid-sized US financial services firm serving regional businesses and high-net-worth clients.",
    "industry": "Financial services",
    "headquarters": "Chicago, IL, USA",
    "size": "Mid-sized"
  },
  "branding": {
    "tone": "Professional and reassuring",
    "voice": "Confident, precise, and client-focused",
    "primaryColor": "#123B5D",
    "secondaryColor": "#B58A3B",
    "keywords": ["trust", "compliance", "growth"]
  },
  "departments": [
    { "name": "Finance", "description": "Financial planning, reporting, and budgeting." },
    { "name": "Human Resources", "description": "Recruiting, onboarding, and people operations." },
    { "name": "Information Technology", "description": "Internal systems, endpoint support, and security coordination." },
    { "name": "Operations", "description": "Client onboarding, process execution, and vendor management." },
    { "name": "Compliance", "description": "Regulatory reporting, policy oversight, and audit readiness." }
  ],
  "employees": [
    { "fullName": "Lauren Chen", "title": "Managing Director", "department": "Finance", "email": "lauren.chen@northbridge.example" },
    { "fullName": "Marcus Bell", "title": "Controller", "department": "Finance", "email": "marcus.bell@northbridge.example" },
    { "fullName": "Priya Nair", "title": "HR Manager", "department": "Human Resources", "email": "priya.nair@northbridge.example" },
    { "fullName": "Dylan Brooks", "title": "IT Lead", "department": "Information Technology", "email": "dylan.brooks@northbridge.example" },
    { "fullName": "Avery Patel", "title": "Operations Manager", "department": "Operations", "email": "avery.patel@northbridge.example" },
    { "fullName": "Sofia Ramirez", "title": "Compliance Officer", "department": "Compliance", "email": "sofia.ramirez@northbridge.example" },
    { "fullName": "Ethan Cole", "title": "Financial Analyst", "department": "Finance", "email": "ethan.cole@northbridge.example" },
    { "fullName": "Grace Kim", "title": "Client Operations Specialist", "department": "Operations", "email": "grace.kim@northbridge.example" },
    { "fullName": "Noah Foster", "title": "Systems Administrator", "department": "Information Technology", "email": "noah.foster@northbridge.example" },
    { "fullName": "Maya Singh", "title": "Talent Coordinator", "department": "Human Resources", "email": "maya.singh@northbridge.example" }
  ],
  "projects": [
    { "name": "Quarterly Portfolio Review", "description": "Prepare advisory packets for top-tier clients.", "status": "active" },
    { "name": "SOX Control Refresh", "description": "Update internal control narratives and evidence collection.", "status": "active" },
    { "name": "Endpoint Upgrade Initiative", "description": "Refresh employee laptops and enforce new baseline policies.", "status": "planned" },
    { "name": "Benefits Renewal", "description": "Coordinate annual benefits review and employee communications.", "status": "planned" }
  ],
  "documentThemes": [
    { "name": "Finance", "description": "Budgets, forecasts, statements, and board-ready financial narratives." },
    { "name": "HR", "description": "Policies, onboarding guides, reviews, and employee communications." },
    { "name": "IT", "description": "Access procedures, system inventories, and security incident communications." },
    { "name": "Operations", "description": "Runbooks, vendor packets, and client onboarding documents." },
    { "name": "Compliance", "description": "Regulatory summaries, audit checklists, and policy attestations." }
  ]
}`
