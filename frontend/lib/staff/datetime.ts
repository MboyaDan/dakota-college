// The college works in Africa/Nairobi time (A-01), which is UTC+3 with no
// daylight saving. Date-time inputs show Nairobi time whatever the browser's
// own time zone is.

const NAIROBI_OFFSET = "+03:00";

/** ISO timestamp -> "YYYY-MM-DDTHH:mm" in Nairobi time, for <input type="datetime-local">. */
export function isoToNairobiInput(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const shifted = new Date(d.getTime() + 3 * 60 * 60 * 1000);
  return shifted.toISOString().slice(0, 16);
}

/** "YYYY-MM-DDTHH:mm" in Nairobi time -> ISO timestamp with offset, or null. */
export function nairobiInputToIso(value: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null;
  return `${value}:00${NAIROBI_OFFSET}`;
}

/** Human date-time in Nairobi, e.g. "22 Oct 2026, 16:30". */
export function formatNairobi(iso: string | null | undefined): string {
  if (!iso) return "";
  return new Intl.DateTimeFormat("en-KE", {
    timeZone: "Africa/Nairobi",
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

export function formatDate(date: string | null | undefined): string {
  if (!date) return "";
  return new Intl.DateTimeFormat("en-KE", {
    timeZone: "UTC",
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(new Date(`${date}T00:00:00Z`));
}
