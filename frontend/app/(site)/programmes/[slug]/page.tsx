import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { ArrowRight, Calendar, Clock, GraduationCap, MapPin } from "@/components/icons";
import { JsonLd } from "@/components/public/json-ld";
import { PageHeader, Prose, Section } from "@/components/public/page-header";
import { Accordion, AccordionItem } from "@/components/ui/accordion";
import { LinkButton } from "@/components/ui/button";
import { StatusTag, Tag } from "@/components/ui/tag";
import { centsToInput } from "@/lib/money";
import { applyHref } from "@/lib/navigation";
import {
  durationText,
  feeText,
  levelLabels,
  modeLabels,
  nextIntake,
  tuitionFee,
} from "@/lib/public/programmes";
import { getSite } from "@/lib/public/site";
import type { PublicIntake, PublicProgramme } from "@/lib/public/types";
import { pageMetadata } from "@/lib/seo";
import { site as siteFacts } from "@/lib/site";
import { resolveSiteUrl } from "@/lib/site-url";
import { formatDate } from "@/lib/staff/datetime";

async function findProgramme(slug: string) {
  const site = await getSite();
  const programme = site.programmes.find((p) => p.slug === slug);
  const school = programme && site.schools.find((s) => s.slug === programme.school_slug);
  const faqs = programme ? site.faqs.filter((f) => f.programme_slug === programme.slug) : [];
  return { programme, school, faqs };
}

// With Cache Components the list must not be empty, so an empty site
// prerenders one placeholder that answers 404. Programmes published after the
// build render on first visit and are then cached.
export async function generateStaticParams() {
  const site = await getSite();
  return site.programmes.length ? site.programmes.map((p) => ({ slug: p.slug })) : [{ slug: "none" }];
}

export async function generateMetadata({ params }: PageProps<"/programmes/[slug]">): Promise<Metadata> {
  const { programme } = await findProgramme((await params).slug);
  if (!programme) return {};
  return pageMetadata({
    title: programme.seo_title || programme.title,
    description:
      programme.seo_description ||
      programme.summary ||
      `${levelLabels[programme.level]} at Dakota Training College: what you learn, entry requirements, fees and intakes.`,
    path: `/programmes/${programme.slug}`,
  });
}

function applyLink(programme: PublicProgramme, intake?: PublicIntake) {
  const q = new URLSearchParams({ programme: programme.slug });
  if (intake) q.set("intake", intake.id);
  return `${applyHref}?${q}`;
}

function Fact({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 rounded-control bg-ivory p-3">
      <dt className="flex items-center gap-2 text-sm text-ink-muted">
        <span className="text-royal">{icon}</span>
        {label}
      </dt>
      <dd className="pl-7 font-semibold text-ink">{children}</dd>
    </div>
  );
}

const requirementGroups = [
  { kind: "academic", title: "Academic" },
  { kind: "document", title: "Documents" },
  { kind: "other", title: "Other" },
] as const;

