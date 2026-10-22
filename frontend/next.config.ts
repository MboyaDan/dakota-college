import type { NextConfig } from "next";
import { resolveApiOrigin } from "./lib/api-origin";

const apiOrigin = resolveApiOrigin(process.env.API_ORIGIN);

const nextConfig: NextConfig = {
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
  // The browser only talks to this origin; /api/* is proxied to the Go API so
  // session cookies stay first-party (ADR-04, D-11).
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
  },
};

export default nextConfig;
