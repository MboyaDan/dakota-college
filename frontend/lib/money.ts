// Money is whole cents of KES (ADR-05). These helpers never use floating
// point: amounts are parsed and printed as digit strings.

/** Parses "45,000", "45000.5" or "KES 1,200.50" into cents; null if invalid. */
export function parseKesToCents(input: string): number | null {
  const s = input
    .trim()
    .replace(/^KES\s*/i, "")
    .replace(/,/g, "");
  const m = /^(\d{1,9})(?:\.(\d{1,2}))?$/.exec(s);
  if (!m) return null;
  const shillings = Number(m[1]);
  const cents = Number((m[2] ?? "").padEnd(2, "0"));
  return shillings * 100 + cents;
}

/** Formats cents as "KES 45,000.00". */
export function formatKes(cents: number): string {
  if (!Number.isSafeInteger(cents)) return "";
  const negative = cents < 0;
  const abs = Math.abs(cents);
  const whole = Math.floor(abs / 100)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const fraction = (abs % 100).toString().padStart(2, "0");
  return `${negative ? "-" : ""}KES ${whole}.${fraction}`;
}

/** Plain amount for an input box: "45000.00". */
export function centsToInput(cents: number): string {
  return `${Math.floor(cents / 100)}.${(cents % 100).toString().padStart(2, "0")}`;
}
