import type { MetadataRoute } from "next";
import { getSite } from "@/lib/public/site";
import { resolveSiteUrl } from "@/lib/site-url";

// Public pages only; portals, the API and dev pages are never listed.
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = resolveSiteUrl(process.env.SITE_URL);
  const url = (path: string) => new URL(path, base).href;
  const site = await getSite();
  return [
    { url: url("/"), changeFrequency: "weekly", priority: 1 },
    { url: url("/programmes"), changeFrequency: "weekly", priority: 0.9 },
    ...site.programmes.map((p) => ({
      url: url(`/programmes/${p.slug}`),
      lastModified: p.updated_at,
      changeFrequency: "monthly" as const,
      priority: 0.8,
    })),
    { url: url("/schools"), changeFrequency: "monthly", priority: 0.7 },
    ...site.schools.map((s) => ({
      url: url(`/schools/${s.slug}`),
      changeFrequency: "monthly" as const,
      priority: 0.7,
    })),
    { url: url("/about"), changeFrequency: "yearly", priority: 0.5 },
    { url: url("/contact"), changeFrequency: "yearly", priority: 0.5 },
    { url: url("/apply"), changeFrequency: "monthly", priority: 0.6 },
  ];
}
