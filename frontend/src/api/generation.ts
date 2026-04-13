import { apiRequest, buildQuery } from "./client";
import type { GenerationJob } from "../types/generation";

export function runGeneration(worldModelID: string): Promise<GenerationJob> {
  return apiRequest<GenerationJob>("/api/generation/run", {
    method: "POST",
    body: JSON.stringify({ world_model_id: worldModelID }),
  });
}

export async function listGenerationJobs(filters: {
  world_model_id?: string;
  limit?: number;
  offset?: number;
} = {}): Promise<GenerationJob[]> {
  const response = await apiRequest<{ items: GenerationJob[] }>(
    `/api/generation/jobs${buildQuery(filters)}`,
  );
  return response.items ?? [];
}

export function getGenerationJob(id: string): Promise<GenerationJob> {
  return apiRequest<GenerationJob>(`/api/generation/jobs/${id}`);
}
