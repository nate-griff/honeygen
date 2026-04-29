import { apiRequest, buildQuery } from "./client";
import type { Asset, AssetContentResponse, AssetTreeNode } from "../types/assets";

export async function listAssetTree(filters: {
  world_model_id?: string;
  generation_job_id?: string;
} = {}): Promise<AssetTreeNode[]> {
  const response = await apiRequest<{ items: AssetTreeNode[] }>(`/api/assets/tree${buildQuery(filters)}`);
  return response.items ?? [];
}

export function getAsset(id: string): Promise<Asset> {
  return apiRequest<Asset>(`/api/assets/${id}`);
}

export function getAssetContent(id: string): Promise<AssetContentResponse> {
  return apiRequest<AssetContentResponse>(`/api/assets/${id}/content`);
}

export interface UploadAssetParams {
  generation_job_id: string;
  target_path: string;
  file: File;
}

export function uploadAsset(params: UploadAssetParams): Promise<Asset> {
  const form = new FormData();
  form.append("generation_job_id", params.generation_job_id);
  form.append("target_path", params.target_path);
  form.append("file", params.file);
  return apiRequest<Asset>("/api/assets/upload", { method: "POST", body: form });
}
