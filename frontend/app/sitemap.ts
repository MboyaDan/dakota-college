import type { MetadataRoute } from "next";
import { getEvents, getNewsPage } from "@/lib/public/news";
import { getSite } from "@/lib/public/site";
import { resolveSiteUrl } from "@/lib/site-url";

// Public pages only; portals, the API and dev pages are never listed.
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = resolveSiteUrl(process.env.SITE_URL);
  const url = (path: string) => new URL(path, base).href;
  const site = await getSite();
  const published = new Set(site.pages.map((p) => p.slug));
  // Text pages are listed only once the college publishes them (D-82).
  const textPages = [
    ["admissions", "/admissions"],
    ["student-life", "/student-life"],
    ["alumni", "/alumni"],
    ["privacy", "/privacy"],
    ["cookies", "/cookies"],
    ["terms", "/terms"],
    ["accessibility", "/accessibility"],
  ].filter(([slug]) => published.has(slug));

  const news = [];
  for (let page = 1, total = 1; page <= total && page <= 50; page++) {
    const list = await getNewsPage(page);
    news.push(...list.items);
    total = list.total_pages;
  }
  const events = await getEvents();

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
    { url: url("/admissions/requirements"), changeFrequency: "monthly", priority: 0.7 },
    { url: url("/admissions/fees"), changeFrequency: "monthly", priority: 0.7 },
    { url: url("/admissions/intakes"), changeFrequency: "weekly", priority: 0.7 },
    { url: url("/faqs"), changeFrequency: "monthly", priority: 0.6 },
    { url: url("/news"), changeFrequency: "weekly", priority: 0.6 },
    ...news.map((n) => ({
      url: url(`/news/${n.slug}`),
      lastModified: n.published_at ?? undefined,
      priority: 0.5,
    })),
    { url: url("/events"), changeFrequency: "weekly", priority: 0.5 },
    ...[...events.upcoming, ...events.past].map((e) => ({ url: url(`/events/${e.slug}`), priority: 0.4 })),
    ...textPages.map(([slug, path]) => ({
      url: url(path),
      lastModified: site.pages.find((p) => p.slug === slug)?.updated_at,
      changeFrequency: "yearly" as const,
      priority: slug === "admissions" ? 0.8 : 0.3,
    })),
    { url: url("/about"), changeFrequency: "yearly", priority: 0.5 },
    { url: url("/contact"), changeFrequency: "yearly", priority: 0.5 },
    { url: url("/apply"), changeFrequency: "monthly", priority: 0.6 },
  ];
}
