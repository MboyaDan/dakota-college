import type { Metadata } from "next";
import { connection } from "next/server";
import { Suspense } from "react";
import { Mail, MessageCircle, Phone } from "@/components/icons";
import { PageHeader } from "@/components/public/page-header";
import { Alert } from "@/components/ui/alert";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { intakesTakingApplications } from "@/lib/public/programmes";
import { settingsOf, telHref, whatsappHref } from "@/lib/public/settings";
import { getSite, getSiteOrFallback } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import { formatDate } from "@/lib/staff/datetime";

// Start of the application path (AC-03). Programme pages link here with
// ?programme= and ?intake=; the primary action starts the online form.

export const metadata: Metadata = pageMetadata({
  title: "Apply",
  description: "How to apply to Dakota Training College.",
  path: "/apply",
});

export default async function ApplyPage({ searchParams }: PageProps<"/apply">) {
  const site = await getSite();
  const s = settingsOf(site);
  const whatsapp = whatsappHref(s.whatsapp, "Hello Dakota Training College, I would like to apply.");
  const email = s.admissionsEmail || s.email;

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Apply" }]}
        title="Apply to Dakota Training College"
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10 lg:px-6">
        <Suspense fallback={null}>
          <Choice searchParams={searchParams} />
        </Suspense>
        <Alert title="You can start your application now">
          <p>
            Fill in the form online; it saves as you go. Uploading documents and submitting are the last part
            of the form and open soon. The admissions office can also help you apply.
          </p>
        </Alert>
        <Card>
          <h2 className="text-xl font-semibold text-navy-900">Contact admissions</h2>
          <ul className="flex flex-col gap-3 [&_a]:text-royal [&_a]:underline [&_a]:underline-offset-4">
            {s.phone ? (
              <li className="flex items-center gap-2">
                <Phone aria-hidden className="size-5 text-royal" />
                <a href={telHref(s.phone)}>Call {s.phone}</a>
              </li>
            ) : null}
            {whatsapp ? (
              <li className="flex items-center gap-2">
                <MessageCircle aria-hidden className="size-5 text-royal" />
                <a href={whatsapp} target="_blank" rel="noopener noreferrer">
                  Message us on WhatsApp <span className="sr-only">(opens WhatsApp)</span>
                </a>
              </li>
            ) : null}
            {email ? (
              <li className="flex items-center gap-2">
                <Mail aria-hidden className="size-5 text-royal" />
                <a href={`mailto:${email}`}>{email}</a>
              </li>
            ) : null}
          </ul>
          <div className="flex flex-wrap gap-3 pt-2">
            <LinkButton href="/contact" variant="outline">
              Send a message
            </LinkButton>
            <LinkButton href="/programmes" variant="outline">
              Browse programmes
            </LinkButton>
          </div>
        </Card>
        <Card>
          <h2 className="text-xl font-semibold text-navy-900">Your applicant account</h2>
          <p>
            Sign in with a link sent to your email, with no password, to see your applications and their
            status.
          </p>
          <div className="flex flex-wrap gap-3 pt-2">
            <LinkButton href="/apply/start" variant="secondary" reload>
              Email me a sign-in link
            </LinkButton>
            <LinkButton href="/apply/resume" variant="outline" reload>
              I have a reference number
            </LinkButton>
          </div>
        </Card>
      </div>
    </>
  );
}

/**
 * The programme and intake chosen on the programme page, with the one
 * primary action: start the application. Without a choice, how to begin.
 */
async function Choice({ searchParams }: Pick<PageProps<"/apply">, "searchParams">) {
  const [site, params] = await Promise.all([getSiteOrFallback(), searchParams]);
  // Which intakes are open depends on today's date, so this part renders per request.
  await connection();
  const programme = site.programmes.find((p) => p.slug === params.programme);
  if (!programme) {
    return (
      <Card>
        <h2 className="text-xl font-semibold text-navy-900">How to apply online</h2>
        <ol className="list-decimal space-y-1 pl-5">
          <li>Find the programme you want and choose Apply on its page.</li>
          <li>Enter your email address; we send you a sign-in link, with no password to remember.</li>
          <li>Fill in the form. It saves as you go, so you can stop and come back.</li>
        </ol>
        <div className="pt-2">
          <LinkButton href="/programmes">Find a course</LinkButton>
        </div>
      </Card>
    );
  }
  const intake = programme.intakes.find((i) => i.id === params.intake);
  const open = intakesTakingApplications(programme);
  const q = new URLSearchParams({ programme: programme.slug });
  if (intake && open.some((i) => i.id === intake.id)) q.set("intake", intake.id);
  return (
    <Card>
      <h2 className="text-xl font-semibold text-navy-900">Your choice</h2>
      <p>
        <strong>{programme.title}</strong>
        {intake ? `, ${intake.name} intake starting ${formatDate(intake.starts_on)}` : null}
      </p>
      {open.length > 0 ? (
        <div className="pt-2">
          <LinkButton href={`/apply/new?${q}`} reload>
            Start your application
          </LinkButton>
        </div>
      ) : (
        <p>
          This programme is not taking applications now. Contact the admissions office to ask about the next
          intake.
        </p>
      )}
    </Card>
  );
}
