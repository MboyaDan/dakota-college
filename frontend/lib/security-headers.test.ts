import { describe, expect, it } from "vitest";
import {
  CLIENT_IP_HEADER,
  CLIENT_IP_KEY_HEADER,
  contentSecurityPolicy,
  forwardClientIP,
  isStrictPath,
  resolveContentImageOrigin,
} from "./security-headers";

describe("isStrictPath", () => {
  it("covers the portals and the application form", () => {
    for (const p of [
      "/staff",
      "/staff/login",
      "/student",
      "/student/fees",
      "/apply/new",
      "/apply/application",
      "/apply/start",
    ]) {
      expect(isStrictPath(p), p).toBe(true);
    }
  });
  it("leaves public pages, including the apply landing page", () => {
    for (const p of [
      "/",
      "/apply",
      "/applying",
      "/staffroom",
      "/admissions",
      "/programmes/x",
      "/apply/track",
    ]) {
      expect(isStrictPath(p), p).toBe(false);
    }
  });
});

describe("contentSecurityPolicy", () => {
  const imageOrigin = "https://media.example";
  it("uses a nonce and strict-dynamic on signed-in pages, never unsafe-inline scripts", () => {
    const p = contentSecurityPolicy({ nonce: "abc123", imageOrigin });
    expect(p).toContain("script-src 'self' 'nonce-abc123' 'strict-dynamic'");
    expect(p).not.toMatch(/script-src[^;]*unsafe-inline/);
    expect(p).not.toContain("unsafe-eval");
  });
  it("allows inline scripts on cached public pages", () => {
    expect(contentSecurityPolicy({ imageOrigin })).toContain("script-src 'self' 'unsafe-inline'");
  });
  it("blocks framing, plugins, base tags and other origins, except content images", () => {
    const p = contentSecurityPolicy({ imageOrigin });
    for (const d of [
      "default-src 'self'",
      "frame-ancestors 'none'",
      "object-src 'none'",
      "base-uri 'self'",
      "form-action 'self'",
      "connect-src 'self'",
      "frame-src 'none'",
      "img-src 'self' data: blob: https://media.example",
    ]) {
      expect(p).toContain(d);
    }
  });
  it("adds eval only in development", () => {
    expect(contentSecurityPolicy({ imageOrigin, dev: true })).toContain("'unsafe-eval'");
  });
});

describe("resolveContentImageOrigin", () => {
  it("defaults to local MinIO and accepts an origin", () => {
    expect(resolveContentImageOrigin(undefined)).toBe("http://localhost:9000");
    expect(resolveContentImageOrigin(" https://media.example/ ")).toBe("https://media.example");
  });
  it("refuses paths, other schemes and credentials", () => {
    for (const v of [
      "https://media.example/dakota-public",
      "ftp://media.example",
      "https://u:p@media.example",
      "nope",
    ]) {
      expect(() => resolveContentImageOrigin(v), v).toThrow();
    }
  });
});

describe("forwardClientIP", () => {
  it("copies the platform's address with the secret", () => {
    const h = new Headers({ "cf-connecting-ip": "41.90.1.2" });
    forwardClientIP(h, "cf-connecting-ip", "s3cret");
    expect(h.get(CLIENT_IP_HEADER)).toBe("41.90.1.2");
    expect(h.get(CLIENT_IP_KEY_HEADER)).toBe("s3cret");
  });
  it("takes the first address of a list", () => {
    const h = new Headers({ "x-forwarded-for": "41.90.1.2, 10.0.0.1" });
    forwardClientIP(h, "x-forwarded-for", "s3cret");
    expect(h.get(CLIENT_IP_HEADER)).toBe("41.90.1.2");
  });
  it("removes what a visitor sent under our header names", () => {
    const h = new Headers({ [CLIENT_IP_HEADER]: "1.2.3.4", [CLIENT_IP_KEY_HEADER]: "guess" });
    forwardClientIP(h, undefined, undefined);
    expect(h.has(CLIENT_IP_HEADER)).toBe(false);
    expect(h.has(CLIENT_IP_KEY_HEADER)).toBe(false);
    const h2 = new Headers({ [CLIENT_IP_HEADER]: "1.2.3.4" });
    forwardClientIP(h2, "cf-connecting-ip", "s3cret"); // platform header missing
    expect(h2.has(CLIENT_IP_HEADER)).toBe(false);
  });
});
