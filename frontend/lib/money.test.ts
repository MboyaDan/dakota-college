import { describe, expect, it } from "vitest";
import { centsToInput, formatKes, parseKesToCents } from "./money";

describe("parseKesToCents", () => {
  it.each([
    ["45000", 4500000],
    ["45,000", 4500000],
    ["KES 1,200.50", 120050],
    ["0.5", 50],
    ["0.05", 5],
    ["  99.99 ", 9999],
    ["0", 0],
  ])("%s -> %d", (input, cents) => {
    expect(parseKesToCents(input)).toBe(cents);
  });

  it.each(["", "abc", "-5", "1.234", "1e5", "12,34.5.6", "1234567890"])("rejects %s", (input) => {
    expect(parseKesToCents(input)).toBeNull();
  });

  it("does not suffer floating point rounding", () => {
    // 0.1 + 0.2 style errors would give 1029 here.
    expect(parseKesToCents("10.30")).toBe(1030);
    expect(parseKesToCents("19.99")).toBe(1999);
  });
});

describe("formatKes", () => {
  it("formats with thousands separators and two decimals", () => {
    expect(formatKes(4500000)).toBe("KES 45,000.00");
    expect(formatKes(5)).toBe("KES 0.05");
    expect(formatKes(123456789)).toBe("KES 1,234,567.89");
  });

  it("round-trips through the input format", () => {
    for (const c of [0, 1, 99, 100, 4500050]) {
      expect(parseKesToCents(centsToInput(c))).toBe(c);
    }
  });
});
