import { describe, expect, it } from "vitest";
import { fieldErrors, fieldId, getAt, setAt, toPath } from "./doc";
import { kindOrder, kinds, slugify } from "./kinds";

describe("document paths", () => {
  it("converts API field names", () => {
    expect(toPath("fees[0].amount_cents")).toBe("fees.0.amount_cents");
    expect(toPath("title")).toBe("title");
  });

  it("reads and writes nested values without mutating the original", () => {
    const doc = { title: "A", fees: [{ label: "Tuition", amount_cents: 1 }] };
    const next = setAt(doc, "fees.0.amount_cents", 500);
    expect(getAt(next, "fees.0.amount_cents")).toBe(500);
    expect(doc.fees[0].amount_cents).toBe(1);
    expect(next.title).toBe("A");
  });

  it("keys API problems by path and keeps the first message", () => {
    expect(
      fieldErrors([
        { field: "fees[1].label", message: "Fill this in." },
        { field: "fees[1].label", message: "Second" },
      ]),
    ).toEqual({ "fees.1.label": "Fill this in." });
  });

  it("makes safe ids", () => {
    expect(fieldId("fees.0.amount_cents")).toBe("f-fees-0-amount_cents");
  });
});

describe("content kinds", () => {
  it("covers every kind the API knows, with empty documents for each", () => {
    expect(kindOrder.sort()).toEqual(Object.keys(kinds).sort());
    for (const k of kindOrder) {
      const empty = kinds[k].empty();
      for (const f of kinds[k].fields) {
        expect(empty, `${k}.${f.name}`).toHaveProperty(f.name);
      }
    }
  });

  it("makes web addresses", () => {
    expect(slugify("Diploma in Community Health ")).toBe("diploma-in-community-health");
    expect(slugify("Café & Hôtel Management!")).toBe("cafe-hotel-management");
  });
});
