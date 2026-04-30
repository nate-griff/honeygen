import { useRef, useState } from "react";
import { useLoaderData, useNavigate } from "react-router-dom";
import { getAsset, getAssetContent, listAssetTree, uploadAsset } from "../api/assets";
import { listGenerationJobs } from "../api/generation";
import { listWorldModels } from "../api/worldModels";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { AssetMetadataCard } from "../components/assets/AssetMetadataCard";
import { deleteAsset } from "../api/assets";
import { AssetPreview } from "../components/assets/AssetPreview";
import { AssetTree } from "../components/assets/AssetTree";
import { EmptyState } from "../components/layout/EmptyState";
import { ErrorAlert } from "../components/layout/ErrorAlert";
import type { Asset, AssetContentResponse, AssetTreeNode } from "../types/assets";
import type { GenerationJob } from "../types/generation";
import type { WorldModelSummary } from "../types/worldModels";
import { getUploadErrorMessage } from "./fileBrowserUploadErrors";

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

  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploadTargetPath, setUploadTargetPath] = useState("");
  const [uploadLoading, setUploadLoading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const selectedJob = jobs.find((j) => j.id === selectedGenerationJobID) ?? null;
  const canUpload = selectedJob?.status === "completed";
  const canDeleteAsset = selectedJob?.status === "completed" && !!asset;

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

  async function handleUpload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!uploadFile || !uploadTargetPath.trim() || !selectedGenerationJobID) return;

    setUploadLoading(true);
    setUploadError(null);

      try {
        const created = await uploadAsset({
          generation_job_id: selectedGenerationJobID,
          target_path: uploadTargetPath.trim(),
          file: uploadFile,
        });

        setUploadFile(null);
        setUploadTargetPath("");
        if (fileInputRef.current) {
          fileInputRef.current.value = "";
        }
        setUploadLoading(false);

        updateQuery({
          world_model_id: selectedWorldModelID,
          generation_job_id: selectedGenerationJobID,
          asset_id: created.id,
      });
    } catch (error) {
      setUploadError(getUploadErrorMessage(error));
      setUploadLoading(false);
    }
  }

  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  async function handleDeleteAsset() {
    if (!asset) return;
    if (!window.confirm("Are you sure you want to delete this asset? This action cannot be undone.")) return;
    setDeleteLoading(true);
    setDeleteError(null);
    try {
      await deleteAsset(asset.id);
      setDeleteLoading(false);
      updateQuery({
        world_model_id: selectedWorldModelID,
        generation_job_id: selectedGenerationJobID,
        asset_id: undefined,
      });
    } catch (error) {
      let msg = "Failed to delete asset.";
      if (error instanceof Error) {
        msg = error.message;
        if ((error as any).code === "asset_not_deletable") {
          msg = "This asset cannot be deleted.";
        }
      }
      setDeleteError(msg);
      setDeleteLoading(false);
    }
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
      {canUpload && (
        <Panel
          title="Upload file"
          subtitle={`Add a custom file to job ${selectedGenerationJobID}. Server-enforced upload limit defaults to 25 MB and never exceeds 100 MB. Existing files cannot be overwritten.`}
        >
          <form className="stack stack--compact" onSubmit={handleUpload}>
            <div className="form-grid">
              <label className="toolbar-field form-grid__full">
                File
                <input
                  ref={fileInputRef}
                  type="file"
                  required
                  disabled={uploadLoading}
                  onChange={(e) => {
                    const f = e.target.files?.[0] ?? null;
                    setUploadFile(f);
                    setUploadError(null);
                    if (f && !uploadTargetPath.trim()) {
                      setUploadTargetPath(f.name);
                    }
                  }}
                />
              </label>
              <label className="toolbar-field form-grid__full">
                Target path within job tree
                <input
                  type="text"
                  placeholder="e.g. public/custom/report.pdf"
                  value={uploadTargetPath}
                  required
                  disabled={uploadLoading}
                  onChange={(e) => {
                    setUploadTargetPath(e.target.value);
                    setUploadError(null);
                  }}
                />
                <span className="field-hint">
                  Relative path inside the job — e.g. <code>public/notes.txt</code> or <code>shared/data.csv</code>
                </span>
              </label>
            </div>
            {uploadError && <ErrorAlert message={uploadError} />}
            <div className="button-row">
              <button
                type="submit"
                className="button button--primary"
                disabled={uploadLoading || !uploadFile || !uploadTargetPath.trim()}
              >
                {uploadLoading ? "Uploading…" : "Upload file"}
              </button>
            </div>
          </form>
        </Panel>
      )}
      {selectedGenerationJobID && !canUpload && selectedJob && (
        <Panel title="Upload file" subtitle="Upload a custom file into the selected job">
          <EmptyState
            title="Job not completed"
            message={`Uploads are only available for completed generation jobs. The selected job status is "${selectedJob.status}".`}
          />
        </Panel>
      )}
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
