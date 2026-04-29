package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/provider"
	"github.com/natet/honeygen/backend/internal/rendering"
	"github.com/natet/honeygen/backend/internal/storage"
	"github.com/natet/honeygen/backend/internal/worldmodels"
)

type RunRequest struct {
	WorldModelID string `json:"world_model_id"`
}

type worldModelReader interface {
	Get(context.Context, string) (worldmodels.StoredWorldModel, error)
}

type assetWriter interface {
	Create(context.Context, assets.Asset) (assets.Asset, error)
}

type fileStore interface {
	Write(context.Context, string, []byte) (storage.StoredFile, error)
	Read(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type ServiceConfig struct {
	WorldModels       worldModelReader
	Planner           *Planner
	Provider          provider.Provider
	ProviderProvider  func() provider.Provider
	Jobs              *JobStore
	Assets            assetWriter
	Storage           fileStore
	Renderers         rendering.Registry
	RenderersProvider func() rendering.Registry
	LifecycleContext  context.Context
}

type Service struct {
	worldModels worldModelReader
	planner     *Planner
	provider    func() provider.Provider
	jobs        *JobStore
	assets      assetWriter
	storage     fileStore
	renderers   func() rendering.Registry
	lifecycle   context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	jobCancels  map[string]context.CancelFunc
	wg          sync.WaitGroup
}

func NewService(config ServiceConfig) *Service {
	planner := config.Planner
	if planner == nil {
		planner = NewPlanner()
	}
	providerFunc := config.ProviderProvider
	if providerFunc == nil {
		staticProvider := config.Provider
		providerFunc = func() provider.Provider {
			return staticProvider
		}
	}
	renderersFunc := config.RenderersProvider
	if renderersFunc == nil {
		staticRenderers := config.Renderers
		renderersFunc = func() rendering.Registry {
			return staticRenderers
		}
	}
	lifecycleContext := config.LifecycleContext
	if lifecycleContext == nil {
		lifecycleContext = context.Background()
	}
	serviceContext, cancel := context.WithCancel(lifecycleContext)

	return &Service{
		worldModels: config.WorldModels,
		planner:     planner,
		provider:    providerFunc,
		jobs:        config.Jobs,
		assets:      config.Assets,
		storage:     config.Storage,
		renderers:   renderersFunc,
		lifecycle:   serviceContext,
		cancel:      cancel,
		jobCancels:  map[string]context.CancelFunc{},
	}
}

func (s *Service) Run(ctx context.Context, request RunRequest) (Job, error) {
	if strings.TrimSpace(request.WorldModelID) == "" {
		return Job{}, fmt.Errorf("world_model_id is required")
	}
	jobProvider := s.provider()
	if jobProvider == nil {
		return Job{}, &provider.Error{Kind: provider.KindConfig, Message: "provider is not configured"}
	}
	jobRenderers := s.renderers()

	summary := Summary{}
	appendLog := func(level, message, path, category string) {
		appendSummaryLog(&summary, level, message, path, category)
	}

	worldModel, err := s.worldModels.Get(ctx, request.WorldModelID)
	if err != nil {
		return Job{}, err
	}

	model, err := decodeWorldModel(worldModel.JSONBlob)
	if err != nil {
		return Job{}, err
	}

	manifest, err := s.planner.Plan(request.WorldModelID, model)
	if err != nil {
		return Job{}, err
	}

	job, err := s.jobs.Create(ctx, request.WorldModelID)
	if err != nil {
		return Job{}, err
	}
	appendLog("info", "generation job created", "", "")
	summary.ManifestCount = len(manifest)
	summary.Categories = uniqueCategories(manifest)
	appendLog("info", fmt.Sprintf("planned %d assets", len(manifest)), "", "")

	job, err = s.jobs.SetRunning(ctx, job.ID, summary)
	if err != nil {
		return Job{}, err
	}

	jobCtx, cancel := context.WithCancel(s.lifecycle)
	s.trackJob(job.ID, cancel)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.untrackJob(job.ID)
		s.runJob(jobCtx, jobProvider, jobRenderers, job.ID, request.WorldModelID, model, manifest, summary)
	}()

	return job, nil
}

func (s *Service) Cancel(ctx context.Context, jobID string) (Job, error) {
	return s.cancelJob(ctx, jobID, "generation canceled")
}

func (s *Service) Close() error {
	s.cancel()

	s.mu.Lock()
	jobIDs := make([]string, 0, len(s.jobCancels))
	for jobID := range s.jobCancels {
		jobIDs = append(jobIDs, jobID)
	}
	s.mu.Unlock()

	for _, jobID := range jobIDs {
		if _, err := s.cancelJob(s.finalizeContext(), jobID, "generation canceled during shutdown"); err != nil && !errors.Is(err, ErrJobNotFound) && !errors.Is(err, ErrJobNotCancelable) {
			return err
		}
	}

	s.wg.Wait()
	return nil
}

func (s *Service) runJob(ctx context.Context, jobProvider provider.Provider, jobRenderers rendering.Registry, jobID, worldModelID string, model worldmodels.WorldModel, manifest []ManifestEntry, summary Summary) {
	appendLog := func(level, message, path, category string) {
		appendSummaryLog(&summary, level, message, path, category)
	}

	for _, entry := range manifest {
		if s.isCanceled(ctx) {
			_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
			return
		}

		appendLog("info", "generating asset", entry.Path, entry.Category)
		if _, err := s.jobs.UpdateSummary(ctx, jobID, summary); err != nil {
			return
		}

		response, err := jobProvider.Generate(ctx, provider.GenerateRequest{
			SystemPrompt: "Generate realistic enterprise decoy content. Follow the requested format exactly and keep the content coherent with the supplied world model.",
			Prompt:       buildPrompt(model, entry),
			Metadata: map[string]string{
				"world_model_id": worldModelID,
				"path":           entry.Path,
				"category":       entry.Category,
				"rendered_type":  entry.RenderedType,
				"title":          entry.Title,
			},
		})
		if err != nil {
			if s.isCanceled(ctx) {
				_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
				return
			}
			message := provider.SafeErrorMessage(err)
			appendLog("error", "provider generation failed: "+message, entry.Path, entry.Category)
			_, _ = s.jobs.SetFailed(s.finalizeContext(), jobID, summary, message)
			return
		}

		content := cleanProviderResponse(response.Content, entry.RenderedType)

		output, err := jobRenderers.Render(ctx, entry.RenderedType, rendering.Document{
			Title: entry.Title,
			Body:  content,
			Metadata: map[string]string{
				"category": entry.Category,
				"path":     entry.Path,
			},
		})
		if err != nil {
			if s.isCanceled(ctx) {
				_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
				return
			}
			appendLog("error", "rendering failed: "+err.Error(), entry.Path, entry.Category)
			_, _ = s.jobs.SetFailed(s.finalizeContext(), jobID, summary, err.Error())
			return
		}

		storedPath, err := storage.JoinRelative("generated", worldModelID, jobID, entry.Path)
		if err != nil {
			appendLog("error", "storage path invalid: "+err.Error(), entry.Path, entry.Category)
			_, _ = s.jobs.SetFailed(s.finalizeContext(), jobID, summary, err.Error())
			return
		}

		storedFile, err := s.storage.Write(ctx, storedPath, output.Bytes)
		if err != nil {
			if s.isCanceled(ctx) {
				_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
				return
			}
			appendLog("error", "storage write failed: "+err.Error(), entry.Path, entry.Category)
			_, _ = s.jobs.SetFailed(s.finalizeContext(), jobID, summary, err.Error())
			return
		}
		persistenceCtx := s.finalizeContext()
		if _, err := s.assets.Create(persistenceCtx, assets.Asset{
			ID:              newAssetID(),
			GenerationJobID: jobID,
			WorldModelID:    worldModelID,
			SourceType:      entry.SourceType,
			RenderedType:    entry.RenderedType,
			Path:            storedFile.Path,
			MIMEType:        output.MIMEType,
			SizeBytes:       storedFile.SizeBytes,
			Tags:            append(append([]string{}, entry.Tags...), "category:"+entry.Category),
			Previewable:     output.Previewable,
			Checksum:        storedFile.Checksum,
		}); err != nil {
			s.cleanupStoredFile(s.finalizeContext(), storedFile.Path)
			if s.isCanceled(ctx) {
				_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
				return
			}
			appendLog("error", "asset persistence failed: "+err.Error(), entry.Path, entry.Category)
			_, _ = s.jobs.SetFailed(s.finalizeContext(), jobID, summary, err.Error())
			return
		}

		summary.AssetCount++
		appendLog("info", "asset stored", entry.Path, entry.Category)
		if _, err := s.jobs.UpdateSummary(persistenceCtx, jobID, summary); err != nil {
			return
		}
		if s.isCanceled(ctx) {
			_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
			return
		}
	}

	if s.isCanceled(ctx) {
		_, _ = s.jobs.SetCanceled(s.finalizeContext(), jobID, summary, "generation canceled")
		return
	}

	_, _ = s.jobs.SetCompleted(s.finalizeContext(), jobID, summary)
}

func (s *Service) cancelJob(ctx context.Context, jobID, message string) (Job, error) {
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if !CanCancel(job.Status) {
		return job, ErrJobNotCancelable
	}

	appendSummaryLog(&job.Summary, "warn", message, "", "")
	job, err = s.jobs.SetCanceled(s.finalizeContext(), jobID, job.Summary, message)
	if err != nil {
		return Job{}, err
	}

	if cancel := s.jobCancel(jobID); cancel != nil {
		cancel()
	}

	return job, nil
}

func (s *Service) trackJob(jobID string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobCancels[jobID] = cancel
}

func (s *Service) untrackJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobCancels, jobID)
}

