import { describe, expect, it } from "vitest";
import { safeNext } from "@/components/staff/auth-forms";

describe("safeNext", () => {
  it("only follows staff portal paths", () => {
    expect(safeNext("/staff/content/news")).toBe("/staff/content/news");
    expect(safeNext(null)).toBe("/staff");
    expect(safeNext("https://evil.example/staff")).toBe("/staff");
    expect(safeNext("//evil.example")).toBe("/staff");
    expect(safeNext("/admin")).toBe("/staff");
  });
});
