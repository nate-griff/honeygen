import { redirect } from "react-router-dom";
import { APIClientError, apiRequest } from "./client";

export interface AdminSessionResponse {
  authenticated: boolean;
}

export function getAdminSession(): Promise<AdminSessionResponse> {
  return apiRequest<AdminSessionResponse>("/api/auth/session");
}

export function loginAdminSession(password: string): Promise<AdminSessionResponse> {
  return apiRequest<AdminSessionResponse>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}

export function logoutAdminSession(): Promise<void> {
  return apiRequest<void>("/api/auth/logout", { method: "POST" });
}

export async function requireAdminSession(nextPath: string): Promise<void> {
  try {
    await getAdminSession();
  } catch (error) {
    if (error instanceof APIClientError && error.status === 401) {
      throw redirect(`/login?next=${encodeURIComponent(nextPath)}`);
    }
    throw error;
  }
}
