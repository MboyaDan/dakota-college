import { describe, expect, it } from "vitest";
import {
  describeBalance,
  describePlan,
  invoiceFor,
  normaliseMpesaCode,
  parseAmount,
  percentTotal,
} from "./finance";

describe("parseAmount", () => {
  it("takes positive amounts to the cent", () => {
    expect(parseAmount("12,500")).toBe(1_250_000);
    expect(parseAmount("KES 0.01")).toBe(1);
    expect(parseAmount("10.30")).toBe(1030);
  });
  it("refuses zero, negatives and more than two decimals", () => {
    for (const bad of ["", "0", "0.00", "-5", "12.505", "abc"]) expect(parseAmount(bad), bad).toBeNull();
  });
});

describe("wording", () => {
  it("describes balances, plans and invoices", () => {
    expect(describeBalance(500)).toBe("Owes KES 5.00");
    expect(describeBalance(-500)).toBe("In credit KES 5.00");
    expect(describeBalance(0)).toBe("Nothing owed");
    expect(describePlan([])).toBe("All due when the invoice is issued.");
    expect(
      describePlan([
        { percent: 50, due_days: 0 },
        { percent: 50, due_days: 30 },
      ]),
    ).toBe("50% on issue, 50% after 30 days.");
    expect(invoiceFor("enrolment", null)).toBe("First fees, at enrolment");
    expect(invoiceFor("term", "Term 1")).toBe("Term 1");
    expect(invoiceFor("other", null)).toBe("Other fees");
  });
  it("adds shares and tidies M-Pesa codes", () => {
    expect(percentTotal([{ percent: 60 }, { percent: Number.NaN }, { percent: 30 }])).toBe(90);
    expect(normaliseMpesaCode(" qk12 ab34cd ")).toBe("QK12AB34CD");
  });
});
