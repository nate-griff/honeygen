package worldmodels

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("world model not found")
	ErrAlreadyExists = errors.New("world model already exists")
)

const DemoWorldModelID = "northbridge-financial"

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type StoredWorldModel struct {
	ID          string
	Name        string
	Description string
	JSONBlob    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorldModelSummary struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	DepartmentCount    int       `json:"departmentCount"`
	EmployeeCount      int       `json:"employeeCount"`
	ProjectCount       int       `json:"projectCount"`
	DocumentThemeCount int       `json:"documentThemeCount"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type Organization struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Industry    string `json:"industry"`
	Size        string `json:"size"`
	Region      string `json:"region"`
	DomainTheme string `json:"domain_theme"`
}

type Branding struct {
	Tone   string   `json:"tone"`
	Colors []string `json:"colors"`
}

type Employee struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Department string `json:"department"`
}

type WorldModel struct {
	Organization   Organization `json:"organization"`
	Branding       Branding     `json:"branding"`
	Departments    []string     `json:"departments"`
	Employees      []Employee   `json:"employees"`
	Projects       []string     `json:"projects"`
	DocumentThemes []string     `json:"document_themes"`
}
