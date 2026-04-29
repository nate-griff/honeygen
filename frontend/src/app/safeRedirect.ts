const loginRedirectOrigin = "http://honeygen.local";

export function sanitizeNextPath(next: string | null | undefined): string {
  const candidate = next?.trim();
  if (!candidate || !candidate.startsWith("/")) {
    return "/";
  }

  const parsed = new URL(candidate, loginRedirectOrigin);
  if (parsed.origin !== loginRedirectOrigin) {
    return "/";
  }

  return `${parsed.pathname}${parsed.search}${parsed.hash}`;
}
