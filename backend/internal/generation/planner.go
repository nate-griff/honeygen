package generation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/natet/honeygen/backend/internal/worldmodels"
)

type ManifestEntry struct {
	Path         string
	Category     string
	Title        string
	RenderedType string
	SourceType   string
	PromptHint   string
	Tags         []string
}

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) Plan(worldModelID string, model worldmodels.WorldModel) ([]ManifestEntry, error) {
	if strings.TrimSpace(worldModelID) == "" {
		return nil, fmt.Errorf("world model id is required")
	}

	departments := append([]string(nil), model.Departments...)
	sort.Strings(departments)

	employees := append([]worldmodels.Employee(nil), model.Employees...)
	sort.Slice(employees, func(i, j int) bool {
		return strings.ToLower(employees[i].Name) < strings.ToLower(employees[j].Name)
	})

	projects := append([]string(nil), model.Projects...)
	sort.Strings(projects)

	entries := []ManifestEntry{
		{
			Path:         "public/about.html",
			Category:     "public-about-page",
			Title:        model.Organization.Name + " Overview",
			RenderedType: "html",
			SourceType:   "provider",
			PromptHint:   "Create a concise public-facing organization overview page.",
			Tags:         []string{"audience:public"},
		},
		{
			Path:         "public/faq.html",
			Category:     "faq-help-page",
			Title:        model.Organization.Name + " Frequently Asked Questions",
			RenderedType: "html",
			SourceType:   "provider",
			PromptHint:   "Create a concise FAQ/help page for external visitors.",
			Tags:         []string{"audience:public"},
		},
		{
			Path:         "intranet/about.html",
			Category:     "intranet-about-page",
			Title:        model.Organization.Name + " Intranet About",
			RenderedType: "html",
			SourceType:   "provider",
			PromptHint:   "Create an internal intranet about page with teams and operating context.",
			Tags:         []string{"audience:internal"},
		},
		{
			Path:         "intranet/policies/acceptable-use-policy.md",
			Category:     "policy-document",
			Title:        model.Organization.Name + " Acceptable Use Policy",
			RenderedType: "markdown",
			SourceType:   "provider",
			PromptHint:   "Create a practical acceptable use policy for employees.",
			Tags:         []string{"audience:internal"},
		},
		{
			Path:         "intranet/policies/acceptable-use-policy.pdf",
			Category:     "policy-document",
			Title:        model.Organization.Name + " Acceptable Use Policy",
			RenderedType: "pdf",
			SourceType:   "provider",
			PromptHint:   "Create an HTML policy document suitable for PDF export.",
			Tags:         []string{"audience:internal"},
		},
		{
			Path:         "intranet/vendors/vendor-contacts.csv",
			Category:     "vendor-contact-csv",
			Title:        model.Organization.Name + " Vendor Contacts",
			RenderedType: "csv",
			SourceType:   "provider",
			PromptHint:   "Return CSV with headers name,email,phone,service and realistic vendor contacts.",
			Tags:         []string{"audience:internal"},
		},
		{
			Path:         "intranet/roster/employee-roster-excerpt.csv",
			Category:     "employee-roster-excerpt",
			Title:        model.Organization.Name + " Employee Roster Excerpt",
			RenderedType: "csv",
			SourceType:   "provider",
			PromptHint:   "Return CSV with headers name,role,department,email for an employee roster excerpt.",
			Tags:         []string{"audience:internal"},
		},
	}

	for _, department := range departments {
		departmentSlug := slugify(department)
		entries = append(entries, ManifestEntry{
			Path:         "shared/" + departmentSlug + "/team-memo.md",
			Category:     "internal-memo",
			Title:        department + " Team Memo",
			RenderedType: "markdown",
			SourceType:   "provider",
			PromptHint:   "Create a short internal team memo with operational updates.",
			Tags:         []string{"department:" + departmentSlug},
		})
	}

	for index, employee := range employees {
		employeeSlug := slugify(employee.Name)
		project := "Operations Overview"
		if len(projects) > 0 {
			project = projects[index%len(projects)]
		}
		entries = append(entries,
			ManifestEntry{
				Path:         "users/" + employeeSlug + "/Documents/meeting-notes.txt",
				Category:     "meeting-notes",
				Title:        employee.Name + " Meeting Notes",
				RenderedType: "text",
				SourceType:   "provider",
				PromptHint:   "Create concise internal meeting notes for the employee's current work.",
				Tags:         []string{"employee:" + employeeSlug, "department:" + slugify(employee.Department)},
			},
			ManifestEntry{
				Path:         "users/" + employeeSlug + "/Desktop/help.txt",
				Category:     "desktop-reference",
				Title:        employee.Name + " Desktop Reference",
				RenderedType: "text",
				SourceType:   "provider",
				PromptHint:   "Create a short desktop quick-reference note with recurring tasks.",
				Tags:         []string{"employee:" + employeeSlug},
			},
			ManifestEntry{
				Path:         "users/" + employeeSlug + "/Projects/" + slugify(project) + "-summary.md",
				Category:     "project-summary",
				Title:        project + " Summary",
				RenderedType: "markdown",
				SourceType:   "provider",
				PromptHint:   "Create a project summary with objectives, current status, and next steps.",
				Tags:         []string{"employee:" + employeeSlug, "project:" + slugify(project)},
			},
		)
	}

	return entries, nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("&", " and ", "/", " ", "\\", " ", "_", " ", ".", " ", ",", " ", "'", "")
	value = replacer.Replace(value)
	fields := strings.Fields(value)
	return strings.Join(fields, "-")
}
