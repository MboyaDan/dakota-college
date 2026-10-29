import type { NextConfig } from "next";
import { resolveApiOrigin } from "./lib/api-origin";

const apiOrigin = resolveApiOrigin(process.env.API_ORIGIN);

// Headers for every page. The Content-Security-Policy is set per request in
// proxy.ts (nonces on signed-in pages, D-191). API responses carry the Go
// API's own headers. HSTS is ignored by browsers over plain http, so local
// runs are unaffected.
const pageHeaders = [
  { key: "Strict-Transport-Security", value: "max-age=31536000" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
  {
    key: "Permissions-Policy",
    value: "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()",
  },
];

const nextConfig: NextConfig = {
  // Decided at build time and baked into the build, so a page regenerated
  // later (every 5 minutes, D-74) cannot disagree with how it was built.
  env: { ENABLE_DESIGN_PAGE: process.env.ENABLE_DESIGN_PAGE === "1" ? "1" : "" },
  cacheComponents: true,
  partialPrefetching: true,
  poweredByHeader: false,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async headers() {
    return [{ source: "/((?!api/).*)", headers: pageHeaders }];
  },
  // The browser only talks to this origin; /api/* is proxied to the Go API so
  // session cookies stay first-party (ADR-04, D-11).
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
  },
};

export default nextConfig;
