import Image from "next/image";
import Link from "next/link";
import { ArrowRight, Calendar, Clock, MapPin, Search } from "@/components/icons";
import { FinderForm } from "@/components/public/finder-form";
import { JsonLd } from "@/components/public/json-ld";
import { Section } from "@/components/public/page-header";
import { SchoolCard } from "@/components/public/school-card";
import { Accordion, AccordionItem } from "@/components/ui/accordion";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { applyHref } from "@/lib/navigation";
import { upcomingIntakes } from "@/lib/public/programmes";
import { settingsOf, telHref } from "@/lib/public/settings";
import { getSite } from "@/lib/public/site";
import { site as siteFacts } from "@/lib/site";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";

export default async function Home() {
  const site = await getSite();
  const settings = settingsOf(site);
  const intakes = upcomingIntakes(site, 4);
  const firstOpen = upcomingIntakes(site).find((u) => u.intake.status === "open");
  const faqs = site.faqs.filter((f) => !f.programme_slug).slice(0, 4);
  const values = [
    { label: "Motto", text: settings.motto },
    { label: "Mission", text: settings.mission },
    { label: "Vision", text: settings.vision },
  ].filter((v) => v.text);
  const programmeCount = (slug: string) => site.programmes.filter((p) => p.school_slug === slug).length;

  return (
    <>
      <JsonLd
        data={{
          "@context": "https://schema.org",
          "@type": "EducationalOrganization",
          name: siteFacts.name,
          logo: "/brand/dakota-icon.svg",
          ...(settings.address
            ? { address: { "@type": "PostalAddress", streetAddress: settings.address, addressCountry: "KE" } }
            : { address: { "@type": "PostalAddress", addressRegion: "Makueni", addressCountry: "KE" } }),
          ...(settings.phone ? { telephone: settings.phone } : {}),
          ...(settings.email ? { email: settings.email } : {}),
          ...(settings.social.length ? { sameAs: settings.social.map((s) => s.href) } : {}),
        }}
      />

      {/* Split hero (brief 5.1): words and actions first, artwork beside. */}
      <section aria-labelledby="hero-title" className="bg-ivory">
        <div className="mx-auto grid max-w-6xl items-center gap-8 px-4 py-10 lg:grid-cols-[1.2fr_1fr] lg:px-6 lg:py-16">
          <div className="flex flex-col gap-5">
            <h1 id="hero-title" className="text-4xl font-semibold text-navy-900 sm:text-5xl">
              Train for your career at Dakota Training College
            </h1>
            <p className="max-w-prose text-lg">
              Five schools on one campus in Makueni, Kenya: Medical and Health Sciences, Hospitality and
              Tourism, ICT, Business, and Pool Management.
            </p>
            <div className="flex flex-wrap gap-3">
              <LinkButton href={applyHref}>Apply now</LinkButton>
              <LinkButton
                href="/programmes"
                variant="outline"
                icon={<Search aria-hidden className="size-5" />}
              >
                Find a course
              </LinkButton>
            </div>
            {firstOpen ? (
              <p className="flex items-start gap-2 text-ink">
                <Calendar aria-hidden className="mt-1 size-5 shrink-0 text-royal" />
                <span>
                  Applications are open for the {firstOpen.intake.name} intake, starting{" "}
                  {formatDate(firstOpen.intake.starts_on)}
                  {firstOpen.intake.application_deadline
                    ? `. Apply by ${formatDate(firstOpen.intake.application_deadline)}.`
                    : "."}
                </span>
              </p>
            ) : null}
          </div>
          {/* Supplied tagline artwork with "Progression" shortened to "Progress"
              from its own letter outlines (docs/brand/retagline.py, D-86). */}
          <Image
            src="/brand/dakota-logo-stacked-tagline-progress.svg"
            alt="Dakota Training College logo with the tagline Turning Potential into Progress"
            width={916}
            height={858}
            unoptimized
            priority
            className="mx-auto h-auto w-full max-w-xs sm:max-w-sm"
          />
        </div>
      </section>

      {/* Quick course search: a plain GET form, so it works without JavaScript. */}
      <section aria-labelledby="finder-title" className="on-dark bg-navy-900 text-white">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8 lg:px-6">
          <h2 id="finder-title" className="text-2xl font-semibold text-white">
            Find a course
          </h2>
          <FinderForm schools={site.schools} compact idPrefix="home-finder" />
        </div>
      </section>

      {site.schools.length ? (
        <Section id="schools" title="Our schools">
          <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {site.schools.map((school) => (
              <li key={school.slug} className="flex">
                <SchoolCard school={school} programmeCount={programmeCount(school.slug)} />
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {/* Only verified figures are published, each with its date (AC-16). */}
      {site.statistics.length ? (
        <Section id="why" title="Why Dakota" className="bg-white">
          <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {site.statistics.map((s) => (
              <li key={s.label}>
                <Card className="h-full">
                  <p className="font-serif text-4xl font-semibold text-navy-900">{s.value}</p>
                  <p className="font-semibold">{s.label}</p>
                  <p className="text-sm text-ink-muted">Last verified {formatDate(s.verified_on)}</p>
                </Card>
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {values.length ? (
        <Section id="values" title="What we stand for">
          <dl className="grid gap-4 md:grid-cols-3">
            {values.map((v) => (
              <Card key={v.label}>
                <dt className="font-serif text-xl font-semibold text-navy-900">{v.label}</dt>
                <dd className="whitespace-pre-line">{v.text}</dd>
              </Card>
            ))}
          </dl>
        </Section>
      ) : null}

      {intakes.length ? (
        <Section id="intakes" title="Upcoming intakes" className="bg-white">
          <ul className="grid gap-3 md:grid-cols-2">
            {intakes.map(({ programme, intake }) => (
              <li key={intake.id}>
                <Card className="relative h-full">
                  <p className="flex items-center gap-2 font-semibold text-royal">
                    <Calendar aria-hidden className="size-5" />
                    Starts {formatDate(intake.starts_on)}
                  </p>
                  <h3 className="text-lg font-semibold text-navy-900">
                    <Link
                      href={`/programmes/${programme.slug}`}
                      className="after:absolute after:inset-0 hover:underline"
                    >
                      {programme.title}
                    </Link>
                  </h3>
                  <p className="text-sm">
                    {intake.name} intake.{" "}
                    {intake.status === "open" ? "Applications open" : "Applications not open yet"}
                    {intake.application_deadline
                      ? `, deadline ${formatDate(intake.application_deadline)}`
                      : ""}
                    .
                  </p>
                </Card>
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {settings.address || settings.directions || settings.visitingHours ? (
        <Section id="campus" title="Visit the campus">
          <Card className="max-w-2xl">
            {settings.address ? (
              <p className="flex items-start gap-2">
                <MapPin aria-hidden className="mt-1 size-5 shrink-0 text-royal" />
                <span>{settings.address}</span>
              </p>
            ) : null}
            {settings.visitingHours ? (
              <p className="flex items-start gap-2">
                <Clock aria-hidden className="mt-1 size-5 shrink-0 text-royal" />
                <span>Visiting hours: {settings.visitingHours}</span>
              </p>
            ) : null}
            <Link
              href="/about#campus"
              className="inline-flex min-h-touch items-center gap-1 font-semibold text-royal underline"
            >
              Directions and campus details
              <ArrowRight aria-hidden className="size-4" />
            </Link>
          </Card>
        </Section>
      ) : null}

      {site.events.length || site.news.length ? (
        <Section id="news" title="News and events" className="bg-white">
          <div className="grid gap-8 md:grid-cols-2">
            {site.events.length ? (
              <div className="flex flex-col gap-3">
                <h3 className="text-xl font-semibold text-navy-900">Coming up</h3>
                <ul className="flex flex-col gap-3">
                  {site.events.map((e) => (
                    <li key={e.slug}>
                      <Card className="relative">
                        <p className="text-sm font-semibold text-royal">{formatNairobi(e.starts_at)}</p>
                        <h4 className="font-sans text-base font-semibold text-navy-900">
                          <Link
                            href={`/events/${e.slug}`}
                            className="after:absolute after:inset-0 hover:underline"
                          >
                            {e.title}
                          </Link>
                        </h4>
                        {e.location ? <p className="text-sm">{e.location}</p> : null}
                      </Card>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {site.news.length ? (
              <div className="flex flex-col gap-3">
                <h3 className="text-xl font-semibold text-navy-900">Latest news</h3>
                <ul className="flex flex-col gap-3">
                  {site.news.map((n) => (
                    <li key={n.slug}>
                      <Card className="relative">
                        {n.published_at ? (
                          <p className="text-sm text-ink-muted">{formatDate(n.published_at.slice(0, 10))}</p>
                        ) : null}
                        <h4 className="font-sans text-base font-semibold text-navy-900">
                          <Link
                            href={`/news/${n.slug}`}
                            className="after:absolute after:inset-0 hover:underline"
                          >
                            {n.title}
                          </Link>
                        </h4>
                        {n.summary ? <p className="text-sm">{n.summary}</p> : null}
                      </Card>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        </Section>
      ) : null}

      {faqs.length ? (
        <Section id="faqs" title="Questions applicants ask">
          <Accordion className="max-w-3xl">
            {faqs.map((f) => (
              <AccordionItem key={f.question} title={f.question}>
                <p className="whitespace-pre-line">{f.answer}</p>
              </AccordionItem>
            ))}
          </Accordion>
        </Section>
      ) : null}

      <section aria-labelledby="apply-title" className="on-dark bg-navy-900 text-white">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-10 sm:flex-row sm:items-center sm:justify-between lg:px-6">
          <div className="flex flex-col gap-2">
            <h2 id="apply-title" className="text-2xl font-semibold text-white sm:text-3xl">
              Ready to start?
            </h2>
            <p className="text-white/90">
              Choose a programme and intake, then apply.
              {settings.phone ? (
                <>
                  {" "}
                  Questions? Call{" "}
                  <a href={telHref(settings.phone)} className="underline underline-offset-4">
                    {settings.phone}
                  </a>
                  .
                </>
              ) : null}
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <LinkButton href={applyHref}>Apply now</LinkButton>
            <LinkButton href="/contact" variant="inverse">
              Ask a question
            </LinkButton>
          </div>
        </div>
      </section>
    </>
  );
}
