import type { Metadata } from "next";
import { FounderProfile } from "@/components/public/founders";
import { PageHeader } from "@/components/public/page-header";
import { LinkButton } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

const title = "Our founders";

// Like the text pages (D-82): the address always exists, and until a
// profile is published it says so and is not indexed.
export async function generateMetadata(): Promise<Metadata> {
  const site = await getSite();
  if (!site.founders.length) return { title, robots: { index: false, follow: true } };
  return pageMetadata({
    title,
    description: `The people who founded Dakota Training College: ${site.founders.map((f) => f.name).join(", ")}.`,
    path: "/about/founders",
  });
}

export default async function FoundersPage() {
  const site = await getSite();
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "About", href: "/about" }, { label: title }]}
        title={title}
        lead={site.founders.length ? "The people who started Dakota Training College, and why." : undefined}
      />
      <div className="bg-ivory-glow">
        <div className="mx-auto flex w-full max-w-6xl flex-col gap-16 px-4 py-12 lg:px-6 lg:py-16">
          {site.founders.length ? (
            site.founders.map((f, i) => <FounderProfile key={f.name} founder={f} reverse={i % 2 === 1} />)
          ) : (
            <EmptyState
              title="This page is being prepared"
              description="The college is writing about its founders. Contact us if you would like to know more now."
              action={
                <LinkButton href="/contact" variant="outline">
                  Contact us
                </LinkButton>
              }
            />
          )}
        </div>
      </div>
    </>
  );
}
