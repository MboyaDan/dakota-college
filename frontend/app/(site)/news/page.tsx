import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { ArrowRight } from "@/components/icons";
import { ContentImage } from "@/components/public/content-image";
import { PageHeader } from "@/components/public/page-header";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Pagination } from "@/components/ui/pagination";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { getNewsPage } from "@/lib/public/news";
import { pageMetadata } from "@/lib/seo";
import { formatDate } from "@/lib/staff/datetime";

export const metadata: Metadata = pageMetadata({
  title: "News",
  description: "News from Dakota Training College.",
  path: "/news",
});

export default function NewsPage({ searchParams }: PageProps<"/news">) {
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "News" }]}
        title="News"
        lead={
          <>
            What is happening at the college.{" "}
            <Link href="/events" className="text-royal underline">
              See upcoming events
            </Link>
            .
          </>
        }
      />
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-8 lg:px-6">
        <Suspense
          fallback={
            <Loading label="Loading news">
              <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                <Skeleton className="h-72" />
                <Skeleton className="h-72" />
                <Skeleton className="h-72" />
              </div>
            </Loading>
          }
        >
          <NewsList searchParams={searchParams} />
        </Suspense>
      </div>
    </>
  );
}

async function NewsList({ searchParams }: Pick<PageProps<"/news">, "searchParams">) {
  const params = await searchParams;
  const raw = Number(Array.isArray(params.page) ? params.page[0] : params.page);
  const page = Number.isInteger(raw) && raw >= 1 && raw <= 10000 ? raw : 1;
  const list = await getNewsPage(page);

  if (!list.items.length) {
    return (
      <EmptyState
        title={page > 1 ? "No more news" : "No news yet"}
        description={page > 1 ? "There are no older articles." : "News from the college will appear here."}
        action={
          page > 1 ? (
            <Link href="/news" className="font-semibold text-royal underline">
              Back to the latest news
            </Link>
          ) : undefined
        }
      />
    );
  }

  return (
    <>
      <ul className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {list.items.map((n) => (
          <li key={n.slug} className="flex">
            <Card className="relative w-full overflow-hidden p-0">
              {n.cover ? (
                <ContentImage
                  image={n.cover}
                  alt={n.cover_alt}
                  sizes="(min-width: 1024px) 33vw, (min-width: 768px) 50vw, 100vw"
                  className="aspect-[16/9] w-full object-cover"
                />
              ) : null}
              <div className="flex flex-1 flex-col gap-2 p-5">
                {n.published_at ? (
                  <p className="text-sm text-ink-muted">{formatDate(n.published_at.slice(0, 10))}</p>
                ) : null}
                <h2 className="text-xl font-semibold text-navy-900">
                  <Link href={`/news/${n.slug}`} className="after:absolute after:inset-0 hover:underline">
                    {n.title}
                  </Link>
                </h2>
                {n.summary ? <p>{n.summary}</p> : null}
                <ArrowRight aria-hidden className="mt-auto size-5 text-royal" />
              </div>
            </Card>
          </li>
        ))}
      </ul>
      <Pagination
        page={list.page}
        totalPages={list.total_pages}
        hrefFor={(p) => (p === 1 ? "/news" : `/news?page=${p}`)}
        label="News pages"
      />
    </>
  );
}
