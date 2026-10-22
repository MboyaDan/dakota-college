import type { Metadata } from "next";
import Link from "next/link";
import { Calendar, MapPin } from "@/components/icons";
import { PageHeader, Section } from "@/components/public/page-header";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { getEvents } from "@/lib/public/news";
import type { PublicEvent } from "@/lib/public/types";
import { pageMetadata } from "@/lib/seo";
import { formatNairobi } from "@/lib/staff/datetime";

export const metadata: Metadata = pageMetadata({
  title: "Events",
  description: "Open days, graduations and other events at Dakota Training College.",
  path: "/events",
});

function EventCard({ event }: { event: PublicEvent }) {
  return (
    <Card className="relative h-full">
      <p className="flex items-center gap-2 font-semibold text-royal">
        <Calendar aria-hidden className="size-5" />
        {formatNairobi(event.starts_at)}
      </p>
      <h3 className="text-lg font-semibold text-navy-900">
        <Link href={`/events/${event.slug}`} className="after:absolute after:inset-0 hover:underline">
          {event.title}
        </Link>
      </h3>
      {event.location ? (
        <p className="flex items-center gap-2 text-sm">
          <MapPin aria-hidden className="size-4 text-ink-muted" />
          {event.location}
        </p>
      ) : null}
      {event.summary ? <p className="text-sm">{event.summary}</p> : null}
    </Card>
  );
}

export default async function EventsPage() {
  const { upcoming, past } = await getEvents();
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Events" }]}
        title="Events"
        lead={
          <>
            Open days, graduations and other dates at the college.{" "}
            <Link href="/news" className="text-royal underline">
              Read the news
            </Link>
            .
          </>
        }
      />
      <Section id="upcoming" title="Coming up">
        {upcoming.length ? (
          <ul className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {upcoming.map((e) => (
              <li key={e.slug}>
                <EventCard event={e} />
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState
            title="No upcoming events"
            description="New events will appear here when the college announces them."
          />
        )}
      </Section>
      {past.length ? (
        <Section id="past" title="Recent events" className="bg-white">
          <ul className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {past.map((e) => (
              <li key={e.slug}>
                <EventCard event={e} />
              </li>
            ))}
          </ul>
        </Section>
      ) : null}
    </>
  );
}
