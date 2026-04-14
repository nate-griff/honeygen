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

	// Fixed infrastructure categories that always appear.
	requiredCategories := map[string]bool{
		"policy-document":         false,
		"internal-memo":           false,
		"project-summary":         false,
		"vendor-contact-csv":      false,
		"faq-help-page":           false,
		"intranet-about-page":     false,
		"employee-roster-excerpt": false,
	}
	// Fixed path prefixes that always exist.
	requiredPrefixes := map[string]bool{
		"public/":                        false,
		"intranet/":                      false,
		"shared/finance/":                false,
		"shared/information-technology/": false,
		"users/lauren-chen/Projects/":    false,
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

	// Each employee should have at least 1 user-directory file plus the project summary.
	laurenCount := 0
	dylanCount := 0
	for _, entry := range first {
		if strings.HasPrefix(entry.Path, "users/lauren-chen/") {
			laurenCount++
		}
		if strings.HasPrefix(entry.Path, "users/dylan-brooks/") {
			dylanCount++
		}
	}
	if laurenCount < 2 {
		t.Fatalf("lauren-chen has %d files, want at least 2 (project summary + 1 template)", laurenCount)
	}
	if dylanCount < 2 {
		t.Fatalf("dylan-brooks has %d files, want at least 2 (project summary + 1 template)", dylanCount)
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
	if got, want := err.Error(), "world model must include at least one department, employee, and project"; got != want {
		t.Fatalf("Plan() error = %q, want %q", got, want)
	}
}

func TestPlannerAllowsWorldModelsWithoutDocumentThemes(t *testing.T) {
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
		Branding:    worldmodels.Branding{Tone: "formal"},
		Departments: []string{"Finance"},
		Employees: []worldmodels.Employee{
			{Name: "Lauren Chen", Role: "Managing Director", Department: "Finance"},
		},
		Projects: []string{"Quarterly Portfolio Review"},
	}

	manifest, err := planner.Plan("world-1", model)
	if err != nil {
		t.Fatalf("Plan() error = %v, want nil", err)
	}
	if len(manifest) == 0 {
		t.Fatal("Plan() returned no manifest entries")
	}
}
