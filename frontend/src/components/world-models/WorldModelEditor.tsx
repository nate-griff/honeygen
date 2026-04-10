import { useEffect, useMemo, useState } from "react";
import type { WorldModelDetails, WorldModelPayload } from "../../types/worldModels";

interface WorldModelEditorProps {
  model: WorldModelDetails | null;
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
}

function createDraft(model: WorldModelDetails | null): WorldModelDraft {
  return {
    organizationName: model?.organization.name ?? "",
    organizationDescription: model?.organization.description ?? "",
    organizationIndustry: model?.organization.industry ?? "",
    organizationSize: model?.organization.size ?? "",
    organizationRegion: model?.organization.region ?? "",
    organizationDomainTheme: model?.organization.domain_theme ?? "",
    brandingTone: model?.branding.tone ?? "",
    brandingColors: (model?.branding.colors ?? []).join(", "),
    departments: (model?.departments ?? []).join("\n"),
    employees: (model?.employees ?? []).map((employee) => `${employee.name} | ${employee.role} | ${employee.department}`).join("\n"),
    projects: (model?.projects ?? []).join("\n"),
    documentThemes: (model?.document_themes ?? []).join("\n"),
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
  isSubmitting,
  submitError,
  submitSuccess,
  onSubmit,
}: WorldModelEditorProps) {
  const [draft, setDraft] = useState<WorldModelDraft>(() => createDraft(model));

  useEffect(() => {
    setDraft(createDraft(model));
  }, [model]);

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
    });
  }

  return (
    <form className="world-model-editor" onSubmit={handleSubmit}>
      <div className="stack stack--compact">
        <h3>{modeLabel}</h3>
        <p className="muted">{helperText}</p>
        {submitError ? <div className="error-alert">{submitError}</div> : null}
        {submitSuccess ? <div className="success-alert">{submitSuccess}</div> : null}
      </div>
      <div className="form-grid">
        <label>
          Organization name
          <input
            value={draft.organizationName}
            onChange={(event) => setDraft((current) => ({ ...current, organizationName: event.target.value }))}
          />
        </label>
        <label>
          Industry
          <input
            value={draft.organizationIndustry}
            onChange={(event) => setDraft((current) => ({ ...current, organizationIndustry: event.target.value }))}
          />
        </label>
        <label>
          Size
          <input
            value={draft.organizationSize}
            onChange={(event) => setDraft((current) => ({ ...current, organizationSize: event.target.value }))}
          />
        </label>
        <label>
          Region
          <input
            value={draft.organizationRegion}
            onChange={(event) => setDraft((current) => ({ ...current, organizationRegion: event.target.value }))}
          />
        </label>
        <label>
          Domain theme
          <input
            value={draft.organizationDomainTheme}
            onChange={(event) => setDraft((current) => ({ ...current, organizationDomainTheme: event.target.value }))}
          />
        </label>
        <label>
          Brand tone
          <input
            value={draft.brandingTone}
            onChange={(event) => setDraft((current) => ({ ...current, brandingTone: event.target.value }))}
          />
        </label>
        <label className="form-grid__full">
          Description
          <textarea
            rows={3}
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
