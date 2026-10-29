/** Local Compose MinIO, where S3_PUBLIC_BASE_URL points (backend/.env.example). */
export const DEFAULT_CONTENT_IMAGE_ORIGIN = "http://localhost:9000";

/**
 * Pages behind a sign-in, which hold personal data: the staff portal, the
 * student portal and the application form. They render per request so every
 * response gets a fresh script nonce (D-191). Public pages, including the
 * /apply landing page, stay cached under a fixed policy.
 */
const STRICT_PATH =
  /^\/(?:staff|student)(?:\/|$)|^\/apply\/(?:account|application|continue|new|resume|start)(?:\/|$)/;

export function isStrictPath(pathname: string): boolean {
  return STRICT_PATH.test(pathname);
}

/**
 * Resolves the origin uploaded content images are served from (the public
 * bucket in S3_PUBLIC_BASE_URL), for img-src. Origin only, like API_ORIGIN.
 */
export function resolveContentImageOrigin(value: string | undefined): string {
  const raw = value?.trim();
  if (!raw) return DEFAULT_CONTENT_IMAGE_ORIGIN;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`CONTENT_IMAGE_ORIGIN is not a valid URL: "${raw}"`);
  }
  if ((url.protocol !== "http:" && url.protocol !== "https:") || url.username || url.password) {
    throw new Error(`CONTENT_IMAGE_ORIGIN must be an http(s) origin, got "${raw}"`);
  }
  if (url.pathname !== "/" || url.search || url.hash) {
    throw new Error(`CONTENT_IMAGE_ORIGIN must be an origin only (no path), got "${raw}"`);
  }
  return url.origin;
}

type PolicyOptions = {
  /** Per-request nonce; strict pages only. Without one, inline scripts are allowed. */
  nonce?: string;
  imageOrigin: string;
  /** Development needs eval for React's error overlay. */
  dev?: boolean;
};

/**
 * The page Content-Security-Policy. Nothing loads from another origin except
 * content images; no frames, plugins or other-origin connections; forms post
 * only here. Inline styles stay allowed because components set style
 * attributes, which nonces cannot cover.
 */
export function contentSecurityPolicy({ nonce, imageOrigin, dev = false }: PolicyOptions): string {
  const script = [
    "'self'",
    nonce ? `'nonce-${nonce}' 'strict-dynamic'` : "'unsafe-inline'",
    dev ? "'unsafe-eval'" : "",
  ]
    .filter(Boolean)
    .join(" ");
  return [
    "default-src 'self'",
    `script-src ${script}`,
    "style-src 'self' 'unsafe-inline'",
    `img-src 'self' data: blob: ${imageOrigin}`,
    "font-src 'self'",
    "connect-src 'self'",
    "frame-src 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");
}

export const CLIENT_IP_HEADER = "x-dakota-client-ip";
export const CLIENT_IP_KEY_HEADER = "x-dakota-client-ip-key";

/**
 * Passes the visitor's address to the API (R-23). `sourceHeader` names the
 * header the hosting platform sets and overwrites (for example
 * `cf-connecting-ip`), so visitors cannot forge it; the shared secret tells
 * the API the value came from here. Whatever a visitor sent under our own
 * header names is always removed first.
 */
export function forwardClientIP(
  headers: Headers,
  sourceHeader: string | undefined,
  secret: string | undefined,
): void {
  headers.delete(CLIENT_IP_HEADER);
  headers.delete(CLIENT_IP_KEY_HEADER);
  if (!sourceHeader || !secret) return;
  const ip = headers.get(sourceHeader)?.split(",")[0]?.trim();
  if (!ip) return;
  headers.set(CLIENT_IP_HEADER, ip);
  headers.set(CLIENT_IP_KEY_HEADER, secret);
}
