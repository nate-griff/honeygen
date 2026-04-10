import { apiRequest } from "./client";
import type { StatusResponse } from "../types/status";

export function getStatus(): Promise<StatusResponse> {
  return apiRequest<StatusResponse>("/api/status");
}
