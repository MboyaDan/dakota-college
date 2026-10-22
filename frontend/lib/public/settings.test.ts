import { describe, expect, it } from "vitest";
import { settingsOf, telHref, whatsappHref } from "./settings";

describe("settingsOf", () => {
  it("trims values, keeps empty ones empty and drops non-https links", () => {
    const s = settingsOf({
      settings: {
        "contact.phone": "  +254 000 000 001 ",
        "campus.map_url": "http://maps.example",
        "social.facebook": "https://facebook.example/dakota",
        "social.x": "javascript:alert(1)",
      },
    });
    expect(s.phone).toBe("+254 000 000 001");
    expect(s.email).toBe("");
    expect(s.mapUrl).toBe("");
    expect(s.social).toEqual([{ label: "Facebook", href: "https://facebook.example/dakota" }]);
  });
});

describe("links", () => {
  it("builds tel and WhatsApp links from formatted numbers", () => {
    expect(telHref("+254 000 000 001")).toBe("tel:+254000000001");
    expect(whatsappHref("+254 000 000 001", "Hello, I have a question")).toBe(
      "https://wa.me/254000000001?text=Hello%2C%20I%20have%20a%20question",
    );
    expect(whatsappHref("", "x")).toBe("");
  });
});
