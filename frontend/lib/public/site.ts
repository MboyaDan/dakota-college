import { cacheLife, cacheTag } from "next/cache";
import { resolveApiOrigin } from "@/lib/api-origin";
import type { PublicSite } from "./types";
import bundled from "./snapshot.json";

// Public content for the website (D-74). Pages read one cached snapshot from
// the API, refreshed in the background every five minutes, so a change
// published in the staff portal is live within about five minutes (AC-06).
//
// When the API is asleep or down (AC-15):
//   - a prerendered page keeps serving its last good version, because a
//     failed refresh throws and Next.js keeps the previous result;
//   - during `next build` and `next dev`, and for pages rendered per request
//     (the filtered course finder), the copy bundled at build time is used.

// The file is the API's own response (D-74), so it has the contract's shape;
// cast through unknown because JSON imports type their literal values.
const fallback = bundled as unknown as PublicSite;

export const PUBLIC_SITE_TAG = "public-site";

async function loadSite(): Promise<PublicSite> {
  "use cache";
  cacheLife({ stale: 300, revalidate: 300 });
  cacheTag(PUBLIC_SITE_TAG);
  try {
    const origin = resolveApiOrigin(process.env.API_ORIGIN);
    // A free-tier API can take a while to wake up.
    const res = await fetch(`${origin}/api/v1/public/site`, { signal: AbortSignal.timeout(20_000) });
    if (!res.ok) throw new Error(`public site snapshot: HTTP ${res.status}`);
    return (await res.json()) as PublicSite;
  } catch (err) {
    if (process.env.NEXT_PHASE === "phase-production-build" || process.env.NODE_ENV === "development") {
      console.warn(`Using the bundled content snapshot: ${(err as Error).message}`);
      return fallback;
    }
    throw err;
  }
}

/** For prerendered pages: a failed refresh keeps the last good page. */
export const getSite = loadSite;

/** For pages rendered per request, which have no previous version to keep. */
export async function getSiteOrFallback(): Promise<PublicSite> {
  try {
    return await loadSite();
  } catch {
    return fallback;
  }
}