export default async function ProgrammePage({ params }: PageProps<"/programmes/[slug]">) {
  const { programme, school, faqs } = await findProgramme((await params).slug);
  if (!programme) notFound();

  const intake = nextIntake(programme);
  const openIntake = programme.intakes.find((i) => i.status === "open");
  const tuition = tuitionFee(programme);
  const duration = durationText(programme);
  const intakeName = new Map(programme.intakes.map((i) => [i.id, i.name]));
  const base = resolveSiteUrl(process.env.SITE_URL);

  return (
    <>
      <JsonLd
        data={{
          "@context": "https://schema.org",
          "@type": "Course",
          name: programme.title,
          description: programme.seo_description || programme.summary || programme.title,
          url: new URL(`/programmes/${programme.slug}`, base).href,
          inLanguage: "en",
          educationalCredentialAwarded: levelLabels[programme.level],
          ...(programme.duration_months ? { timeRequired: `P${programme.duration_months}M` } : {}),
          provider: { "@type": "EducationalOrganization", name: siteFacts.name, url: base.href },
          ...(programme.intakes.length
            ? {
                hasCourseInstance: programme.intakes.map((i) => ({
                  "@type": "CourseInstance",
                  name: `${i.name} intake`,
                  courseMode: programme.mode === "blended" ? "Blended" : "Onsite",
                  startDate: i.starts_on,
                  location: { "@type": "Place", name: siteFacts.name, address: siteFacts.location },
                })),
              }
            : {}),
          ...(tuition
            ? {
                offers: {
                  "@type": "Offer",
                  category: "Paid",
                  price: centsToInput(tuition.amount_cents),
                  priceCurrency: "KES",
                  description: `Tuition ${feeText(tuition)}`,
                },
              }
            : {}),
        }}
      />

      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Course finder", href: "/programmes" },
          ...(school ? [{ label: school.short_name, href: `/schools/${school.slug}` }] : []),
          { label: programme.title },
        ]}
        title={programme.title}
        lead={programme.summary || undefined}
      >
        <div className="flex flex-wrap gap-2">
          <Tag tone="navy">{levelLabels[programme.level]}</Tag>
          {school ? <Tag school={school.accent_key}>{school.short_name}</Tag> : null}
        </div>
        <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {duration ? (
            <Fact icon={<Clock aria-hidden className="size-5" />} label="How long">
              {duration}
            </Fact>
          ) : null}
          <Fact icon={<MapPin aria-hidden className="size-5" />} label="Study mode">
            {modeLabels[programme.mode]}
          </Fact>
          <Fact icon={<Calendar aria-hidden className="size-5" />} label="Next intake">
            {intake ? `${intake.name}, starts ${formatDate(intake.starts_on)}` : "Not announced yet"}
          </Fact>
          {tuition ? (
            <Fact icon={<GraduationCap aria-hidden className="size-5" />} label="Tuition">
              {feeText(tuition)}
            </Fact>
          ) : null}
        </dl>
        <div className="flex flex-wrap items-center gap-3 pt-2">
          <LinkButton href={applyLink(programme, openIntake)}>Apply for this programme</LinkButton>
          <Link
            href={`/contact?programme=${programme.slug}`}
            className="inline-flex min-h-touch items-center font-semibold text-royal underline"
          >
            Ask a question about it
          </Link>
        </div>
      </PageHeader>

      <nav aria-label="On this page" className="border-b border-stone bg-white">
        <ul className="mx-auto flex max-w-6xl gap-x-5 overflow-x-auto px-4 text-sm font-semibold lg:px-6">
          {[
            ["learn", "What you will learn"],
            ["requirements", "Entry requirements"],
            ["fees", "Fees"],
            ["intakes", "Intakes"],
            ["apply", "How to apply"],
          ].map(([id, label]) => (
            <li key={id} className="shrink-0">
              <a href={`#${id}`} className="inline-flex min-h-touch items-center text-royal hover:underline">
                {label}
              </a>
            </li>
          ))}
        </ul>
      </nav>

      <Section id="learn" title="What you will learn">
        <Prose text={programme.what_you_learn} />
        {programme.overview ? (
          <>
            <h3 className="text-xl font-semibold text-navy-900">About the programme</h3>
            <Prose text={programme.overview} />
          </>
        ) : null}
      </Section>

      <Section id="requirements" title="Entry requirements" className="bg-white">
        {programme.min_kcse_grade ? (
          <p className="text-lg">
            Minimum KCSE mean grade: <strong>{programme.min_kcse_grade}</strong>
          </p>
        ) : null}
        {requirementGroups.map(({ kind, title }) => {
          const items = programme.requirements.filter((r) => r.kind === kind);
          if (!items.length) return null;
          return (
            <div key={kind} className="flex flex-col gap-2">
              <h3 className="text-xl font-semibold text-navy-900">{title}</h3>
              <ul className="flex max-w-prose list-disc flex-col gap-1 pl-6">
                {items.map((r) => (
                  <li key={r.requirement}>{r.requirement}</li>
                ))}
              </ul>
            </div>
          );
        })}
      </Section>

      <Section id="fees" title="Fees">
        {programme.fees.length ? (
          <>
            {/* Phones: one card per fee line; wider screens: a table. */}
            <ul className="flex flex-col gap-3 sm:hidden">
              {programme.fees.map((f, i) => (
                <li key={i} className="rounded-card border border-stone bg-white p-4">
                  <p className="font-semibold">{f.label}</p>
                  {f.intake_id && intakeName.get(f.intake_id) ? (
                    <p className="text-sm text-ink-muted">{intakeName.get(f.intake_id)} intake only</p>
                  ) : null}
                  <p className="tabular text-lg">{feeText(f)}</p>
                  {f.notes ? <p className="text-sm">{f.notes}</p> : null}
                </li>
              ))}
            </ul>
            <div className="hidden overflow-x-auto rounded-card border border-stone bg-white sm:block">
              <table className="w-full text-left">
                <caption className="sr-only">Fees for {programme.title}, in Kenya shillings</caption>
                <thead className="bg-ivory text-sm">
                  <tr>
                    <th scope="col" className="px-4 py-3">
                      Fee
                    </th>
                    <th scope="col" className="px-4 py-3">
                      Amount
                    </th>
                    <th scope="col" className="px-4 py-3">
                      Notes
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone">
                  {programme.fees.map((f, i) => (
                    <tr key={i}>
                      <th scope="row" className="px-4 py-3 font-semibold">
                        {f.label}
                        {f.intake_id && intakeName.get(f.intake_id) ? (
                          <span className="block text-sm font-normal text-ink-muted">
                            {intakeName.get(f.intake_id)} intake only
                          </span>
                        ) : null}
                      </th>
                      <td className="px-4 py-3 tabular whitespace-nowrap">{feeText(f)}</td>
                      <td className="px-4 py-3 text-sm">{f.notes}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ) : (
          <p>The college has not published fees for this programme yet. Ask us for the current fees.</p>
        )}
      </Section>

      <Section id="intakes" title="Intakes" className="bg-white">
        {programme.intakes.length ? (
          <ul className="grid gap-3 md:grid-cols-2">
            {programme.intakes.map((i) => (
              <li key={i.id} className="flex flex-col gap-2 rounded-card border border-stone p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-lg font-semibold text-navy-900">{i.name} intake</h3>
                  {i.status === "open" ? (
                    <StatusTag status="success">Applications open</StatusTag>
                  ) : (
                    <StatusTag status="info">Opening soon</StatusTag>
                  )}
                </div>
                <dl className="grid gap-1 text-sm">
                  <div>
                    <dt className="inline">Starts: </dt>
                    <dd className="inline font-semibold">{formatDate(i.starts_on)}</dd>
                  </div>
                  {i.applications_open_on ? (
                    <div>
                      <dt className="inline">Applications open: </dt>
                      <dd className="inline font-semibold">{formatDate(i.applications_open_on)}</dd>
                    </div>
                  ) : null}
                  {i.application_deadline ? (
                    <div>
                      <dt className="inline">Apply by: </dt>
                      <dd className="inline font-semibold">{formatDate(i.application_deadline)}</dd>
                    </div>
                  ) : null}
                </dl>
                {i.status === "open" ? (
                  <Link
                    href={applyLink(programme, i)}
                    className="inline-flex min-h-touch items-center gap-1 font-semibold text-royal underline"
                  >
                    Apply for the {i.name} intake
                    <ArrowRight aria-hidden className="size-4" />
                  </Link>
                ) : null}
              </li>
            ))}
          </ul>
        ) : (
          <p>No upcoming intake is published yet. Contact us to ask when the next one starts.</p>
        )}
      </Section>

      {programme.career_outcomes ? (
        <Section id="careers" title="Where it can lead">
          <Prose text={programme.career_outcomes} />
        </Section>
      ) : null}

      {faqs.length ? (
        <Section id="questions" title="Questions about this programme" className="bg-white">
          <Accordion className="max-w-3xl">
            {faqs.map((f) => (
              <AccordionItem key={f.question} title={f.question}>
                <p className="whitespace-pre-line">{f.answer}</p>
              </AccordionItem>
            ))}
          </Accordion>
        </Section>
      ) : null}

      <Section id="apply" title="How to apply">
        <ol className="flex max-w-prose list-decimal flex-col gap-2 pl-6">
          <li>Check that you meet the entry requirements above.</li>
          <li>Choose the intake you want to join.</li>
          <li>Fill in the application form. Have your documents ready to upload.</li>
        </ol>
        <div className="flex flex-wrap gap-3">
          <LinkButton href={applyLink(programme, openIntake)}>Apply for this programme</LinkButton>
          <LinkButton href={`/contact?programme=${programme.slug}`} variant="outline">
            Ask a question
          </LinkButton>
        </div>
      </Section>
    </>
  );
}
