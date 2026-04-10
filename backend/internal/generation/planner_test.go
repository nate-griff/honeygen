package generation

import (
	"reflect"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/worldmodels"
)

func TestPlannerBuildsDeterministicManifestWithRequiredCoverage(t *testing.T) {
	t.Parallel()

	planner := NewPlanner()
	model := worldmodels.WorldModel{
		Organization: worldmodels.Organization{
			Name:        "Northbridge Financial Advisory",
			Industry:    "Financial Services",
			Size:        "mid-size",
			Region:      "United States",
			DomainTheme: "northbridgefinancial.local",
		},
		Branding: worldmodels.Branding{
			Tone:   "formal",
			Colors: []string{"#123B5D", "#B58A3B"},
		},
		Departments: []string{"Finance", "Information Technology", "Operations"},
		Employees: []worldmodels.Employee{
			{Name: "Lauren Chen", Role: "Managing Director", Department: "Finance"},
			{Name: "Dylan Brooks", Role: "IT Lead", Department: "Information Technology"},
		},
		Projects:       []string{"Quarterly Portfolio Review", "Endpoint Upgrade Initiative"},
		DocumentThemes: []string{"policies", "meeting notes", "vendor lists"},
	}

	first, err := planner.Plan("world-1", model)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	second, err := planner.Plan("world-1", model)
	if err != nil {
		t.Fatalf("Plan() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Plan() is not deterministic")
	}
	if len(first) < 10 {
		t.Fatalf("len(manifest) = %d, want at least 10 entries", len(first))
	}

	requiredCategories := map[string]bool{
		"policy-document":         false,
		"internal-memo":           false,
		"meeting-notes":           false,
		"project-summary":         false,
		"vendor-contact-csv":      false,
		"faq-help-page":           false,
		"intranet-about-page":     false,
		"employee-roster-excerpt": false,
	}
	requiredPrefixes := map[string]bool{
		"public/":                        false,
		"intranet/":                      false,
		"shared/finance/":                false,
		"shared/information-technology/": false,
		"users/lauren-chen/Documents/":   false,
		"users/lauren-chen/Desktop/":     false,
		"users/lauren-chen/Projects/":    false,
		"users/dylan-brooks/Documents/":  false,
		"users/dylan-brooks/Desktop/":    false,
		"users/dylan-brooks/Projects/":   false,
	}

	for _, entry := range first {
		if _, ok := requiredCategories[entry.Category]; ok {
			requiredCategories[entry.Category] = true
		}
		for prefix := range requiredPrefixes {
			if strings.HasPrefix(entry.Path, prefix) {
				requiredPrefixes[prefix] = true
			}
		}
	}

	for category, seen := range requiredCategories {
		if !seen {
			t.Fatalf("manifest missing category %q", category)
		}
	}
	for prefix, seen := range requiredPrefixes {
		if !seen {
			t.Fatalf("manifest missing path prefix %q", prefix)
		}
	}
}

func TestPlannerRejectsWorldModelsWithoutCoverageInputs(t *testing.T) {
	t.Parallel()

	planner := NewPlanner()
	model := worldmodels.WorldModel{
		Organization: worldmodels.Organization{
			Name:        "Northbridge Financial Advisory",
			Industry:    "Financial Services",
			Size:        "mid-size",
			Region:      "United States",
			DomainTheme: "northbridgefinancial.local",
		},
		Branding: worldmodels.Branding{Tone: "formal"},
	}

	_, err := planner.Plan("world-1", model)
	if err == nil {
		t.Fatal("Plan() error = nil, want validation error")
	}
	if got, want := err.Error(), "world model must include at least one department, employee, project, and document theme"; got != want {
		t.Fatalf("Plan() error = %q, want %q", got, want)
	}
}
