import type { Metadata } from "next";
import { connection } from "next/server";
import Link from "next/link";
import { Suspense } from "react";
import { NewApplication } from "@/components/apply/new-application";
import { PageHeader } from "@/components/public/page-header";
import { Alert } from "@/components/ui/alert";
import { LinkButton } from "@/components/ui/button";
import { intakesTakingApplications } from "@/lib/public/programmes";
import { getSiteOrFallback } from "@/lib/public/site";

export const metadata: Metadata = {
  title: "Start an application",
  robots: { index: false, follow: false },
};

async function Body({ searchParams }: Pick<PageProps<"/apply/new">, "searchParams">) {
  const [site, params] = await Promise.all([getSiteOrFallback(), searchParams]);
  // Which intakes are open depends on today's date, so this renders per request.
  await connection();
  const programme = site.programmes.find((p) => p.slug === params.programme);
  if (!programme) {
    return (
      <Alert tone="info" title="Choose a programme first">
        <p>Find the programme you want, then choose Apply on its page.</p>
        <p className="mt-3">
          <LinkButton href="/programmes">Find a course</LinkButton>
        </p>
      </Alert>
    );
  }
  const intakes = intakesTakingApplications(programme);
  if (intakes.length === 0) {
    return (
      <Alert tone="warning" title="This programme is not taking applications now">
        See the{" "}
        <Link href={`/programmes/${programme.slug}`} className="text-royal underline">
          programme page
        </Link>{" "}
        for its next intake, or{" "}
        <Link href="/contact" className="text-royal underline">
          contact the admissions office
        </Link>
        .
      </Alert>
    );
  }
  return (
    <NewApplication
      programme={{ slug: programme.slug, title: programme.title }}
      intakes={intakes}
      chosen={typeof params.intake === "string" ? params.intake : undefined}
    />
  );
}

export default function NewApplicationPage({ searchParams }: PageProps<"/apply/new">) {
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Apply", href: "/apply" },
          { label: "Start an application" },
        ]}
        title="Start an application"
      />
      <div className="mx-auto flex w-full max-w-xl flex-col gap-6 px-4 py-10 lg:px-6">
        <Suspense fallback={null}>
          <Body searchParams={searchParams} />
        </Suspense>
      </div>
    </>
  );
}
