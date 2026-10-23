import { describe, expect, it } from "vitest";
import { renderTemplate, unknownPlaceholders } from "./messages";

describe("renderTemplate", () => {
  it("fills known placeholders and leaves others", () => {
    expect(renderTemplate("Hi {name}, {reference}. {other}")).toBe(
      "Hi Test Applicant 001, DTC-APP-2026-000123. {other}",
    );
  });
});

describe("unknownPlaceholders", () => {
  it("lists each unknown name once", () => {
    expect(unknownPlaceholders("{name} {link} {fees} {fees} {Name}", ["name", "link"])).toEqual(["fees"]);
  });
});
