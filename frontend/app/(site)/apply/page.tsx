import type { Metadata } from "next";
import { Suspense } from "react";
import { Mail, MessageCircle, Phone } from "@/components/icons";
import { PageHeader } from "@/components/public/page-header";
import { Alert } from "@/components/ui/alert";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { settingsOf, telHref, whatsappHref } from "@/lib/public/settings";
import { getSite, getSiteOrFallback } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import { formatDate } from "@/lib/staff/datetime";

// Interim start of the application path (AC-03). The online application form
// replaces the body of this page in Stage 9; until then it says so plainly
// and shows how to apply through the admissions office.

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
        <Alert title="Online applications are not open yet">
          <p>
            The online application form is being prepared. Until it opens, apply through the admissions
            office: tell them the programme and intake you want, and they will guide you.
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
            <LinkButton href="/contact">Send a message</LinkButton>
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
            <LinkButton href="/apply/start" variant="secondary">
              Email me a sign-in link
            </LinkButton>
            <LinkButton href="/apply/resume" variant="outline">
              I have a reference number
            </LinkButton>
          </div>
        </Card>
      </div>
    </>
  );
}

/** Repeats the programme and intake chosen on the programme page. */
async function Choice({ searchParams }: Pick<PageProps<"/apply">, "searchParams">) {
  const [site, params] = await Promise.all([getSiteOrFallback(), searchParams]);
  const programme = site.programmes.find((p) => p.slug === params.programme);
  if (!programme) return null;
  const intake = programme.intakes.find((i) => i.id === params.intake);
  return (
    <Card>
      <h2 className="text-xl font-semibold text-navy-900">Your choice</h2>
      <p>
        <strong>{programme.title}</strong>
        {intake ? `, ${intake.name} intake starting ${formatDate(intake.starts_on)}` : null}
      </p>
    </Card>
  );
}
