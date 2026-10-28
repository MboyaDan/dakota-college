import { describe, expect, it } from "vitest";
import { attachmentName, monthLabel, percent } from "./reports";

describe("percent", () => {
  it("matches the CSV: one decimal, half up", () => {
    expect(percent(1, 3)).toBe("33.3%");
    expect(percent(2, 3)).toBe("66.7%");
    expect(percent(1, 8)).toBe("12.5%");
    expect(percent(1, 16)).toBe("6.3%");
    expect(percent(5, 5)).toBe("100.0%");
    expect(percent(0, 7)).toBe("0.0%");
  });

  it("is empty when there is nothing to divide by", () => {
    expect(percent(0, 0)).toBe("");
  });
});

describe("attachmentName", () => {
  it("reads the quoted file name", () => {
    expect(
      attachmentName('attachment; filename="dakota-results-2026-01-01-to-2026-10-28.csv"', "x.csv"),
    ).toBe("dakota-results-2026-01-01-to-2026-10-28.csv");
  });

  it("falls back and strips path separators", () => {
    expect(attachmentName(null, "x.csv")).toBe("x.csv");
    expect(attachmentName("attachment", "x.csv")).toBe("x.csv");
    expect(attachmentName('attachment; filename="../a/b.csv"', "x.csv")).toBe("..ab.csv");
  });
});

describe("monthLabel", () => {
  it("names the month", () => {
    expect(monthLabel("2026-03")).toBe("March 2026");
    expect(monthLabel("bad")).toBe("bad");
  });
});
