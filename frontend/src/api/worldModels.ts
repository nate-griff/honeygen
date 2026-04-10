import { apiRequest } from "./client";
import type { WorldModelDetails, WorldModelPayload, WorldModelSummary } from "../types/worldModels";

export async function listWorldModels(): Promise<WorldModelSummary[]> {
  const response = await apiRequest<{ items: WorldModelSummary[] }>("/api/world-models");
  return response.items;
}

export function getWorldModel(id: string): Promise<WorldModelDetails> {
  return apiRequest<WorldModelDetails>(`/api/world-models/${id}`);
}

export function createWorldModel(payload: WorldModelPayload): Promise<WorldModelDetails> {
  return apiRequest<WorldModelDetails>("/api/world-models", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function updateWorldModel(id: string, payload: WorldModelPayload): Promise<WorldModelDetails> {
  return apiRequest<WorldModelDetails>(`/api/world-models/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}
