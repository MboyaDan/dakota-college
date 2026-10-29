import { createElement, type CSSProperties } from "react";
import Link from "next/link";
import { ArrowRight, GraduationCap } from "@/components/icons";
import type { PublicSchool } from "@/lib/public/types";
import { schools as schoolIdentity, schoolAccent } from "@/lib/schools";
import { ContentImage } from "./content-image";

const icons = new Map(schoolIdentity.map((s) => [s.key, s.icon]));

/** The school's icon from the site icon set, in its accent colour. Decorative. */
export function SchoolIcon({
  school,
  className,
  onDark = false,
}: {
  school: Pick<PublicSchool, "accent_key">;
  className?: string;
  onDark?: boolean;
}) {
  const accent = schoolAccent[school.accent_key];
  // The icons are module-level constants, not components made during render.
  return createElement(icons.get(school.accent_key) ?? GraduationCap, {
    "aria-hidden": true,
    className: `${className ?? "size-8"} ${onDark ? accent.textOnDark : accent.text}`,
  });
}

/**
 * A school tile. With a photo (uploaded with consent), the photo fills the
 * tile under a navy fade that keeps the text readable whatever the photo
 * shows. Without one, a plain navy tile with a glow in the school's colour,
 * so the grid looks finished before the college has photos.
 */
export function SchoolCard({ school, programmeCount }: { school: PublicSchool; programmeCount: number }) {
  const accent = schoolAccent[school.accent_key];
  return (
    <article
      className={`on-dark group relative isolate flex min-h-72 flex-1 flex-col justify-end gap-3 overflow-hidden rounded-card p-6 text-white shadow-md ${school.photo ? "bg-navy-900" : "school-tile-plain"}`}
      style={school.photo ? undefined : ({ "--tile-accent": accent.cssVar } as CSSProperties)}
    >
      {school.photo ? (
        <>
          <ContentImage
            image={school.photo}
            alt=""
            sizes="(min-width: 1024px) 33vw, (min-width: 640px) 50vw, 100vw"
            className="absolute inset-0 -z-20 size-full object-cover transition-transform duration-500 group-hover:scale-105"
          />
          <div
            aria-hidden
            className="absolute inset-0 -z-10 bg-linear-to-t from-navy-900 from-30% via-navy-900/85 via-70% to-navy-900/35"
          />
        </>
      ) : (
        <SchoolIcon
          school={school}
          onDark
          className="absolute -top-4 -right-4 -z-10 size-40 opacity-15 transition-transform duration-500 group-hover:scale-105"
        />
      )}
      <SchoolIcon school={school} onDark className="size-9" />
      <h3 className="text-2xl font-semibold text-white">
        <Link href={`/schools/${school.slug}`} className="after:absolute after:inset-0 hover:underline">
          {school.name}
        </Link>
      </h3>
      {school.summary ? <p className="text-white/90">{school.summary}</p> : null}
      <p className="flex items-center gap-1 font-semibold">
        {programmeCount === 1 ? "1 programme" : `${programmeCount} programmes`}
        <ArrowRight aria-hidden className="size-4 transition-transform group-hover:translate-x-1" />
      </p>
    </article>
  );
}
