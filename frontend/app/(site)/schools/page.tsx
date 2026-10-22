import type { Metadata } from "next";
import { PageHeader } from "@/components/public/page-header";
import { SchoolCard } from "@/components/public/school-card";
import { EmptyState } from "@/components/ui/empty-state";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

export const metadata: Metadata = pageMetadata({
  title: "Our schools",
  description: "The five schools of Dakota Training College in Makueni and the programmes each one offers.",
  path: "/schools",
});

export default async function SchoolsPage() {
  const site = await getSite();
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Schools" }]}
        title="Our schools"
        lead="Every programme belongs to one of our schools. Choose a school to see what it teaches."
      />
      <div className="mx-auto w-full max-w-6xl px-4 py-10 lg:px-6">
        {site.schools.length ? (
          <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {site.schools.map((school) => (
              <li key={school.slug} className="flex">
                <SchoolCard
                  school={school}
                  programmeCount={site.programmes.filter((p) => p.school_slug === school.slug).length}
                />
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState
            title="School details are on their way"
            description="The college is preparing these pages. Contact us to ask about programmes in the meantime."
          />
        )}
      </div>
    </>
  );
}
