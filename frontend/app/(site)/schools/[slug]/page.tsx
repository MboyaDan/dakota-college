import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PageHeader, Prose, Section } from "@/components/public/page-header";
import { ProgrammeCard } from "@/components/public/programme-card";
import { SchoolIcon } from "@/components/public/school-card";
import { LinkButton } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { applyHref } from "@/lib/navigation";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";

async function findSchool(slug: string) {
  const site = await getSite();
  const school = site.schools.find((s) => s.slug === slug);
  return { site, school };
}

// With Cache Components the list must not be empty, so an empty site
// prerenders one placeholder that answers 404.
export async function generateStaticParams() {
  const site = await getSite();
  return site.schools.length ? site.schools.map((s) => ({ slug: s.slug })) : [{ slug: "none" }];
}

export async function generateMetadata({ params }: PageProps<"/schools/[slug]">): Promise<Metadata> {
  const { school } = await findSchool((await params).slug);
  if (!school) return {};
  return pageMetadata({
    title: school.name,
    description: school.summary || `Programmes offered by the ${school.name} at Dakota Training College.`,
    path: `/schools/${school.slug}`,
  });
}

export default async function SchoolPage({ params }: PageProps<"/schools/[slug]">) {
  const { site, school } = await findSchool((await params).slug);
  if (!school) notFound();
  const programmes = site.programmes.filter((p) => p.school_slug === school.slug);

  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Schools", href: "/schools" },
          { label: school.short_name },
        ]}
        title={school.name}
        lead={school.summary || undefined}
      >
        <SchoolIcon school={school} className="order-first size-10" />
      </PageHeader>

      {school.description ? (
        <Section id="overview" title="About the school">
          <Prose text={school.description} />
        </Section>
      ) : null}

      {school.head_message ? (
        <Section id="head" title="A message from the head of school" className="bg-white">
          <blockquote className="max-w-prose border-l-4 border-gold pl-4">
            <Prose text={school.head_message} />
          </blockquote>
        </Section>
      ) : null}

      <Section id="programmes" title="Programmes">
        {programmes.length ? (
          <ul className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {programmes.map((p) => (
              <li key={p.slug} className="flex">
                <ProgrammeCard programme={p} school={school} />
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState
            title="No programmes published yet"
            description="This school's programmes will appear here once the college publishes them. Contact us to ask what is planned."
            action={
              <LinkButton href="/contact" variant="outline">
                Contact us
              </LinkButton>
            }
          />
        )}
        {programmes.length ? (
          <div>
            <LinkButton href={applyHref}>Apply now</LinkButton>
          </div>
        ) : null}
      </Section>
    </>
  );
}
