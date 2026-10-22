import { formatKes } from "@/lib/money";
import type { Level, PublicFee, PublicIntake, PublicProgramme, PublicSite } from "./types";

// Plain-language labels and the course finder filters. Pure functions, so the
// finder works the same in a test, on the server and in the bundled snapshot.

export const levelLabels: Record<Level, string> = {
  short_course: "Short course",
  artisan: "Artisan certificate",
  certificate: "Certificate",
  diploma: "Diploma",
  higher_diploma: "Higher diploma",
};

export const levelOrder: Level[] = ["short_course", "artisan", "certificate", "diploma", "higher_diploma"];

export const modeLabels: Record<PublicProgramme["mode"], string> = {
  full_time: "Full time",
  part_time: "Part time",
  blended: "Blended (on campus and online)",
};

const frequencyLabels: Record<PublicFee["frequency"], string> = {
  once: "once",
  per_term: "per term",
  per_year: "per year",
  whole_programme: "for the whole programme",
};

/** KCSE mean grades from highest to lowest. */
export const kcseGrades = ["A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D+", "D", "D-", "E"] as const;
export type KcseGrade = (typeof kcseGrades)[number];

export function durationText(p: Pick<PublicProgramme, "duration_months" | "duration_note">): string {
  const m = p.duration_months;
  let text = "";
  if (m != null) {
    if (m % 12 === 0) text = m === 12 ? "1 year" : `${m / 12} years`;
    else text = m === 1 ? "1 month" : `${m} months`;
  }
  if (p.duration_note) text = text ? `${text} (${p.duration_note})` : p.duration_note;
  return text;
}

export function feeText(fee: Pick<PublicFee, "amount_cents" | "frequency">): string {
  return `${formatKes(fee.amount_cents)} ${frequencyLabels[fee.frequency]}`;
}

/** The headline tuition line, if any, for cards and the finder. */
export function tuitionFee(p: PublicProgramme): PublicFee | undefined {
  return (
    p.fees.find((f) => f.category === "tuition" && f.intake_id == null) ??
    p.fees.find((f) => f.category === "tuition")
  );
}

/** The first intake people can still apply for or plan around. Intakes arrive sorted by start date. */
export function nextIntake(p: PublicProgramme): PublicIntake | undefined {
  return p.intakes.find((i) => i.status === "open") ?? p.intakes[0];
}

export type UpcomingIntake = { programme: PublicProgramme; intake: PublicIntake };

/** Intakes across all programmes, soonest first. */
export function upcomingIntakes(site: Pick<PublicSite, "programmes">, limit?: number): UpcomingIntake[] {
  const all = site.programmes.flatMap((programme) =>
    programme.intakes.map((intake) => ({ programme, intake })),
  );
  all.sort((a, b) => a.intake.starts_on.localeCompare(b.intake.starts_on));
  return limit ? all.slice(0, limit) : all;
}

export type DurationBand = "up-to-6" | "7-12" | "13-24" | "over-24";

export const durationBands: { value: DurationBand; label: string }[] = [
  { value: "up-to-6", label: "Up to 6 months" },
  { value: "7-12", label: "7 months to 1 year" },
  { value: "13-24", label: "1 to 2 years" },
  { value: "over-24", label: "More than 2 years" },
];

function inBand(months: number | null | undefined, band: DurationBand): boolean {
  if (months == null) return false;
  switch (band) {
    case "up-to-6":
      return months <= 6;
    case "7-12":
      return months > 6 && months <= 12;
    case "13-24":
      return months > 12 && months <= 24;
    case "over-24":
      return months > 24;
  }
}

export type FinderFilters = {
  q?: string;
  school?: string;
  level?: Level;
  duration?: DurationBand;
  /** The visitor's KCSE mean grade: show programmes they meet the minimum for. */
  grade?: KcseGrade;
};

type SearchParams = Record<string, string | string[] | undefined>;

function one(v: string | string[] | undefined): string | undefined {
  const s = Array.isArray(v) ? v[0] : v;
  return s?.trim() || undefined;
}

/** Reads filters from the URL, ignoring anything not on the lists. */
export function parseFilters(params: SearchParams): FinderFilters {
  const level = one(params.level);
  const duration = one(params.duration);
  const grade = one(params.grade);
  return {
    q: one(params.q)?.slice(0, 100),
    school: one(params.school),
    level: level && level in levelLabels ? (level as Level) : undefined,
    duration: durationBands.some((b) => b.value === duration) ? (duration as DurationBand) : undefined,
    grade: kcseGrades.includes(grade as KcseGrade) ? (grade as KcseGrade) : undefined,
  };
}

function meetsGrade(min: string | null | undefined, grade: KcseGrade): boolean {
  if (!min) return true;
  // Lower index is a higher grade.
  return kcseGrades.indexOf(grade) <= kcseGrades.indexOf(min as KcseGrade);
}

export function filterProgrammes(programmes: PublicProgramme[], f: FinderFilters): PublicProgramme[] {
  const words = (f.q ?? "").toLowerCase().split(/\s+/).filter(Boolean);
  return programmes.filter((p) => {
    if (f.school && p.school_slug !== f.school) return false;
    if (f.level && p.level !== f.level) return false;
    if (f.duration && !inBand(p.duration_months, f.duration)) return false;
    if (f.grade && !meetsGrade(p.min_kcse_grade, f.grade)) return false;
    if (words.length) {
      const text = `${p.title} ${p.summary} ${p.what_you_learn} ${p.career_outcomes}`.toLowerCase();
      if (!words.every((w) => text.includes(w))) return false;
    }
    return true;
  });
}

export function hasFilters(f: FinderFilters): boolean {
  return Object.values(f).some(Boolean);
}
