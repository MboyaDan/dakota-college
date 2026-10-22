import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Calendar, MapPin } from "@/components/icons";
import { JsonLd } from "@/components/public/json-ld";
import { PageHeader } from "@/components/public/page-header";
import { RichText } from "@/components/public/rich-text";
import { getEvent } from "@/lib/public/news";
import { getSite } from "@/lib/public/site";
import { pageMetadata } from "@/lib/seo";
import { site as siteFacts } from "@/lib/site";
import { formatNairobi } from "@/lib/staff/datetime";

export async function generateStaticParams() {
  const site = await getSite();
  return site.events.length ? site.events.map((e) => ({ slug: e.slug })) : [{ slug: "none" }];
}

export async function generateMetadata({ params }: PageProps<"/events/[slug]">): Promise<Metadata> {
  const event = await getEvent((await params).slug);
  if (!event) return {};
  return pageMetadata({
    title: event.title,
    description: event.summary || `${event.title}, ${formatNairobi(event.starts_at)}`,
    path: `/events/${event.slug}`,
  });
}

export default async function EventPage({ params }: PageProps<"/events/[slug]">) {
  const event = await getEvent((await params).slug);
  if (!event) notFound();

  return (
    <>
      <JsonLd
        data={{
          "@context": "https://schema.org",
          "@type": "Event",
          name: event.title,
          startDate: event.starts_at,
          ...(event.ends_at ? { endDate: event.ends_at } : {}),
          eventAttendanceMode: "https://schema.org/OfflineEventAttendanceMode",
          location: { "@type": "Place", name: event.location || siteFacts.name, address: siteFacts.location },
          organizer: { "@type": "EducationalOrganization", name: siteFacts.name },
          ...(event.summary ? { description: event.summary } : {}),
        }}
      />
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Events", href: "/events" }, { label: event.title }]}
        title={event.title}
        lead={event.summary || undefined}
      >
        <dl className="grid gap-2 sm:grid-cols-2">
          <div className="flex items-center gap-2">
            <dt>
              <Calendar aria-hidden className="size-5 text-royal" />
              <span className="sr-only">When</span>
            </dt>
            <dd className="font-semibold">
              {formatNairobi(event.starts_at)}
              {event.ends_at ? ` to ${formatNairobi(event.ends_at)}` : ""}
            </dd>
          </div>
          {event.location ? (
            <div className="flex items-center gap-2">
              <dt>
                <MapPin aria-hidden className="size-5 text-royal" />
                <span className="sr-only">Where</span>
              </dt>
              <dd className="font-semibold">{event.location}</dd>
            </div>
          ) : null}
        </dl>
      </PageHeader>
      <article className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10 lg:px-6">
        <RichText text={event.body} />
        <Link
          href="/events"
          className="inline-flex min-h-touch items-center font-semibold text-royal underline"
        >
          All events
        </Link>
      </article>
    </>
  );
}
