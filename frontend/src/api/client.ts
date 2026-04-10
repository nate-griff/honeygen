import type { APIErrorShape } from "../types/common";

const configuredBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? "").trim().replace(/\/+$/, "");

export class APIClientError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIClientError";
    this.status = status;
    this.code = code;
  }
}

function hasAPIErrorShape(payload: unknown): payload is APIErrorShape {
  if (!payload || typeof payload !== "object" || !("error" in payload)) {
    return false;
  }

  const { error } = payload as APIErrorShape;
  return typeof error?.code === "string" && typeof error?.message === "string";
}

export function buildQuery(params: Record<string, string | number | null | undefined>): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    query.set(key, String(value));
  }
  const result = query.toString();
  return result ? `?${result}` : "";
}

export function resolveAPIPath(path: string): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return configuredBaseUrl ? `${configuredBaseUrl}${normalizedPath}` : normalizedPath;
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers ?? {});
  headers.set("Accept", "application/json");

  const isStringBody = typeof init.body === "string";
  if (isStringBody && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(resolveAPIPath(path), {
    ...init,
    headers,
  });

  const rawText = await response.text();
  let payload: unknown = undefined;
  if (rawText) {
    try {
      payload = JSON.parse(rawText);
    } catch {
      if (response.ok) {
        throw new APIClientError(response.status, "invalid_json", "API returned invalid JSON");
      }
    }
  }

  if (!response.ok) {
    if (hasAPIErrorShape(payload)) {
      throw new APIClientError(response.status, payload.error.code, payload.error.message);
    }
    throw new APIClientError(response.status, "request_failed", response.statusText || "Request failed");
  }

  return payload as T;
}
