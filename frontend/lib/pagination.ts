export type PageSlot = number | "gap";

/**
 * Page numbers to show: first, last, and `siblings` either side of the
 * current page, with "gap" where pages are skipped. Short enough for 360 px.
 */
export function pageSlots(current: number, total: number, siblings = 1): PageSlot[] {
  if (total <= 0) return [];
  const page = Math.min(Math.max(1, current), total);
  const wanted = new Set<number>([1, total]);
  for (let p = page - siblings; p <= page + siblings; p++) {
    if (p >= 1 && p <= total) wanted.add(p);
  }
  const sorted = [...wanted].sort((a, b) => a - b);
  const slots: PageSlot[] = [];
  for (const p of sorted) {
    const prev = slots.at(-1);
    if (typeof prev === "number" && p - prev === 2) slots.push(prev + 1);
    else if (typeof prev === "number" && p - prev > 2) slots.push("gap");
    slots.push(p);
  }
  return slots;
}
