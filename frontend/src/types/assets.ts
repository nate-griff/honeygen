export interface Asset {
  id: string;
  generation_job_id: string;
  world_model_id: string;
  source_type: string;
  rendered_type: string;
  path: string;
  mime_type: string;
  size_bytes: number;
  tags: string[];
  previewable: boolean;
  checksum: string;
  created_at: string;
}

export interface AssetTreeNode {
  name: string;
  path: string;
  kind: "directory" | "file";
  asset_id?: string;
  children?: AssetTreeNode[];
}

export interface AssetContentResponse {
  asset: Asset;
  previewable: boolean;
  content?: string;
  message?: string;
}
