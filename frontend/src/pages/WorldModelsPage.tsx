import { useEffect, useState } from "react";
import { Link, useLoaderData, useNavigate, useRevalidator, useSearchParams } from "react-router-dom";
import { APIClientError } from "../api/client";
import { createWorldModel, generateWorldModel, getWorldModel, listWorldModels, updateWorldModel } from "../api/worldModels";
import { ErrorAlert } from "../components/layout/ErrorAlert";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { WorldModelEditor } from "../components/world-models/WorldModelEditor";
import { WorldModelList } from "../components/world-models/WorldModelList";
import type { WorldModelDetails, WorldModelPayload, WorldModelSummary } from "../types/worldModels";

interface WorldModelsLoaderData {
  items: WorldModelSummary[];
  selectedWorldModel: WorldModelDetails | null;
}

export async function worldModelsLoader({ request }: { request: Request }): Promise<WorldModelsLoaderData> {
  const url = new URL(request.url);
  const requestedID = url.searchParams.get("world_model_id")?.trim() ?? "";
  const items = await listWorldModels();

  if (!requestedID) {
    return { items, selectedWorldModel: null };
  }

  try {
    return {
      items,
      selectedWorldModel: await getWorldModel(requestedID),
    };
  } catch (error) {
    if (error instanceof APIClientError && error.status === 404) {
      return { items, selectedWorldModel: null };
    }
    throw error;
  }
}

export default function WorldModelsPage() {
  const { items, selectedWorldModel } = useLoaderData() as WorldModelsLoaderData;
  const navigate = useNavigate();
  const revalidator = useRevalidator();
  const [searchParams] = useSearchParams();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string>();
  const [submitSuccess, setSubmitSuccess] = useState<string>();
  const [isGenerating, setIsGenerating] = useState(false);
  const [generateError, setGenerateError] = useState<string>();
  const [generateDescription, setGenerateDescription] = useState("");
  const [generatedPayload, setGeneratedPayload] = useState<WorldModelPayload | null>(null);

  useEffect(() => {
    setSubmitError(undefined);
  }, [selectedWorldModel?.id]);

  async function handleGenerate() {
    if (!generateDescription.trim()) return;
    setIsGenerating(true);
    setGenerateError(undefined);
    try {
      const result = await generateWorldModel(generateDescription);
      setGeneratedPayload(result.payload);
      navigate("/world-models");
    } catch (error) {
      setGenerateError(error instanceof Error ? error.message : "Unable to generate world model");
    } finally {
      setIsGenerating(false);
    }
  }

  async function handleSave(payload: Parameters<typeof createWorldModel>[0]) {
    setIsSubmitting(true);
    setSubmitError(undefined);
    setSubmitSuccess(undefined);
    try {
      const saved = selectedWorldModel
        ? await updateWorldModel(selectedWorldModel.id, payload)
        : await createWorldModel(payload);
      revalidator.revalidate();
      navigate(`/world-models?world_model_id=${saved.id}`);
      setSubmitSuccess(selectedWorldModel ? "World model updated." : "World model created.");
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "Unable to save world model");
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleSelect(id: string) {
    navigate(`/world-models?world_model_id=${id}`);
    setGeneratedPayload(null);
  }

  function handleCreateNew() {
    navigate("/world-models");
    setSubmitSuccess(undefined);
    setSubmitError(undefined);
    setGeneratedPayload(null);
  }

  const selectedID = selectedWorldModel?.id || searchParams.get("world_model_id") || undefined;

  return (
    <div className="stack">
      <PageHeader
        title="World Models"
        description="Create and update live world models, then hand them off to generation workflows."
        actions={
          selectedWorldModel ? (
            <Link className="button button--ghost" to={`/generation?world_model_id=${selectedWorldModel.id}`}>
              Use in generation
            </Link>
          ) : null
        }
      />
      <Panel title="Generate from description" subtitle="Use AI to create a world model from a natural language description.">
        <div className="stack stack--compact">
          <label className="toolbar-field">
            Describe the organization
            <textarea
              rows={3}
              value={generateDescription}
              onChange={(e) => setGenerateDescription(e.target.value)}
              placeholder="A mid-size law firm in Chicago specializing in corporate mergers..."
            />
          </label>
          {generateError ? <ErrorAlert message={generateError} /> : null}
          <div className="button-row">
            <button
              className="button button--ghost"
              disabled={isGenerating || !generateDescription.trim()}
              onClick={handleGenerate}
              type="button"
            >
              {isGenerating ? "Generating…" : "Generate world model"}
            </button>
          </div>
        </div>
      </Panel>
      <div className="split-layout">
        <Panel title="Available world models" subtitle="Live summaries from /api/world-models">
          <WorldModelList items={items} onCreateNew={handleCreateNew} onSelect={handleSelect} selectedID={selectedID} />
        </Panel>
        <Panel title={selectedWorldModel ? "Model editor" : "New model"} subtitle="Create and update the live payload used by generation.">
          <WorldModelEditor
            initialPayload={!selectedWorldModel ? (generatedPayload ?? undefined) : undefined}
            isSubmitting={isSubmitting}
            key={generatedPayload && !selectedWorldModel ? "generated" : (selectedWorldModel?.id ?? "new-model")}
            model={selectedWorldModel}
            onSubmit={handleSave}
            submitError={submitError}
            submitSuccess={submitSuccess}
          />
        </Panel>
      </div>
    </div>
  );
}
