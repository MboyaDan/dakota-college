import { SiteLink } from "./site-link";
import { ArrowRight, Megaphone } from "@/components/icons";
import type { NavLink } from "@/lib/navigation";

export type Announcement = { message: string; link?: NavLink };

/**
 * Editable notice above the header (brief 5.1). Callers pass only an active
 * announcement; expiry is filtered where the data is loaded (Stage 5/6).
 * Renders nothing when there is no announcement.
 */
export function AnnouncementBar({ announcement }: { announcement?: Announcement | null }) {
  if (!announcement) return null;
  return (
    <div className="bg-gold text-navy-900">
      <p className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2 text-sm font-medium lg:px-6">
        <Megaphone aria-hidden className="size-4 shrink-0" />
        <span>{announcement.message}</span>
        {announcement.link ? (
          <SiteLink
            href={announcement.link.href}
            className="inline-flex min-h-touch items-center gap-1 font-semibold underline underline-offset-4 sm:min-h-0"
          >
            {announcement.link.label}
            <ArrowRight aria-hidden className="size-4" />
          </SiteLink>
        ) : null}
      </p>
    </div>
  );
}

/** Contact, Alumni and portal links. On small screens they move into the menu. */
export function UtilityBar({ links }: { links: NavLink[] }) {
  return (
    <div className="on-dark hidden border-b border-white/15 bg-navy-900 text-white lg:block">
      <nav aria-label="Utility" className="mx-auto flex max-w-6xl justify-end px-6">
        <ul className="flex gap-1 text-sm">
          {links.map((link) => (
            <li key={link.href}>
              <SiteLink
                href={link.href}
                className="inline-flex min-h-touch items-center px-3 text-white/90 hover:text-white hover:underline"
              >
                {link.label}
              </SiteLink>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}

export function SkipLink() {
  return (
    <a
      href="#main"
      className="sr-only z-50 rounded-control bg-gold px-4 py-3 font-semibold text-navy-900 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
    >
      Skip to main content
    </a>
  );
}
