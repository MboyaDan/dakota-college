import { describe, expect, it } from "vitest";
import { groupFaqs, searchFaqs } from "./faqs";
import type { PublicFAQ } from "./types";

const faq = (category: string, question: string, answer = ""): PublicFAQ => ({
  category_slug: category,
  category_name: category.toUpperCase(),
  programme_slug: null,
  question,
  answer,
  is_sample: false,
});

const faqs = [
  faq("fees", "How can I pay fees?", "By bank or M-Pesa."),
  faq("fees", "Instalments?"),
  faq("admissions", "How do I apply?"),
];

describe("faqs", () => {
  it("groups by category in order", () => {
    expect(groupFaqs(faqs).map((g) => [g.slug, g.items.length])).toEqual([
      ["fees", 2],
      ["admissions", 1],
    ]);
  });

  it("searches questions and answers for every word", () => {
    expect(searchFaqs(faqs, "m-pesa BANK").map((f) => f.question)).toEqual(["How can I pay fees?"]);
    expect(searchFaqs(faqs, "")).toHaveLength(3);
    expect(searchFaqs(faqs, "scholarship")).toEqual([]);
  });
});