func (s *Service) jobCancel(jobID string) context.CancelFunc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobCancels[jobID]
}

func (s *Service) finalizeContext() context.Context {
	return context.Background()
}

func (s *Service) isCanceled(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}

func (s *Service) cleanupStoredFile(ctx context.Context, relativePath string) {
	if strings.TrimSpace(relativePath) == "" {
		return
	}
	_ = s.storage.Delete(ctx, relativePath)
}

func appendSummaryLog(summary *Summary, level, message, path, category string) {
	summary.Logs = append(summary.Logs, LogEntry{
		Time:     time.Now().UTC(),
		Level:    level,
		Message:  message,
		Path:     path,
		Category: category,
	})
}

func decodeWorldModel(jsonBlob string) (worldmodels.WorldModel, error) {
	var model worldmodels.WorldModel
	if err := json.Unmarshal([]byte(jsonBlob), &model); err != nil {
		return worldmodels.WorldModel{}, fmt.Errorf("decode world model json: %w", err)
	}
	return model, nil
}

func uniqueCategories(entries []ManifestEntry) []string {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		seen[entry.Category] = struct{}{}
	}
	items := make([]string, 0, len(seen))
	for category := range seen {
		items = append(items, category)
	}
	sort.Strings(items)
	return items
}

func buildPrompt(model worldmodels.WorldModel, entry ManifestEntry) string {
	var builder strings.Builder
	builder.WriteString("Organization: ")
	builder.WriteString(model.Organization.Name)
	builder.WriteString("\nIndustry: ")
	builder.WriteString(model.Organization.Industry)
	builder.WriteString("\nRegion: ")
	builder.WriteString(model.Organization.Region)
	builder.WriteString("\nTone: ")
	builder.WriteString(model.Branding.Tone)
	if len(model.Branding.Colors) > 0 {
		builder.WriteString("\nBrand colors: ")
		builder.WriteString(strings.Join(model.Branding.Colors, ", "))
		builder.WriteString("\nUse these exact brand colors in any HTML/CSS styling. Do not invent different color values.")
	}
	builder.WriteString("\nRequested asset: ")
	builder.WriteString(entry.Title)
	builder.WriteString("\nCategory: ")
	builder.WriteString(entry.Category)
	builder.WriteString("\nPath: ")
	builder.WriteString(entry.Path)
	builder.WriteString("\nFormat: ")
	builder.WriteString(entry.RenderedType)
	builder.WriteString("\nInstructions: ")
	builder.WriteString(entry.PromptHint)
	if len(model.Departments) > 0 {
		builder.WriteString("\nDepartments: ")
		builder.WriteString(strings.Join(model.Departments, ", "))
	}
	if len(model.Projects) > 0 {
		builder.WriteString("\nProjects: ")
		builder.WriteString(strings.Join(model.Projects, ", "))
	}
	switch entry.RenderedType {
	case "text", "docx":
		builder.WriteString("\nReturn plain text only. Do not include HTML, XML, Markdown formatting, or code fences.")
	case "csv", "xlsx":
		builder.WriteString("\nReturn raw CSV text only. Do not include Markdown, prose, or code fences.")
	case "markdown":
		builder.WriteString("\nReturn Markdown only. Do not include code fences.")
	case "html":
		builder.WriteString("\nReturn HTML only. Do not include Markdown fences or explanatory text.")
	}
	builder.WriteString("\nKeep the content plausible, businesslike, and finite.")
	return builder.String()
}

