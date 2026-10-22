import { describe, expect, it } from "vitest";
import { DEFAULT_SITE_URL, resolveSiteUrl } from "./site-url";

describe("resolveSiteUrl", () => {
  it("defaults to the local Next.js origin when unset or blank", () => {
    expect(resolveSiteUrl(undefined).origin).toBe(DEFAULT_SITE_URL);
    expect(resolveSiteUrl("   ").origin).toBe(DEFAULT_SITE_URL);
  });

  it("normalises a valid origin", () => {
    expect(resolveSiteUrl("https://www.example.invalid/").href).toBe("https://www.example.invalid/");
    expect(resolveSiteUrl(" http://localhost:3000 ").origin).toBe("http://localhost:3000");
  });

  it.each([
    ["not a url", "not a valid URL"],
    ["ftp://www.example.invalid", "http or https"],
    ["https://user:pass@www.example.invalid", "credentials"],
    ["https://www.example.invalid/en", "origin only"],
    ["https://www.example.invalid?x=1", "origin only"],
  ])("rejects %s", (value, message) => {
    expect(() => resolveSiteUrl(value)).toThrow(message);
  });
});
