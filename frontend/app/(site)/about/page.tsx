import type { Metadata } from "next";
import { Clock, ExternalLink, MapPin } from "@/components/icons";
import { FoundersTeaser } from "@/components/public/founders";
import { PageHeader, Prose, Section } from "@/components/public/page-header";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { settingsOf } from "@/lib/public/settings";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import { formatDate } from "@/lib/staff/datetime";

export const metadata: Metadata = pageMetadata({
  title: "About the college",
  description: "Dakota Training College in Makueni, Kenya: our schools, mission, registration and campus.",
  path: "/about",
});

export default async function AboutPage() {
  const site = await getSite();
  const s = settingsOf(site);
  const values = [
    { label: "Motto", text: s.motto },
    { label: "Mission", text: s.mission },
    { label: "Vision", text: s.vision },
  ].filter((v) => v.text);

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "About" }]}
        title="About Dakota Training College"
        lead="A training college in Makueni, Kenya, with five schools on one campus."
      />

      {site.schools.length ? (
        <Section id="schools" title="Our schools">
          <ul className="flex max-w-prose list-disc flex-col gap-1 pl-6">
            {site.schools.map((school) => (
              <li key={school.slug}>
                <a href={`/schools/${school.slug}`} className="text-royal underline underline-offset-4">
                  {school.name}
                </a>
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {values.length ? (
        <Section id="values" title="Mission and values" className="bg-white">
          <dl className="grid gap-4 md:grid-cols-3">
            {values.map((v) => (
              <Card key={v.label}>
                <dt className="font-serif text-xl font-semibold text-navy-900">{v.label}</dt>
                <dd>
                  <Prose text={v.text} />
                </dd>
              </Card>
            ))}
          </dl>
        </Section>
      ) : null}

      {/* Profiles are published only with each founder's consent. */}
      {site.founders.length ? (
        <Section id="founders" title={site.founders.length === 1 ? "Our founder" : "Our founders"}>
          <FoundersTeaser founders={site.founders} />
        </Section>
      ) : null}

      {/* Only verified registrations are published (AC-16). Nothing shows until then. */}
      {site.accreditations.length ? (
        <Section id="registration" title="Registration and accreditation">
          <ul className="grid gap-4 md:grid-cols-2">
            {site.accreditations.map((a) => (
              <li key={`${a.body}-${a.reference}`}>
                <Card className="h-full">
                  <p className="font-semibold text-navy-900">{a.body}</p>
                  <p>
                    Reference <span className="font-mono">{a.reference}</span>
                  </p>
                  {a.description ? <Prose text={a.description} /> : null}
                  <p className="text-sm text-ink-muted">Last verified {formatDate(a.verified_on)}</p>
                </Card>
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {/* One campus, so one section rather than tabs (brief 5.1). */}
      <Section id="campus" title="Campus and visiting" className="bg-white">
        {s.address || s.directions || s.visitingHours ? (
          <Card className="max-w-2xl">
            {s.address ? (
              <p className="flex items-start gap-2">
                <MapPin aria-hidden className="mt-1 size-5 shrink-0 text-royal" />
                <span>{s.address}</span>
              </p>
            ) : null}
            {s.visitingHours ? (
              <p className="flex items-start gap-2">
                <Clock aria-hidden className="mt-1 size-5 shrink-0 text-royal" />
                <span>Visiting hours: {s.visitingHours}</span>
              </p>
            ) : null}
            {s.directions ? (
              <div className="flex flex-col gap-1">
                <h3 className="font-semibold text-navy-900">How to get here</h3>
                <Prose text={s.directions} />
              </div>
            ) : null}
            {s.mapUrl ? (
              <a
                href={s.mapUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex min-h-touch items-center gap-1 font-semibold text-royal underline"
              >
                Get directions <ExternalLink aria-hidden className="size-4" />
                <span className="sr-only">(opens a map)</span>
              </a>
            ) : null}
          </Card>
        ) : (
          <p className="max-w-prose">
            Our campus is in Makueni, Kenya. Contact us for directions and to arrange a visit.
          </p>
        )}
        <div>
          <LinkButton href="/contact" variant="outline">
            Contact us
          </LinkButton>
        </div>
      </Section>
    </>
  );
}
