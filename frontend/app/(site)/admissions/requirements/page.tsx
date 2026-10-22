import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader, Section } from "@/components/public/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { levelLabels, levelOrder } from "@/lib/public/programmes";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

export const metadata: Metadata = pageMetadata({
  title: "Entry requirements",
  description: "Entry requirements for every Dakota Training College programme, by level.",
  path: "/admissions/requirements",
});

export default async function RequirementsPage() {
  const site = await getSite();
  const levels = levelOrder.filter((l) => site.programmes.some((p) => p.level === l));
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Admissions", href: "/admissions" },
          { label: "Entry requirements" },
        ]}
        title="Entry requirements"
        lead="What you need to join each programme. Each programme page has the full details."
      />
      {levels.length ? (
        levels.map((level, i) => (
          <Section
            key={level}
            id={level}
            title={levelLabels[level]}
            className={i % 2 ? "bg-white" : undefined}
          >
            <ul className="grid gap-4 md:grid-cols-2">
              {site.programmes
                .filter((p) => p.level === level)
                .map((p) => (
                  <li
                    key={p.slug}
                    className="flex flex-col gap-2 rounded-card border border-stone bg-white p-4"
                  >
                    <h3 className="text-lg font-semibold text-navy-900">
                      <Link href={`/programmes/${p.slug}#requirements`} className="hover:underline">
                        {p.title}
                      </Link>
                    </h3>
                    {p.min_kcse_grade ? (
                      <p>
                        Minimum KCSE mean grade: <strong>{p.min_kcse_grade}</strong>
                      </p>
                    ) : null}
                    <ul className="flex list-disc flex-col gap-1 pl-6 text-sm">
                      {p.requirements.map((r) => (
                        <li key={r.requirement}>{r.requirement}</li>
                      ))}
                    </ul>
                  </li>
                ))}
            </ul>
          </Section>
        ))
      ) : (
        <div className="mx-auto w-full max-w-6xl px-4 py-10 lg:px-6">
          <EmptyState
            title="Requirements are on their way"
            description="The college has not published its programmes yet. Contact us to ask what you need."
          />
        </div>
      )}
    </>
  );
}
