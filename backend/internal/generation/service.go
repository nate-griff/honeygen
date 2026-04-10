package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
}

type ServiceConfig struct {
	WorldModels worldModelReader
	Planner     *Planner
	Provider    provider.Provider
	Jobs        *JobStore
	Assets      assetWriter
	Storage     fileStore
	Renderers   rendering.Registry
}

type Service struct {
	worldModels worldModelReader
	planner     *Planner
	provider    provider.Provider
	jobs        *JobStore
	assets      assetWriter
	storage     fileStore
	renderers   rendering.Registry
}

func NewService(config ServiceConfig) *Service {
	planner := config.Planner
	if planner == nil {
		planner = NewPlanner()
	}

	return &Service{
		worldModels: config.WorldModels,
		planner:     planner,
		provider:    config.Provider,
		jobs:        config.Jobs,
		assets:      config.Assets,
		storage:     config.Storage,
		renderers:   config.Renderers,
	}
}

func (s *Service) Run(ctx context.Context, request RunRequest) (Job, error) {
	if strings.TrimSpace(request.WorldModelID) == "" {
		return Job{}, fmt.Errorf("world_model_id is required")
	}
	if s.provider == nil {
		return Job{}, &provider.Error{Kind: provider.KindConfig, Message: "provider is not configured"}
	}

	summary := Summary{}
	appendLog := func(level, message, path, category string) {
		summary.Logs = append(summary.Logs, LogEntry{
			Time:     time.Now().UTC(),
			Level:    level,
			Message:  message,
			Path:     path,
			Category: category,
		})
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

	for _, entry := range manifest {
		appendLog("info", "generating asset", entry.Path, entry.Category)
		job, err = s.jobs.UpdateSummary(ctx, job.ID, summary)
		if err != nil {
			return Job{}, err
		}

		response, err := s.provider.Generate(ctx, provider.GenerateRequest{
			SystemPrompt: "Generate realistic enterprise decoy content. Follow the requested format exactly and keep the content coherent with the supplied world model.",
			Prompt:       buildPrompt(model, entry),
			Metadata: map[string]string{
				"world_model_id": request.WorldModelID,
				"path":           entry.Path,
				"category":       entry.Category,
				"rendered_type":  entry.RenderedType,
				"title":          entry.Title,
			},
		})
		if err != nil {
			message := provider.SafeErrorMessage(err)
			appendLog("error", "provider generation failed: "+message, entry.Path, entry.Category)
			job, _ = s.jobs.SetFailed(ctx, job.ID, summary, message)
			return job, err
		}

		output, err := s.renderers.Render(ctx, entry.RenderedType, rendering.Document{
			Title: entry.Title,
			Body:  response.Content,
			Metadata: map[string]string{
				"category": entry.Category,
				"path":     entry.Path,
			},
		})
		if err != nil {
			appendLog("error", "rendering failed: "+err.Error(), entry.Path, entry.Category)
			job, _ = s.jobs.SetFailed(ctx, job.ID, summary, err.Error())
			return job, err
		}

		storedPath, err := storage.JoinRelative("generated", request.WorldModelID, job.ID, entry.Path)
		if err != nil {
			appendLog("error", "storage path invalid: "+err.Error(), entry.Path, entry.Category)
			job, _ = s.jobs.SetFailed(ctx, job.ID, summary, err.Error())
			return job, err
		}

		storedFile, err := s.storage.Write(ctx, storedPath, output.Bytes)
		if err != nil {
			appendLog("error", "storage write failed: "+err.Error(), entry.Path, entry.Category)
			job, _ = s.jobs.SetFailed(ctx, job.ID, summary, err.Error())
			return job, err
		}

		if _, err := s.assets.Create(ctx, assets.Asset{
			ID:              newAssetID(),
			GenerationJobID: job.ID,
			WorldModelID:    request.WorldModelID,
			SourceType:      entry.SourceType,
			RenderedType:    entry.RenderedType,
			Path:            storedFile.Path,
			MIMEType:        output.MIMEType,
			SizeBytes:       storedFile.SizeBytes,
			Tags:            append(append([]string{}, entry.Tags...), "category:"+entry.Category),
			Previewable:     output.Previewable,
			Checksum:        storedFile.Checksum,
		}); err != nil {
			appendLog("error", "asset persistence failed: "+err.Error(), entry.Path, entry.Category)
			job, _ = s.jobs.SetFailed(ctx, job.ID, summary, err.Error())
			return job, err
		}

		summary.AssetCount++
		appendLog("info", "asset stored", entry.Path, entry.Category)
	}

	job, err = s.jobs.SetCompleted(ctx, job.ID, summary)
	if err != nil {
		return Job{}, err
	}
	return job, nil
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
	builder.WriteString("\nKeep the content plausible, businesslike, and finite.")
	return builder.String()
}
