import { apiRequest, buildQuery } from "./client";
import type { EventRecord } from "../types/events";

export async function listEvents(filters: {
  limit?: number;
  offset?: number;
  world_model_id?: string;
  path?: string;
  source_ip?: string;
  status_code?: number;
} = {}): Promise<EventRecord[]> {
  const response = await apiRequest<{ items: EventRecord[] }>(`/api/events${buildQuery(filters)}`);
  return response.items ?? [];
}

export function getEvent(id: string): Promise<EventRecord> {
  return apiRequest<EventRecord>(`/api/events/${id}`);
}