// cleanProviderResponse strips markdown code fences that LLMs often wrap
// around generated content (e.g. ```html ... ``` or ```csv ... ```).
func cleanProviderResponse(content, renderedType string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return trimmed
	}

	// Match opening fence: ```<optional language tag>
	// Common variants: ```html, ```markdown, ```csv, ```text, ```
	if idx := strings.Index(trimmed, "\n"); idx >= 0 {
		firstLine := strings.TrimSpace(trimmed[:idx])
		if strings.HasPrefix(firstLine, "```") {
			// Strip the opening fence line
			trimmed = strings.TrimSpace(trimmed[idx+1:])
		}
	} else if strings.HasPrefix(trimmed, "```") {
		// Single-line content that is just a fence — unlikely but handle it
		return ""
	}

	// Strip trailing fence: ```
	if strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimSpace(trimmed[:len(trimmed)-3])
	}

	if renderedType == "text" || renderedType == "docx" {
		trimmed = normalizePlainTextContent(trimmed)
	}

	return trimmed
}

var htmlTagPattern = regexp.MustCompile(`(?is)<[^>]+>`)

var htmlToTextReplacer = strings.NewReplacer(
	"<br>", "\n",
	"<br/>", "\n",
	"<br />", "\n",
	"</p>", "\n\n",
	"</div>", "\n",
	"</section>", "\n",
	"</article>", "\n",
	"</li>", "\n",
	"</ul>", "\n",
	"</ol>", "\n",
	"</h1>", "\n\n",
	"</h2>", "\n\n",
	"</h3>", "\n\n",
	"</h4>", "\n\n",
	"</h5>", "\n\n",
	"</h6>", "\n\n",
	"</tr>", "\n",
	"</td>", "\t",
	"</th>", "\t",
	"<li>", "- ",
)

func normalizePlainTextContent(content string) string {
	normalized := strings.TrimSpace(content)
	if looksLikeHTML(normalized) {
		normalized = htmlToTextReplacer.Replace(normalized)
		normalized = htmlTagPattern.ReplaceAllString(normalized, "")
		normalized = html.UnescapeString(normalized)
	}

	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	cleaned := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !lastBlank && len(cleaned) > 0 {
				cleaned = append(cleaned, "")
			}
			lastBlank = true
			continue
		}
		cleaned = append(cleaned, trimmed)
		lastBlank = false
	}
	if len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}

	return strings.Join(cleaned, "\n")
}

func looksLikeHTML(content string) bool {
	lowered := strings.ToLower(content)
	return strings.Contains(lowered, "<!doctype html") ||
		strings.Contains(lowered, "<html") ||
		strings.Contains(lowered, "<body") ||
		strings.Contains(lowered, "<p") ||
		strings.Contains(lowered, "<div") ||
		strings.Contains(lowered, "<h1") ||
		strings.Contains(lowered, "<section") ||
		strings.Contains(lowered, "<article")
}
