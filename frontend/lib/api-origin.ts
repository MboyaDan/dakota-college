/** Local Compose API port (backend/.env.example API_PORT). */
export const DEFAULT_API_ORIGIN = "http://localhost:8180";

/**
 * Resolves the Go API origin that `/api/*` is rewritten to (D-11).
 * Accepts only an http(s) origin with no path, query or credentials, so a
 * typo cannot silently send API traffic somewhere unexpected.
 */
export function resolveApiOrigin(value: string | undefined): string {
  const raw = value?.trim();
  if (!raw) return DEFAULT_API_ORIGIN;

  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`API_ORIGIN is not a valid URL: "${raw}"`);
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(`API_ORIGIN must use http or https, got "${url.protocol}"`);
  }
  if (url.username || url.password) {
    throw new Error("API_ORIGIN must not contain credentials");
  }
  if (url.pathname !== "/" || url.search || url.hash) {
    throw new Error(`API_ORIGIN must be an origin only (no path or query), got "${raw}"`);
  }
  return url.origin;
}
