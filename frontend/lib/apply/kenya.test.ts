import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { counties, kcseGrades } from "./kenya";

describe("lists shared with the API", () => {
  const go = readFileSync(resolve(process.cwd(), "../backend/internal/application/kenya.go"), "utf8");
  const quoted = (block: string) => [...block.matchAll(/"([^"]+)"/g)].map((m) => m[1]);

  it("has the 47 counties plus Outside Kenya, as the API does", () => {
    expect(counties).toHaveLength(48);
    expect(new Set(counties).size).toBe(48);
    const goCounties = quoted(go.slice(go.indexOf("var Counties"), go.indexOf("const OutsideKenya")));
    expect([...goCounties, "Outside Kenya"]).toEqual(counties);
  });

  it("has the same KCSE grades as the API", () => {
    expect(quoted(go.slice(go.indexOf("var KCSEGrades")))).toEqual(kcseGrades);
  });
});
