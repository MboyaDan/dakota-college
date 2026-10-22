import { cacheLife, cacheTag } from "next/cache";
import { buildOrDev, fetchPublic } from "./fetch";
import { getSiteOrFallback } from "./site";
import type { EventDetail, EventList, NewsArticle, NewsList } from "./types";

// News and events, read from their own endpoints (D-83) and cached like the
// site snapshot (D-74). The lists fall back to the snapshot's latest news and
// upcoming events when the API cannot be reached.

const life = { stale: 300, revalidate: 300 };

async function loadNewsPage(page: number): Promise<NewsList> {
  "use cache";
  cacheLife(life);
  cacheTag("public-news");
  const list = await fetchPublic<NewsList>(`/api/v1/public/news?page=${page}`);
  if (!list) throw new Error("news list: HTTP 404");
  return list;
}

export async function getNewsPage(page: number): Promise<NewsList> {
  try {
    return await loadNewsPage(page);
  } catch {
    const site = await getSiteOrFallback();
    const items = page === 1 ? site.news : [];
    return { items, page, total_pages: site.news.length ? 1 : 0, total: site.news.length };
  }
}

async function loadArticle(slug: string): Promise<NewsArticle | null> {
  "use cache";
  cacheLife(life);
  cacheTag("public-news");
  try {
    return await fetchPublic<NewsArticle>(`/api/v1/public/news/${encodeURIComponent(slug)}`);
  } catch (err) {
    if (buildOrDev()) return null;
    throw err;
  }
}

/** The article, or null when it is not published. */
export const getArticle = loadArticle;

async function loadEvents(): Promise<EventList> {
  "use cache";
  cacheLife(life);
  cacheTag("public-events");
  const list = await fetchPublic<EventList>("/api/v1/public/events");
  if (!list) throw new Error("events list: HTTP 404");
  return list;
}

export async function getEvents(): Promise<EventList> {
  try {
    return await loadEvents();
  } catch {
    const site = await getSiteOrFallback();
    return { upcoming: site.events, past: [] };
  }
}

async function loadEvent(slug: string): Promise<EventDetail | null> {
  "use cache";
  cacheLife(life);
  cacheTag("public-events");
  try {
    return await fetchPublic<EventDetail>(`/api/v1/public/events/${encodeURIComponent(slug)}`);
  } catch (err) {
    if (buildOrDev()) return null;
    throw err;
  }
}

/** The event, or null when it is not published. */
export const getEvent = loadEvent;
