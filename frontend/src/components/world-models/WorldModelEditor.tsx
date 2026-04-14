import { useEffect, useMemo, useState } from "react";
import { ErrorAlert } from "../layout/ErrorAlert";
import type { WorldModelDetails, WorldModelPayload } from "../../types/worldModels";

interface WorldModelEditorProps {
  model: WorldModelDetails | null;
  initialPayload?: WorldModelPayload;
  isSubmitting: boolean;
  submitError?: string;
  submitSuccess?: string;
  onSubmit: (payload: WorldModelPayload) => Promise<void>;
}

interface WorldModelDraft {
  organizationName: string;
  organizationDescription: string;
  organizationIndustry: string;
  organizationSize: string;
  organizationRegion: string;
  organizationDomainTheme: string;
  brandingTone: string;
  brandingColors: string;
  departments: string;
  employees: string;
  projects: string;
  documentThemes: string;
  fileCountTarget: string;
  fileCountVariance: string;
}

function createDraft(source: WorldModelPayload | null): WorldModelDraft {
  return {
    organizationName: source?.organization.name ?? "",
    organizationDescription: source?.organization.description ?? "",
    organizationIndustry: source?.organization.industry ?? "",
    organizationSize: source?.organization.size ?? "",
    organizationRegion: source?.organization.region ?? "",
    organizationDomainTheme: source?.organization.domain_theme ?? "",
    brandingTone: source?.branding.tone ?? "",
    brandingColors: (source?.branding.colors ?? []).join(", "),
    departments: (source?.departments ?? []).join("\n"),
    employees: (source?.employees ?? []).map((employee) => `${employee.name} | ${employee.role} | ${employee.department}`).join("\n"),
    projects: (source?.projects ?? []).join("\n"),
    documentThemes: (source?.document_themes ?? []).join("\n"),
    fileCountTarget: String(source?.generation_settings?.file_count_target ?? 5),
    fileCountVariance: String(source?.generation_settings?.file_count_variance ?? 2),
  };
}

