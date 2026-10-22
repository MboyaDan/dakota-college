import { describe, expect, it } from "vitest";
import { pageSlots } from "./pagination";

describe("pageSlots", () => {
  it("handles empty and single-page lists", () => {
    expect(pageSlots(1, 0)).toEqual([]);
    expect(pageSlots(1, 1)).toEqual([1]);
  });

  it("shows every page when there are few", () => {
    expect(pageSlots(2, 5)).toEqual([1, 2, 3, 4, 5]);
  });

  it("uses gaps around a middle page", () => {
    expect(pageSlots(10, 20)).toEqual([1, "gap", 9, 10, 11, "gap", 20]);
  });

  it("fills a one-page gap with the page instead of an ellipsis", () => {
    expect(pageSlots(4, 10)).toEqual([1, 2, 3, 4, 5, "gap", 10]);
  });

  it("clamps out-of-range pages", () => {
    expect(pageSlots(99, 3)).toEqual([1, 2, 3]);
    expect(pageSlots(-5, 3)).toEqual([1, 2, 3]);
  });
});
