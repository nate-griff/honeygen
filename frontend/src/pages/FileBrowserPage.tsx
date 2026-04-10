import { useLoaderData, useNavigate } from "react-router-dom";
import { getAsset, getAssetContent, listAssetTree } from "../api/assets";
import { listGenerationJobs } from "../api/generation";
import { listWorldModels } from "../api/worldModels";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { AssetMetadataCard } from "../components/assets/AssetMetadataCard";
import { AssetPreview } from "../components/assets/AssetPreview";
import { AssetTree } from "../components/assets/AssetTree";
import { EmptyState } from "../components/layout/EmptyState";
import { APIClientError } from "../api/client";
import type { Asset, AssetContentResponse, AssetTreeNode } from "../types/assets";
import type { GenerationJob } from "../types/generation";
import type { WorldModelSummary } from "../types/worldModels";

interface FileBrowserLoaderData {
  models: WorldModelSummary[];
  jobs: GenerationJob[];
  selectedWorldModelID: string;
  selectedGenerationJobID: string;
  selectedAssetID: string;
  tree: AssetTreeNode[];
  asset: Asset | null;
  assetContent: AssetContentResponse | null;
}

export async function fileBrowserLoader({ request }: { request: Request }): Promise<FileBrowserLoaderData> {
  const url = new URL(request.url);
  const requestedWorldModelID = url.searchParams.get("world_model_id")?.trim() ?? "";
  const requestedGenerationJobID = url.searchParams.get("generation_job_id")?.trim() ?? "";
  const requestedAssetID = url.searchParams.get("asset_id")?.trim() ?? "";

  const models = await listWorldModels();
  const selectedWorldModelID = requestedWorldModelID || models[0]?.id || "";
  const jobs = selectedWorldModelID ? await listGenerationJobs({ world_model_id: selectedWorldModelID, limit: 25 }) : [];
  const selectedGenerationJobID = requestedGenerationJobID;
  const tree = await listAssetTree({
    world_model_id: selectedWorldModelID || undefined,
    generation_job_id: selectedGenerationJobID || undefined,
  });

  let asset: Asset | null = null;
  let assetContent: AssetContentResponse | null = null;
  if (requestedAssetID) {
    try {
      [asset, assetContent] = await Promise.all([getAsset(requestedAssetID), getAssetContent(requestedAssetID)]);
    } catch (error) {
      if (!(error instanceof APIClientError && error.status === 404)) {
        throw error;
      }
    }
  }

  return {
    models,
    jobs,
    selectedWorldModelID,
    selectedGenerationJobID,
    selectedAssetID: requestedAssetID,
    tree,
    asset,
    assetContent,
  };
}

export default function FileBrowserPage() {
  const {
    models,
    jobs,
    selectedWorldModelID,
    selectedGenerationJobID,
    selectedAssetID,
    tree,
    asset,
    assetContent,
  } = useLoaderData() as FileBrowserLoaderData;
  const navigate = useNavigate();

  function updateQuery(next: {
    world_model_id?: string;
    generation_job_id?: string;
    asset_id?: string;
  }) {
    const params = new URLSearchParams();
    if (next.world_model_id) {
      params.set("world_model_id", next.world_model_id);
    }
    if (next.generation_job_id) {
      params.set("generation_job_id", next.generation_job_id);
    }
    if (next.asset_id) {
      params.set("asset_id", next.asset_id);
    }
    navigate(`/files${params.toString() ? `?${params.toString()}` : ""}`);
  }

  return (
    <div className="stack">
      <PageHeader
        title="File Browser"
        description="Inspect the generated asset tree, review metadata, and safely preview supported content types."
      />
      <Panel title="Filters" subtitle="Limit the tree by world model and generation job">
        <div className="toolbar-row">
          <label className="toolbar-field">
            World model
            <select
              value={selectedWorldModelID}
              onChange={(event) =>
                updateQuery({
                  world_model_id: event.target.value,
                  generation_job_id: "",
                  asset_id: "",
                })
              }
            >
              {models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
          </label>
          <label className="toolbar-field">
            Generation job
            <select
              value={selectedGenerationJobID}
              onChange={(event) =>
                updateQuery({
                  world_model_id: selectedWorldModelID,
                  generation_job_id: event.target.value,
                  asset_id: "",
                })
              }
            >
              <option value="">All jobs</option>
              {jobs.map((job) => (
                <option key={job.id} value={job.id}>
                  {job.id} · {job.status}
                </option>
              ))}
            </select>
          </label>
        </div>
      </Panel>
      <div className="split-layout">
        <Panel title="Asset tree" subtitle="Live tree from /api/assets/tree">
          {tree.length === 0 ? (
            <EmptyState title="No assets found" message="Run generation for this world model to populate the file tree." />
          ) : (
            <AssetTree
              nodes={tree}
              onSelectAsset={(assetID) =>
                updateQuery({
                  world_model_id: selectedWorldModelID,
                  generation_job_id: selectedGenerationJobID,
                  asset_id: assetID,
                })
              }
              selectedAssetID={selectedAssetID}
            />
          )}
        </Panel>
        <div className="stack">
          <Panel title="Asset metadata" subtitle="Selected asset details from /api/assets/{id}">
            {asset ? <AssetMetadataCard asset={asset} /> : <EmptyState title="Pick an asset" message="Choose a file from the tree to inspect metadata." />}
          </Panel>
          <Panel title="Preview" subtitle="Sanitized rendering for markdown and HTML, text for plain formats">
            <AssetPreview asset={asset} content={assetContent} />
          </Panel>
        </div>
      </div>
    </div>
  );
}
