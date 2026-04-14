package generation

import (
	"fmt"
	"hash/fnv"
	"math/rand"
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

// Document template pools for employee file variation.
var userDocTemplates = []struct {
	folder       string
	filename     string
	category     string
	renderedType string
	titleSuffix  string
	promptHint   string
}{
	{"Documents", "meeting-notes.txt", "meeting-notes", "text", "Meeting Notes", "Create concise internal meeting notes for the employee's current work."},
	{"Documents", "quarterly-review.md", "quarterly-review", "markdown", "Quarterly Review Notes", "Create quarterly performance or project review notes."},
	{"Documents", "expense-report.csv", "expense-report", "csv", "Expense Report", "Return CSV with headers date,description,category,amount for realistic expense entries."},
	{"Documents", "expense-report.xlsx", "expense-report-xlsx", "xlsx", "Expense Report", "Return CSV with headers date,description,category,amount for realistic expense entries."},
	{"Documents", "training-notes.txt", "training-notes", "text", "Training Notes", "Create notes from a recent internal training session."},
	{"Documents", "onboarding-checklist.md", "onboarding-checklist", "markdown", "Onboarding Checklist", "Create a new-hire onboarding checklist with common tasks."},
	{"Documents", "policy-memo.docx", "policy-memo-docx", "docx", "Policy Memo", "Create a short internal policy memo about a workplace procedure."},
	{"Desktop", "help.txt", "desktop-reference", "text", "Desktop Reference", "Create a short desktop quick-reference note with recurring tasks."},
	{"Desktop", "todo-list.txt", "desktop-todo", "text", "To-Do List", "Create a short personal work to-do list."},
	{"Desktop", "bookmarks.html", "browser-bookmarks", "html", "Bookmarks", "Create a simple HTML bookmarks page with realistic internal and external links."},
	{"Desktop", "scratch-notes.txt", "scratch-notes", "text", "Scratch Notes", "Create informal scratch notes with miscellaneous work reminders."},
	{"Downloads", "readme.txt", "downloaded-readme", "text", "Downloaded README", "Create a readme for a downloaded software tool or vendor package."},
	{"Notes", "weekly-standup.md", "standup-notes", "markdown", "Weekly Standup Notes", "Create brief weekly standup notes summarizing progress and blockers."},
	{"Notes", "brainstorm.txt", "brainstorm-notes", "text", "Brainstorm Notes", "Create informal brainstorming notes for an upcoming initiative."},
	{"Archive", "old-project-recap.md", "project-recap", "markdown", "Project Recap", "Create a short retrospective summary of a completed project."},
	{"Shared", "team-contacts.xlsx", "team-contacts-xlsx", "xlsx", "Team Contacts", "Return CSV with headers name,email,phone,role for team contact entries."},
}

// Style hints for writing variation across employees.
var styleHints = []string{
	"Write in a casual, abbreviated note-taking style with bullet points.",
	"Write in a formal, detailed memo style with complete sentences.",
	"Write in a minimal, sparse style with short phrases and keywords.",
	"Write in a conversational, friendly style as if written quickly.",
	"Write in a structured, organized style with clear section headers.",
}

func (p *Planner) Plan(worldModelID string, model worldmodels.WorldModel) ([]ManifestEntry, error) {
	if strings.TrimSpace(worldModelID) == "" {
		return nil, fmt.Errorf("world model id is required")
	}
	if len(model.Departments) == 0 || len(model.Employees) == 0 || len(model.Projects) == 0 {
		return nil, fmt.Errorf("world model must include at least one department, employee, and project")
	}

	departments := append([]string(nil), model.Departments...)
	sort.Strings(departments)

	employees := append([]worldmodels.Employee(nil), model.Employees...)
	sort.Slice(employees, func(i, j int) bool {
		return strings.ToLower(employees[i].Name) < strings.ToLower(employees[j].Name)
	})

	projects := append([]string(nil), model.Projects...)
	sort.Strings(projects)

	settings := model.GenerationSettings
	if settings.FileCountTarget <= 0 {
		settings.FileCountTarget = 5
	}
	if settings.FileCountVariance < 0 {
		settings.FileCountVariance = 0
	}

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
			Path:         "intranet/vendors/vendor-contacts.xlsx",
			Category:     "vendor-contact-xlsx",
			Title:        model.Organization.Name + " Vendor Contacts",
			RenderedType: "xlsx",
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
		{
			Path:         "intranet/roster/employee-roster-excerpt.xlsx",
			Category:     "employee-roster-xlsx",
			Title:        model.Organization.Name + " Employee Roster Excerpt",
			RenderedType: "xlsx",
			SourceType:   "provider",
			PromptHint:   "Return CSV with headers name,role,department,email for an employee roster excerpt.",
			Tags:         []string{"audience:internal"},
		},
		{
			Path:         "intranet/policies/acceptable-use-policy.docx",
			Category:     "policy-document-docx",
			Title:        model.Organization.Name + " Acceptable Use Policy",
			RenderedType: "docx",
			SourceType:   "provider",
			PromptHint:   "Create a practical acceptable use policy for employees.",
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

	// Seed RNG per world model for reproducible but varied output.
	rng := rand.New(rand.NewSource(seedFromString(worldModelID)))

	for index, employee := range employees {
		employeeSlug := slugify(employee.Name)
		project := "Operations Overview"
		if len(projects) > 0 {
			project = projects[index%len(projects)]
		}

		// Always include a project summary per employee.
		entries = append(entries, ManifestEntry{
			Path:         "users/" + employeeSlug + "/Projects/" + slugify(project) + "-summary.md",
			Category:     "project-summary",
			Title:        project + " Summary",
			RenderedType: "markdown",
			SourceType:   "provider",
			PromptHint:   "Create a project summary with objectives, current status, and next steps.",
			Tags:         []string{"employee:" + employeeSlug, "project:" + slugify(project)},
		})

		// Determine file count for this employee using verbosity + variance.
		fileCount := settings.FileCountTarget
		if settings.FileCountVariance > 0 {
			fileCount += rng.Intn(settings.FileCountVariance*2+1) - settings.FileCountVariance
		}
		if fileCount < 1 {
			fileCount = 1
		}

		// Pick a random style hint for this employee.
		employeeStyle := styleHints[rng.Intn(len(styleHints))]

		// Shuffle template pool and select fileCount documents.
		pool := make([]int, len(userDocTemplates))
		for i := range pool {
			pool[i] = i
		}
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

		count := fileCount
		if count > len(pool) {
			count = len(pool)
		}
		for _, templateIdx := range pool[:count] {
			tmpl := userDocTemplates[templateIdx]
			entries = append(entries, ManifestEntry{
				Path:         "users/" + employeeSlug + "/" + tmpl.folder + "/" + tmpl.filename,
				Category:     tmpl.category,
				Title:        employee.Name + " " + tmpl.titleSuffix,
				RenderedType: tmpl.renderedType,
				SourceType:   "provider",
				PromptHint:   tmpl.promptHint + " " + employeeStyle,
				Tags:         []string{"employee:" + employeeSlug, "department:" + slugify(employee.Department)},
			})
		}
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

// seedFromString produces a deterministic int64 seed from a string.
func seedFromString(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}
