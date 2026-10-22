import Link from "next/link";
import { Calendar, Clock, GraduationCap } from "@/components/icons";
import { Card } from "@/components/ui/card";
import { Tag } from "@/components/ui/tag";
import { durationText, feeText, levelLabels, nextIntake, tuitionFee } from "@/lib/public/programmes";
import type { PublicProgramme, PublicSchool } from "@/lib/public/types";
import { formatDate } from "@/lib/staff/datetime";

/** Answers "what, how long, what it costs, when" at a glance (CLAUDE.md section 2). */
export function ProgrammeCard({
  programme,
  school,
  headingLevel = 3,
}: {
  programme: PublicProgramme;
  school?: PublicSchool;
  headingLevel?: 2 | 3;
}) {
  const Heading = `h${headingLevel}` as const;
  const intake = nextIntake(programme);
  const tuition = tuitionFee(programme);
  const duration = durationText(programme);
  return (
    <Card accent={school?.accent_key} className="relative h-full">
      <div className="flex flex-wrap gap-2">
        <Tag tone="navy">{levelLabels[programme.level]}</Tag>
        {school ? <Tag school={school.accent_key}>{school.short_name}</Tag> : null}
      </div>
      <Heading className="text-xl font-semibold text-navy-900">
        <Link href={`/programmes/${programme.slug}`} className="after:absolute after:inset-0 hover:underline">
          {programme.title}
        </Link>
      </Heading>
      {programme.summary ? <p className="text-ink">{programme.summary}</p> : null}
      <dl className="mt-auto grid gap-1 text-sm">
        {duration ? (
          <div className="flex items-start gap-2">
            <Clock aria-hidden className="mt-0.5 size-4 shrink-0 text-ink-muted" />
            <dt className="sr-only">Duration</dt>
            <dd>{duration}</dd>
          </div>
        ) : null}
        {intake ? (
          <div className="flex items-start gap-2">
            <Calendar aria-hidden className="mt-0.5 size-4 shrink-0 text-ink-muted" />
            <dt className="sr-only">Next intake</dt>
            <dd>
              Next intake {intake.name}, starts {formatDate(intake.starts_on)}
            </dd>
          </div>
        ) : null}
        {tuition ? (
          <div className="flex items-start gap-2">
            <GraduationCap aria-hidden className="mt-0.5 size-4 shrink-0 text-ink-muted" />
            <dt className="sr-only">Tuition</dt>
            <dd>Tuition {feeText(tuition)}</dd>
          </div>
        ) : null}
      </dl>
    </Card>
  );
}
