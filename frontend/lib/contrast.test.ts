import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { contrastRatio, tokens, usedPairs } from "./contrast";

describe("contrastRatio", () => {
  it("matches known values", () => {
    expect(contrastRatio("#000000", "#FFFFFF")).toBeCloseTo(21, 5);
    expect(contrastRatio("#FFFFFF", "#FFFFFF")).toBeCloseTo(1, 5);
  });

  it("confirms the brief's warning: gold text on white fails", () => {
    expect(contrastRatio(tokens.gold, tokens.white)).toBeLessThan(3);
  });
});

describe("design tokens", () => {
  it.each(usedPairs)("$use: $fg on $bg meets $min:1", ({ fg, bg, min }) => {
    expect(contrastRatio(tokens[fg], tokens[bg])).toBeGreaterThanOrEqual(min);
  });

  it("CSS theme uses the same hex values as the contrast table", () => {
    const css = readFileSync(join(__dirname, "../app/globals.css"), "utf8").toLowerCase();
    const cssNames: Partial<Record<keyof typeof tokens, string>> = {
      navy900: "navy-900",
      royal: "royal",
      gold: "gold",
      goldText: "gold-text",
      ivory: "ivory",
      ink: "ink",
      inkMuted: "ink-muted",
      stone: "stone",
      success: "success",
      danger: "danger",
      warning: "warning",
      medical: "school-medical",
      hospitality: "school-hospitality",
      ict: "school-ict",
      business: "school-business",
      pool: "school-pool",
    };
    for (const [key, name] of Object.entries(cssNames)) {
      expect(css, name).toContain(`--color-${name}: ${tokens[key as keyof typeof tokens].toLowerCase()};`);
    }
  });
});
