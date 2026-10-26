import type { Status } from "@/components/ui/tag";
import type { components } from "@/lib/api/schema";
import { formatDate } from "@/lib/staff/datetime";

// Staff wording for student records and academics (Stage 11b).

export type StudentStatus = components["schemas"]["StudentStatus"];
export type Relationship = components["schemas"]["Guardian"]["relationship"];

export const studentStatus: Record<StudentStatus, { label: string; tone: Status }> = {
  active: { label: "Active", tone: "success" },
  deferred: { label: "Deferred", tone: "warning" },
  suspended: { label: "Suspended", tone: "danger" },
  withdrawn: { label: "Withdrawn", tone: "info" },
  graduated: { label: "Graduated", tone: "success" },
};

export const studentStatuses = Object.keys(studentStatus) as StudentStatus[];

export const relationships: { value: Relationship; label: string }[] = [
  { value: "mother", label: "Mother" },
  { value: "father", label: "Father" },
  { value: "guardian", label: "Guardian" },
  { value: "other_relative", label: "Other relative" },
  { value: "spouse", label: "Spouse" },
  { value: "other", label: "Other" },
];

/** Weekdays as the API numbers them: 1 is Monday. */
export const weekdays = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];

export function weekdayName(n: number | null | undefined): string {
  return n && n >= 1 && n <= 7 ? weekdays[n - 1] : "";
}

type Entry = { kind: "class" | "exam"; weekday: number | null; on_date: string | null; starts_at: string };

/** Weekly classes by weekday and time, then exams by date. */
export function sortTimetable<T extends Entry>(entries: T[]): T[] {
  return [...entries].sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === "class" ? -1 : 1;
    if (a.kind === "class")
      return (a.weekday ?? 0) - (b.weekday ?? 0) || a.starts_at.localeCompare(b.starts_at);
    return (a.on_date ?? "").localeCompare(b.on_date ?? "") || a.starts_at.localeCompare(b.starts_at);
  });
}

type TimetableEntry = components["schemas"]["TimetableEntry"];

/** "Tuesday 08:30 to 10:00, Lab 1" or "Exam 3 March 2027, 09:00 to 11:00". */
export function describeEntry(e: TimetableEntry): string {
  const when = e.kind === "class" ? weekdayName(e.weekday) : `Exam ${formatDate(e.on_date)},`;
  return `${when} ${e.starts_at} to ${e.ends_at}${e.room ? `, ${e.room}` : ""}`;
}
