import { describe, expect, it } from "vitest";
import { DEFAULT_API_ORIGIN, resolveApiOrigin } from "./api-origin";

describe("resolveApiOrigin", () => {
  it("defaults to the local Compose API when unset or blank", () => {
    expect(resolveApiOrigin(undefined)).toBe(DEFAULT_API_ORIGIN);
    expect(resolveApiOrigin("   ")).toBe(DEFAULT_API_ORIGIN);
  });

  it("normalises a valid origin", () => {
    expect(resolveApiOrigin("https://api.example.invalid/")).toBe("https://api.example.invalid");
    expect(resolveApiOrigin(" http://localhost:8180 ")).toBe("http://localhost:8180");
  });

  it.each([
    ["not a url", "not a valid URL"],
    ["ftp://api.example.invalid", "http or https"],
    ["https://user:pass@api.example.invalid", "credentials"],
    ["https://api.example.invalid/api", "origin only"],
    ["https://api.example.invalid?x=1", "origin only"],
  ])("rejects %s", (value, message) => {
    expect(() => resolveApiOrigin(value)).toThrow(message);
  });
});