function linesToList(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function employeesToList(value: string): WorldModelPayload["employees"] {
  return value
    .split(/\r?\n/)
    .map((line) => line.split("|").map((part) => part.trim()))
    .filter((parts) => parts.some(Boolean))
    .map(([name = "", role = "", department = ""]) => ({
      name,
      role,
      department,
    }));
}

export function WorldModelEditor({
  model,
  initialPayload,
  isSubmitting,
  submitError,
  submitSuccess,
  onSubmit,
}: WorldModelEditorProps) {
  const source = model ?? initialPayload ?? null;
  const [draft, setDraft] = useState<WorldModelDraft>(() => createDraft(source));

  useEffect(() => {
    setDraft(createDraft(source));
  }, [model, initialPayload]);

  const modeLabel = model ? "Update world model" : "Create world model";
  const helperText = useMemo(
    () => (model ? "Edit the selected model and save changes to the live API." : "Create a new world model in the live API."),
    [model],
  );

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onSubmit({
      organization: {
        name: draft.organizationName.trim(),
        description: draft.organizationDescription.trim(),
        industry: draft.organizationIndustry.trim(),
        size: draft.organizationSize.trim(),
        region: draft.organizationRegion.trim(),
        domain_theme: draft.organizationDomainTheme.trim(),
      },
      branding: {
        tone: draft.brandingTone.trim(),
        colors: draft.brandingColors
          .split(",")
          .map((color) => color.trim())
          .filter(Boolean),
      },
      departments: linesToList(draft.departments),
      employees: employeesToList(draft.employees),
      projects: linesToList(draft.projects),
      document_themes: linesToList(draft.documentThemes),
      generation_settings: {
        file_count_target: parseInt(draft.fileCountTarget, 10) || 5,
        file_count_variance: parseInt(draft.fileCountVariance, 10) || 2,
      },
    });
  }

  return (
    <form className="world-model-editor" onSubmit={handleSubmit}>
      <div className="stack stack--compact">
        <h3>{modeLabel}</h3>
        <p className="muted">{helperText}</p>
        {submitError ? <ErrorAlert message={submitError} /> : null}
        {submitSuccess ? (
          <div aria-atomic="true" aria-live="polite" className="success-alert" role="status">
            {submitSuccess}
          </div>
        ) : null}
      </div>
      <div className="form-grid">
        <label>
          Organization name
          <input
            placeholder="Northbridge Financial Advisory"
            value={draft.organizationName}
            onChange={(event) => setDraft((current) => ({ ...current, organizationName: event.target.value }))}
          />
        </label>
        <label>
          Industry
          <input
            placeholder="Financial Services"
            value={draft.organizationIndustry}
            onChange={(event) => setDraft((current) => ({ ...current, organizationIndustry: event.target.value }))}
          />
        </label>
        <label>
          Size
          <input
            placeholder="mid-size"
            value={draft.organizationSize}
            onChange={(event) => setDraft((current) => ({ ...current, organizationSize: event.target.value }))}
          />
        </label>
        <label>
          Region
          <input
            placeholder="United States"
            value={draft.organizationRegion}
            onChange={(event) => setDraft((current) => ({ ...current, organizationRegion: event.target.value }))}
          />
        </label>
        <label>
          Domain theme
          <input
            placeholder="northbridgefinancial.local"
            value={draft.organizationDomainTheme}
            onChange={(event) => setDraft((current) => ({ ...current, organizationDomainTheme: event.target.value }))}
          />
        </label>
        <label>
          Brand tone
          <input
            placeholder="formal"
            value={draft.brandingTone}
            onChange={(event) => setDraft((current) => ({ ...current, brandingTone: event.target.value }))}
          />
        </label>
        <label className="form-grid__full">
          Description
          <textarea
            rows={3}
            placeholder="A mid-size financial advisory firm specializing in portfolio management and regulatory compliance."
            value={draft.organizationDescription}
            onChange={(event) => setDraft((current) => ({ ...current, organizationDescription: event.target.value }))}
          />
        </label>
        <label className="form-grid__full">
          Brand colors
          <input
            placeholder="navy, teal, slate"
            value={draft.brandingColors}
            onChange={(event) => setDraft((current) => ({ ...current, brandingColors: event.target.value }))}
          />
        </label>
        <label>
          Departments
          <textarea
            rows={6}
            value={draft.departments}
            onChange={(event) => setDraft((current) => ({ ...current, departments: event.target.value }))}
          />
          <span className="field-hint">One department per line.</span>
        </label>
        <label>
          Projects
          <textarea
            rows={6}
            value={draft.projects}
            onChange={(event) => setDraft((current) => ({ ...current, projects: event.target.value }))}
          />
          <span className="field-hint">One project per line.</span>
        </label>
        <label>
          Document themes
          <textarea
            rows={6}
            value={draft.documentThemes}
            onChange={(event) => setDraft((current) => ({ ...current, documentThemes: event.target.value }))}
          />
          <span className="field-hint">One theme per line.</span>
        </label>
        <label className="form-grid__full">
          Employees
          <textarea
            rows={7}
            value={draft.employees}
            onChange={(event) => setDraft((current) => ({ ...current, employees: event.target.value }))}
          />
          <span className="field-hint">Use “Name | Role | Department” on each line.</span>
        </label>
      </div>
      <div className="stack stack--compact" style={{ marginTop: "1.5rem" }}>
        <h4>Generation settings</h4>
        <p className="muted">Controls the number and variety of files generated per employee.</p>
      </div>
      <div className="form-grid">
        <label>
          Files per employee
          <input
            type="number"
            min="1"
            max="13"
            placeholder="5"
            value={draft.fileCountTarget}
            onChange={(event) => setDraft((current) => ({ ...current, fileCountTarget: event.target.value }))}
          />
          <span className="field-hint">Target number of files per employee.</span>
        </label>
        <label>
          Variance
          <input
            type="number"
            min="0"
            max="6"
            placeholder="2"
            value={draft.fileCountVariance}
            onChange={(event) => setDraft((current) => ({ ...current, fileCountVariance: event.target.value }))}
          />
          <span className="field-hint">±variance randomizes file count per employee.</span>
        </label>
      </div>
      <div className="button-row">
        <button className="button button--primary" disabled={isSubmitting} type="submit">
          {isSubmitting ? "Saving…" : modeLabel}
        </button>
        <button className="button button--ghost" disabled={isSubmitting} onClick={() => setDraft(createDraft(model))} type="button">
          Reset form
        </button>
      </div>
    </form>
  );
}
