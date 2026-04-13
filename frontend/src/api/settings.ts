import { apiRequest } from "./client";

export interface ProviderSettings {
  base_url: string;
  api_key: string;
  model: string;
  ready: boolean;
  mode: string;
}

export function getProviderSettings(): Promise<ProviderSettings> {
  return apiRequest<ProviderSettings>("/api/settings/provider");
}

export function updateProviderSettings(settings: {
  base_url: string;
  api_key: string;
  model: string;
}): Promise<ProviderSettings> {
  return apiRequest<ProviderSettings>("/api/settings/provider", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function testProviderConnection(): Promise<{
  ready: boolean;
  mode: string;
  base_url: string;
  model: string;
}> {
  return apiRequest<{ ready: boolean; mode: string; base_url: string; model: string }>(
    "/api/provider/test",
    { method: "POST" },
  );
}
