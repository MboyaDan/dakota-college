// Saves the API's public content snapshot into lib/public/snapshot.json, the
// copy the site falls back to when the API is unreachable (ADR-03, AC-15).
// Runs before `dev`, `build` and `typecheck`. The file is not committed: each
// build bundles the content of the API it was built against, so development
// sample data never reaches production (AC-17).
//
// If the API cannot be reached, an empty snapshot is written and pages show
// their empty states. Nothing is invented.
import { existsSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const out = fileURLToPath(new URL("../lib/public/snapshot.json", import.meta.url));

for (const file of [".env.local", ".env"]) {
  const path = fileURLToPath(new URL(`../${file}`, import.meta.url));
  if (existsSync(path)) process.loadEnvFile(path);
}
// Same default as lib/api-origin.ts (local Compose API).
const origin = (process.env.API_ORIGIN ?? "").trim() || "http://localhost:8180";

const empty = {
  generated_at: "",
  settings: {},
  banners: [],
  schools: [],
  programmes: [],
  faqs: [],
  statistics: [],
  accreditations: [],
  events: [],
  news: [],
  pages: [],
  gallery: [],
};

// A sleeping free-tier API (Render) answers with a "waking up" page for about
// a minute. On hosted builds set SNAPSHOT_WAIT_SECONDS (for example 150) to
// keep trying until real JSON arrives; locally it is 0, one try only.
const waitMs = Math.max(0, Number(process.env.SNAPSHOT_WAIT_SECONDS ?? "0") || 0) * 1000;
const deadline = Date.now() + waitMs;

async function pull() {
  const res = await fetch(new URL("/api/v1/public/site", origin), { signal: AbortSignal.timeout(30_000) });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  if (!(res.headers.get("content-type") ?? "").includes("application/json")) {
    throw new Error(
      `not JSON (${res.headers.get("content-type") ?? "no content type"}), API may be waking up`,
    );
  }
  return res.json();
}

let snapshot = empty;
for (let attempt = 1; ; attempt++) {
  try {
    snapshot = await pull();
    console.log(
      `snapshot: ${snapshot.schools.length} schools, ${snapshot.programmes.length} programmes from ${origin}`,
    );
    break;
  } catch (err) {
    if (Date.now() + 5_000 < deadline) {
      console.warn(`snapshot: attempt ${attempt} failed (${err.message}); retrying in 5 s`);
      await new Promise((r) => setTimeout(r, 5_000));
      continue;
    }
    console.warn(`snapshot: API not reachable at ${origin} (${err.message}); writing an empty snapshot`);
    break;
  }
}
writeFileSync(out, JSON.stringify(snapshot, null, 2) + "\n");
