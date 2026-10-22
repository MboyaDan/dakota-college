import type { MetadataRoute } from "next";
import { indexable } from "@/lib/site";
import { resolveSiteUrl } from "@/lib/site-url";

// Search engines are kept out entirely until launch (SITE_INDEXABLE=1).
export default function robots(): MetadataRoute.Robots {
  if (!indexable) return { rules: { userAgent: "*", disallow: "/" } };
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/staff", "/student", "/api/", "/design", "/reset-password"],
    },
    sitemap: new URL("/sitemap.xml", resolveSiteUrl(process.env.SITE_URL)).href,
  };
}
