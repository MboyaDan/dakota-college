import { cacheLife, cacheTag } from "next/cache";
import { buildOrDev, fetchPublic } from "./fetch";
import { getSiteOrFallback } from "./site";
import type { EventDetail, EventList, NewsArticle, NewsList } from "./types";

// News and events, read from their own endpoints (D-83) and cached like the
// site snapshot (D-74). The lists fall back to the snapshot's latest news and
// upcoming events when the API cannot be reached.

const life = { stale: 300, revalidate: 300 };

/** The snapshot's latest news as page 1, for when the API cannot be reached. */
async function newsFallback(page: number): Promise<NewsList> {
  const site = await getSiteOrFallback();
  const items = page === 1 ? site.news : [];
  return { items, page, total_pages: site.news.length ? 1 : 0, total: site.news.length };
}

// A failure inside a cached function stops `next build`, even if the caller
// catches it, so during build and dev the fallback is returned from inside.
async function loadNewsPage(page: number): Promise<NewsList> {
  "use cache";
  cacheLife(life);
  cacheTag("public-news");
  try {
    const list = await fetchPublic<NewsList>(`/api/v1/public/news?page=${page}`);
    if (!list) throw new Error("news list: HTTP 404");
    return list;
  } catch (err) {
    if (buildOrDev()) return newsFallback(page);
    throw err;
  }
}

export async function getNewsPage(page: number): Promise<NewsList> {
  try {
    return await loadNewsPage(page);
  } catch {
    return newsFallback(page);
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

async function eventsFallback(): Promise<EventList> {
  const site = await getSiteOrFallback();
  return { upcoming: site.events, past: [] };
}

async function loadEvents(): Promise<EventList> {
  "use cache";
  cacheLife(life);
  cacheTag("public-events");
  try {
    const list = await fetchPublic<EventList>("/api/v1/public/events");
    if (!list) throw new Error("events list: HTTP 404");
    return list;
  } catch (err) {
    if (buildOrDev()) return eventsFallback();
    throw err;
  }
}

export async function getEvents(): Promise<EventList> {
  try {
    return await loadEvents();
  } catch {
    return eventsFallback();
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
