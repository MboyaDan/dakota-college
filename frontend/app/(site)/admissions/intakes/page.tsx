import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "@/components/icons";
import { PageHeader, Section } from "@/components/public/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusTag } from "@/components/ui/tag";
import { applyHref } from "@/lib/navigation";
import { upcomingIntakes, type UpcomingIntake } from "@/lib/public/programmes";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import { formatDate } from "@/lib/staff/datetime";

export const metadata: Metadata = pageMetadata({
  title: "Intake calendar",
  description: "When Dakota Training College programmes start, and the dates to apply by.",
  path: "/admissions/intakes",
});

const monthName = new Intl.DateTimeFormat("en-KE", { month: "long", year: "numeric", timeZone: "UTC" });

/** Intakes grouped by the month they start, soonest first. */
function byMonth(intakes: UpcomingIntake[]) {
  const months = new Map<string, UpcomingIntake[]>();
  for (const u of intakes) {
    const key = u.intake.starts_on.slice(0, 7);
    months.set(key, [...(months.get(key) ?? []), u]);
  }
  return [...months.entries()];
}

export default async function IntakesPage() {
  const site = await getSite();
  const months = byMonth(upcomingIntakes(site));
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Admissions", href: "/admissions" },
          { label: "Intake calendar" },
        ]}
        title="Intake calendar"
        lead="Programmes and the dates they start. Apply before the deadline for the intake you want."
      />
      {months.length ? (
        months.map(([month, items], i) => (
          <Section
            key={month}
            id={`m-${month}`}
            title={`Starting in ${monthName.format(new Date(`${month}-01T00:00:00Z`))}`}
            className={i % 2 ? "bg-white" : undefined}
          >
            <ul className="grid gap-3 md:grid-cols-2">
              {items.map(({ programme, intake }) => (
                <li
                  key={intake.id}
                  className="flex flex-col gap-2 rounded-card border border-stone bg-white p-4"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <h3 className="text-lg font-semibold text-navy-900">
                      <Link href={`/programmes/${programme.slug}#intakes`} className="hover:underline">
                        {programme.title}
                      </Link>
                    </h3>
                    {intake.status === "open" ? (
                      <StatusTag status="success">Applications open</StatusTag>
                    ) : (
                      <StatusTag status="info">Opening soon</StatusTag>
                    )}
                  </div>
                  <p className="text-sm">
                    {intake.name} intake starts <strong>{formatDate(intake.starts_on)}</strong>
                    {intake.application_deadline ? (
                      <>
                        . Apply by <strong>{formatDate(intake.application_deadline)}</strong>
                      </>
                    ) : null}
                    .
                  </p>
                  {intake.status === "open" ? (
                    <Link
                      href={`${applyHref}?${new URLSearchParams({ programme: programme.slug, intake: intake.id })}`}
                      className="inline-flex min-h-touch items-center gap-1 font-semibold text-royal underline"
                    >
                      Apply for this intake <ArrowRight aria-hidden className="size-4" />
                    </Link>
                  ) : null}
                </li>
              ))}
            </ul>
          </Section>
        ))
      ) : (
        <div className="mx-auto w-full max-w-6xl px-4 py-10 lg:px-6">
          <EmptyState
            title="No intakes published yet"
            description="The college will publish its next intake dates here. Contact us to ask when the next one starts."
          />
        </div>
      )}
    </>
  );
}
