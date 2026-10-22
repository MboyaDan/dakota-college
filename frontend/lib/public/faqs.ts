import type { PublicFAQ } from "./types";

export type FaqGroup = { slug: string; name: string; items: PublicFAQ[] };

/** FAQs grouped by category, keeping the API's order (category, then question). */
export function groupFaqs(faqs: PublicFAQ[]): FaqGroup[] {
  const groups: FaqGroup[] = [];
  for (const f of faqs) {
    let g = groups.find((x) => x.slug === f.category_slug);
    if (!g) {
      g = { slug: f.category_slug, name: f.category_name, items: [] };
      groups.push(g);
    }
    g.items.push(f);
  }
  return groups;
}

/** Questions containing every word of the search, in question or answer. */
export function searchFaqs(faqs: PublicFAQ[], query: string): PublicFAQ[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return faqs;
  return faqs.filter((f) => {
    const text = `${f.question} ${f.answer}`.toLowerCase();
    return words.every((w) => text.includes(w));
  });
}
