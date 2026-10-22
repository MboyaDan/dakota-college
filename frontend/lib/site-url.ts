/** Local `next dev` and `next start` origin. */
export const DEFAULT_SITE_URL = "http://localhost:3000";

/**
 * Resolves the public site origin used for absolute URLs in metadata, such as
 * Open Graph images. The production domain is set at deployment (Stage 19).
 * Accepts only an http(s) origin with no path, query or credentials.
 */
export function resolveSiteUrl(value: string | undefined): URL {
  const raw = value?.trim();
  if (!raw) return new URL(DEFAULT_SITE_URL);

  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`SITE_URL is not a valid URL: "${raw}"`);
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(`SITE_URL must use http or https, got "${url.protocol}"`);
  }
  if (url.username || url.password) {
    throw new Error("SITE_URL must not contain credentials");
  }
  if (url.pathname !== "/" || url.search || url.hash) {
    throw new Error(`SITE_URL must be an origin only (no path or query), got "${raw}"`);
  }
  return new URL(url.origin);
}
