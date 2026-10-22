import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { FinderForm } from "@/components/public/finder-form";
import { PageHeader } from "@/components/public/page-header";
import { ProgrammeCard } from "@/components/public/programme-card";
import { EmptyState } from "@/components/ui/empty-state";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { filterProgrammes, hasFilters, parseFilters } from "@/lib/public/programmes";
import { getSiteOrFallback } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

export const metadata: Metadata = pageMetadata({
  title: "Course finder",
  description:
    "Search Dakota Training College programmes by school, level, length and KCSE grade, and see fees, entry requirements and intakes.",
  path: "/programmes",
});

export default function ProgrammesPage({ searchParams }: PageProps<"/programmes">) {
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Course finder" }]}
        title="Course finder"
        lead="Search our programmes, or narrow them down by school, level, length and your KCSE grade."
      />
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-8 lg:px-6">
        <Suspense fallback={<FinderSkeleton />}>
          <FinderResults searchParams={searchParams} />
        </Suspense>
      </div>
    </>
  );
}

// Filters come from the URL, so results render per request; the content
// itself is cached, with the bundled copy as fallback (AC-15).
async function FinderResults({ searchParams }: Pick<PageProps<"/programmes">, "searchParams">) {
  const [site, params] = await Promise.all([getSiteOrFallback(), searchParams]);
  const filters = parseFilters(params);
  const results = filterProgrammes(site.programmes, filters);
  const schoolBySlug = new Map(site.schools.map((s) => [s.slug, s]));
  const count = results.length === 1 ? "1 programme" : `${results.length} programmes`;

  return (
    <>
      <div className="rounded-card border border-stone bg-white p-5">
        <FinderForm schools={site.schools} filters={filters} />
      </div>
      <section aria-labelledby="results-title" className="flex flex-col gap-4">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h2 id="results-title" className="text-2xl font-semibold text-navy-900">
            {hasFilters(filters) ? `${count} match your search` : `All ${count}`}
          </h2>
          {hasFilters(filters) ? (
            <Link
              href="/programmes"
              className="inline-flex min-h-touch items-center font-semibold text-royal underline"
            >
              Clear filters
            </Link>
          ) : null}
        </div>
        {results.length ? (
          <ul className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {results.map((p) => (
              <li key={p.slug} className="flex">
                <ProgrammeCard programme={p} school={schoolBySlug.get(p.school_slug)} />
              </li>
            ))}
          </ul>
        ) : site.programmes.length ? (
          <EmptyState
            title="No programmes match"
            description="Try fewer filters or a different word. You can also ask us which programme suits you."
            action={
              <Link href="/programmes" className="font-semibold text-royal underline">
                Show all programmes
              </Link>
            }
          />
        ) : (
          <EmptyState
            title="Programmes are on their way"
            description="The college has not published its programmes yet. Contact us to ask about courses and intakes."
          />
        )}
      </section>
    </>
  );
}

function FinderSkeleton() {
  return (
    <Loading label="Loading programmes">
      <Skeleton className="h-40 w-full" />
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Skeleton className="h-56" />
        <Skeleton className="h-56" />
        <Skeleton className="h-56" />
      </div>
    </Loading>
  );
}
