import { formatDateTime, formatFileSize, normalizeDownloadPath } from "../../app/format";
import type { Asset } from "../../types/assets";

interface AssetMetadataCardProps {
  asset: Asset;
  onDelete?: (() => void) | null;
  deleteLoading?: boolean;
  canDelete?: boolean;
}

export function AssetMetadataCard({ asset, onDelete, deleteLoading, canDelete }: AssetMetadataCardProps) {
  return (
    <div className="stack stack--compact">
      <dl className="detail-grid">
        <div>
          <dt>Asset ID</dt>
          <dd>{asset.id}</dd>
        </div>
        <div>
          <dt>World model</dt>
          <dd>{asset.world_model_id}</dd>
        </div>
        <div>
          <dt>Generation job</dt>
          <dd>{asset.generation_job_id}</dd>
        </div>
        <div>
          <dt>Rendered type</dt>
          <dd>{asset.rendered_type}</dd>
        </div>
        <div>
          <dt>MIME type</dt>
          <dd>{asset.mime_type}</dd>
        </div>
        <div>
          <dt>Size</dt>
          <dd>{formatFileSize(asset.size_bytes)}</dd>
        </div>
        <div className="detail-grid__full">
          <dt>Path</dt>
          <dd className="code-inline">{asset.path}</dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd>{formatDateTime(asset.created_at)}</dd>
        </div>
        <div>
          <dt>Checksum</dt>
          <dd className="code-inline">{asset.checksum}</dd>
        </div>
      </dl>
      <div className="button-row">
        <a className="button button--ghost" download href={normalizeDownloadPath(asset.path)}>
          Download asset
        </a>
        {canDelete && (
          <button
            className="button button--danger"
            type="button"
            onClick={onDelete ?? undefined}
            disabled={deleteLoading}
            style={{ marginLeft: 8 }}
          >
            {deleteLoading ? "Deleting…" : "Delete asset"}
          </button>
        )}
      </div>
    </div>
  );
}
