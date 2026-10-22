import { resolveApiOrigin } from "@/lib/api-origin";

/** True while `next build` or `next dev` runs, where bundled content may stand in. */
export function buildOrDev(): boolean {
  return process.env.NEXT_PHASE === "phase-production-build" || process.env.NODE_ENV === "development";
}

/**
 * GETs a public API path from the server. Returns null for 404 (not
 * published); throws for anything else, so a cached page keeps its last good
 * version (D-74).
 */
export async function fetchPublic<T>(path: string): Promise<T | null> {
  const origin = resolveApiOrigin(process.env.API_ORIGIN);
  // A free-tier API can take a while to wake up.
  const res = await fetch(`${origin}${path}`, { signal: AbortSignal.timeout(20_000) });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return (await res.json()) as T;
}
