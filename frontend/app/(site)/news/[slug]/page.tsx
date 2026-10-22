import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ContentImage } from "@/components/public/content-image";
import { JsonLd } from "@/components/public/json-ld";
import { PageHeader } from "@/components/public/page-header";
import { RichText } from "@/components/public/rich-text";
import { getArticle } from "@/lib/public/news";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import { site as siteFacts } from "@/lib/site";
import { resolveSiteUrl } from "@/lib/site-url";
import { formatDate } from "@/lib/staff/datetime";

// With Cache Components the list must not be empty (D-74). Older articles
// render on first visit and are then cached.
export async function generateStaticParams() {
  const site = await getSite();
  return site.news.length ? site.news.map((n) => ({ slug: n.slug })) : [{ slug: "none" }];
}

export async function generateMetadata({ params }: PageProps<"/news/[slug]">): Promise<Metadata> {
  const article = await getArticle((await params).slug);
  if (!article) return {};
  return pageMetadata({
    title: article.seo_title || article.title,
    description: article.seo_description || article.summary || article.title,
    path: `/news/${article.slug}`,
  });
}

export default async function ArticlePage({ params }: PageProps<"/news/[slug]">) {
  const article = await getArticle((await params).slug);
  if (!article) notFound();
  const base = resolveSiteUrl(process.env.SITE_URL);

  return (
    <>
      <JsonLd
        data={{
          "@context": "https://schema.org",
          "@type": "NewsArticle",
          headline: article.title,
          ...(article.published_at ? { datePublished: article.published_at } : {}),
          dateModified: article.updated_at,
          ...(article.cover ? { image: [article.cover.url] } : {}),
          publisher: { "@type": "EducationalOrganization", name: siteFacts.name, url: base.href },
          mainEntityOfPage: new URL(`/news/${article.slug}`, base).href,
        }}
      />
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "News", href: "/news" }, { label: article.title }]}
        title={article.title}
        lead={article.summary || undefined}
      >
        {article.published_at ? (
          <p className="text-ink-muted">Published {formatDate(article.published_at.slice(0, 10))}</p>
        ) : null}
      </PageHeader>
      <article className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10 lg:px-6">
        {article.cover ? (
          <ContentImage
            image={article.cover}
            alt={article.cover_alt}
            sizes="(min-width: 768px) 720px, 100vw"
            className="w-full rounded-card"
            priority
          />
        ) : null}
        <RichText text={article.body} />
        <Link
          href="/news"
          className="inline-flex min-h-touch items-center font-semibold text-royal underline"
        >
          More news
        </Link>
      </article>
    </>
  );
}
