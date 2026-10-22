import type { Metadata } from "next";
import type { ReactNode } from "react";
import { LinkButton } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import type { Crumb } from "@/components/ui/breadcrumb";
import { PageHeader } from "./page-header";
import { RichText } from "./rich-text";

// A page whose words come from the Pages content type (D-82), at a fixed
// address. Until the college publishes it, the page says so and is not
// indexed.

async function findPage(slug: string) {
  const site = await getSite();
  return site.pages.find((p) => p.slug === slug);
}

export async function textPageMetadata(slug: string, title: string, path: string): Promise<Metadata> {
  const page = await findPage(slug);
  if (!page) return { title, robots: { index: false, follow: true } };
  return pageMetadata({
    title: page.seo_title || page.title,
    description: page.seo_description || page.summary || title,
    path,
  });
}

type TextPageProps = {
  slug: string;
  /** Title while the page is not published yet. */
  title: string;
  crumbs: Crumb[];
  /** Shown under the page text, whether or not the page is published. */
  children?: ReactNode;
};

export async function TextPage({ slug, title, crumbs, children }: TextPageProps) {
  const page = await findPage(slug);
  return (
    <>
      <PageHeader
        crumbs={[...crumbs, { label: page?.title ?? title }]}
        title={page?.title ?? title}
        lead={page?.summary || undefined}
      />
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-10 lg:px-6">
        {page ? (
          <article className="flex flex-col gap-4">
            <RichText text={page.body} />
          </article>
        ) : (
          <EmptyState
            title="This page is being prepared"
            description="The college is writing this page. Contact us if you need the information now."
            action={
              <LinkButton href="/contact" variant="outline">
                Contact us
              </LinkButton>
            }
          />
        )}
        {children}
      </div>
    </>
  );
}
