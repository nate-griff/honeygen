import { apiRequest } from "./client";
import type { Deployment, CreateDeploymentRequest } from "../types/deployments";

export async function listDeployments(): Promise<Deployment[]> {
  const response = await apiRequest<{ items: Deployment[] }>("/api/deployments");
  return response.items ?? [];
}

export function getDeployment(id: string): Promise<Deployment> {
  return apiRequest<Deployment>(`/api/deployments/${id}`);
}

export function createDeployment(request: CreateDeploymentRequest): Promise<Deployment> {
  return apiRequest<Deployment>("/api/deployments", {
    method: "POST",
    body: JSON.stringify(request),
  });
}

export function deleteDeployment(id: string): Promise<void> {
  return apiRequest<void>(`/api/deployments/${id}`, { method: "DELETE" });
}

export function startDeployment(id: string): Promise<Deployment> {
  return apiRequest<Deployment>(`/api/deployments/${id}/start`, { method: "POST" });
}

export function stopDeployment(id: string): Promise<Deployment> {
  return apiRequest<Deployment>(`/api/deployments/${id}/stop`, { method: "POST" });
}
