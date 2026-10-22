import { createElement } from "react";
import Link from "next/link";
import { ArrowRight, GraduationCap } from "@/components/icons";
import { Card } from "@/components/ui/card";
import type { PublicSchool } from "@/lib/public/types";
import { schools as schoolIdentity, schoolAccent } from "@/lib/schools";

const icons = new Map(schoolIdentity.map((s) => [s.key, s.icon]));

/** The school's icon from the site icon set, in its accent colour. Decorative. */
export function SchoolIcon({
  school,
  className,
}: {
  school: Pick<PublicSchool, "accent_key">;
  className?: string;
}) {
  // The icons are module-level constants, not components made during render.
  return createElement(icons.get(school.accent_key) ?? GraduationCap, {
    "aria-hidden": true,
    className: `${className ?? "size-8"} ${schoolAccent[school.accent_key].text}`,
  });
}

export function SchoolCard({ school, programmeCount }: { school: PublicSchool; programmeCount: number }) {
  return (
    <Card accent={school.accent_key} className="relative">
      <SchoolIcon school={school} />
      <h3 className="text-xl font-semibold text-navy-900">
        <Link href={`/schools/${school.slug}`} className="after:absolute after:inset-0 hover:underline">
          {school.name}
        </Link>
      </h3>
      {school.summary ? <p className="text-ink">{school.summary}</p> : null}
      <p className="mt-auto flex items-center gap-1 font-semibold text-royal">
        {programmeCount === 1 ? "1 programme" : `${programmeCount} programmes`}
        <ArrowRight aria-hidden className="size-4" />
      </p>
    </Card>
  );
}
