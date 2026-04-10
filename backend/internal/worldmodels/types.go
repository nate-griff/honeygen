package worldmodels

import (
	"errors"
	"strings"
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
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Industry     string `json:"industry,omitempty"`
	Headquarters string `json:"headquarters,omitempty"`
	Size         string `json:"size,omitempty"`
}

type Branding struct {
	Tone           string   `json:"tone,omitempty"`
	Voice          string   `json:"voice,omitempty"`
	PrimaryColor   string   `json:"primaryColor,omitempty"`
	SecondaryColor string   `json:"secondaryColor,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
}

type Department struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Employee struct {
	FullName   string `json:"fullName"`
	Title      string `json:"title"`
	Department string `json:"department,omitempty"`
	Email      string `json:"email,omitempty"`
}

type Project struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

type DocumentTheme struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func trimmed(value string) string {
	return strings.TrimSpace(value)
}
