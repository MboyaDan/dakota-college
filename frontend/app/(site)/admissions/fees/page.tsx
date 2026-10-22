import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader, Section } from "@/components/public/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { feeText } from "@/lib/public/programmes";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

export const metadata: Metadata = pageMetadata({
  title: "Fees",
  description: "Published fees for every Dakota Training College programme, in Kenya shillings.",
  path: "/admissions/fees",
});

export default async function FeesPage() {
  const site = await getSite();
  const schools = site.schools.filter((s) => site.programmes.some((p) => p.school_slug === s.slug));
  const feeFaqs = site.faqs.some((f) => f.category_slug === "fees");
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Admissions", href: "/admissions" },
          { label: "Fees" },
        ]}
        title="Fees"
        lead={
          <>
            Every fee the college publishes, in Kenya shillings (KES).
            {feeFaqs ? (
              <>
                {" "}
                For how to pay, see{" "}
                <Link href="/faqs#fees" className="text-royal underline">
                  questions about fees
                </Link>
                .
              </>
            ) : null}
          </>
        }
      />
      {schools.length ? (
        schools.map((school, i) => (
          <Section
            key={school.slug}
            id={school.slug}
            title={school.name}
            className={i % 2 ? "bg-white" : undefined}
          >
            <div className="flex flex-col gap-6">
              {site.programmes
                .filter((p) => p.school_slug === school.slug)
                .map((p) => {
                  const intakes = new Map(p.intakes.map((n) => [n.id, n.name]));
                  return (
                    <div key={p.slug} className="flex flex-col gap-2">
                      <h3 className="text-lg font-semibold text-navy-900">
                        <Link href={`/programmes/${p.slug}#fees`} className="hover:underline">
                          {p.title}
                        </Link>
                      </h3>
                      <ul className="divide-y divide-stone rounded-card border border-stone bg-white">
                        {p.fees.map((f, j) => (
                          <li
                            key={j}
                            className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-baseline sm:justify-between"
                          >
                            <span className="font-semibold">
                              {f.label}
                              {f.intake_id && intakes.get(f.intake_id) ? (
                                <span className="font-normal text-ink-muted">
                                  {" "}
                                  ({intakes.get(f.intake_id)} intake only)
                                </span>
                              ) : null}
                            </span>
                            <span className="tabular">{feeText(f)}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  );
                })}
            </div>
          </Section>
        ))
      ) : (
        <div className="mx-auto w-full max-w-6xl px-4 py-10 lg:px-6">
          <EmptyState
            title="Fees are on their way"
            description="The college has not published its fees yet. Contact us to ask about the cost of a programme."
          />
        </div>
      )}
    </>
  );
}
