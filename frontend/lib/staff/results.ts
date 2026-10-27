import type { Status } from "@/components/ui/tag";
import type { components } from "@/lib/api/schema";

// Results wording and marks-entry rules (Stage 13b), shared by the staff
// screens and the student portal. The backend decides; these only explain.

export type Sheet = components["schemas"]["ResultSheet"];
export type SheetStatus = Sheet["status"];
export type Scheme = components["schemas"]["AssessmentScheme"];
export type Computed = components["schemas"]["ComputedResult"];

export const sheetStatus: Record<SheetStatus, { label: string; tone: Status }> = {
  draft: { label: "Being marked", tone: "info" },
  submitted: { label: "Waiting for approval", tone: "warning" },
  approved: { label: "Approved, not released", tone: "info" },
  released: { label: "Released", tone: "success" },
};

export const outcomeLabel = { competent: "Competent", not_yet_competent: "Not yet competent" } as const;
export type Outcome = keyof typeof outcomeLabel;

/** "Continuous assessment 30%, final exam 70%. Pass mark 40." */
export function describeScheme(s: Scheme): string {
  if (s.kind === "competency") return "Competency based: each student is Competent or Not yet competent.";
  const parts = s.components.map((c) => `${c.name} ${c.weight}%`).join(", ");
  return `${parts}. Pass mark ${s.pass_mark}.`;
}

/** One result in words, for tables and screen readers. */
export function describeResult(
  r: Pick<Computed, "complete" | "total" | "grade" | "passed" | "outcome">,
): string {
  if (!r.complete) return "Marks missing";
  if (r.outcome) return outcomeLabel[r.outcome as Outcome];
  return `${r.total} (${r.grade}), ${r.passed ? "pass" : "fail"}`;
}

export const cellKey = (student: string, component: string) => `${student}:${component}`;

/** What is in the sheet now, as the text a field would show. */
export function initialCells(sheet: Sheet): Record<string, string> {
  const cells: Record<string, string> = {};
  for (const row of sheet.rows) {
    for (const c of sheet.scheme.components) {
      const m = row.marks[c.key ?? ""];
      cells[cellKey(row.student_id, c.key ?? "")] =
        sheet.scheme.kind === "competency" ? (m?.outcome ?? "") : m?.score == null ? "" : String(m.score);
    }
  }
  return cells;
}

export type Entry = { student_id: string; component: string; score?: number; outcome?: Outcome };

/**
 * The marks that changed since the sheet was loaded, ready to send, and a
 * message for each field that cannot be sent. Marks are never removed, only
 * corrected, so emptying a field that had a mark is a problem too.
 */
export function changedEntries(
  sheet: Sheet,
  cells: Record<string, string>,
): { entries: Entry[]; keys: string[]; problems: Record<string, string> } {
  const before = initialCells(sheet);
  const entries: Entry[] = [];
  const keys: string[] = [];
  const problems: Record<string, string> = {};
  for (const row of sheet.rows) {
    for (const c of sheet.scheme.components) {
      const component = c.key ?? "";
      const key = cellKey(row.student_id, component);
      const value = (cells[key] ?? "").trim();
      if (value === before[key]) continue;
      if (value === "") {
        problems[key] = "Marks cannot be removed. Enter the corrected mark.";
        continue;
      }
      if (sheet.scheme.kind === "competency") {
        if (value !== "competent" && value !== "not_yet_competent") {
          problems[key] = "Choose Competent or Not yet competent.";
          continue;
        }
        entries.push({ student_id: row.student_id, component, outcome: value });
      } else {
        if (!/^\d{1,3}$/.test(value) || Number(value) > 100) {
          problems[key] = "Enter a whole number from 0 to 100.";
          continue;
        }
        entries.push({ student_id: row.student_id, component, score: Number(value) });
      }
      keys.push(key);
    }
  }
  return { entries, keys, problems };
}

/** One step in a sheet's history, in words. */
export function describeEvent(from: string, to: string): string {
  if (to === "submitted") return "Sent for approval";
  if (to === "approved") return "Approved";
  if (to === "released") return "Released to students";
  if (to === "draft" && from === "submitted") return "Returned to the trainer";
  if (to === "draft") return "Reopened for a correction";
  return `Moved from ${from} to ${to}`;
}

/** How many students passed, out of those with complete results. */
export function passCount(sheet: Sheet): { passed: number; complete: number } {
  const complete = sheet.rows.filter((r) => r.result.complete);
  return { passed: complete.filter((r) => r.result.passed).length, complete: complete.length };
}

/** The weights so far, for a running total while the scheme is edited. */
export function weightTotal(components: Scheme["components"]): number {
  return components.reduce((sum, c) => sum + (Number.isFinite(c.weight) ? c.weight : 0), 0);
}
