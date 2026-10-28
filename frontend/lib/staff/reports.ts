// Formatting for the reports page (Stage 16c). Figures match the CSV
// exports exactly, so the same integer arithmetic as the Go reports package.

export type ExportKind = "admissions" | "enrolments" | "fee-collection" | "results";

/**
 * Part of whole as "62.5%", one decimal rounded half up, integers only
 * (as reports.Percent). Empty when whole is zero.
 */
export function percent(part: number, whole: number): string {
  if (whole <= 0) return "";
  const tenths = Math.floor((part * 1000 + Math.floor(whole / 2)) / whole);
  return `${Math.floor(tenths / 10)}.${tenths % 10}%`;
}

/** The file name from a Content-Disposition header, or fallback; never a path. */
export function attachmentName(header: string | null, fallback: string): string {
  const m = /filename="([^"]+)"/.exec(header ?? "");
  const name = m?.[1].replace(/[/\\]/g, "") ?? "";
  return name || fallback;
}

/** "2026-03" -> "March 2026". */
export function monthLabel(month: string): string {
  if (!/^\d{4}-\d{2}$/.test(month)) return month;
  return new Intl.DateTimeFormat("en-KE", { timeZone: "UTC", month: "long", year: "numeric" }).format(
    new Date(`${month}-01T00:00:00Z`),
  );
}

export const officeLabels: Record<string, string> = {
  admissions: "Admissions",
  finance: "Finance",
  general: "General",
};
