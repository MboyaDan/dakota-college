import { describe, expect, it } from "vitest";
import { headingId, headingsOf, parseRichText } from "./rich-text";

describe("parseRichText", () => {
  it("reads headings, lists and paragraphs", () => {
    const blocks = parseRichText(
      "Intro line one\nline two\n\n## Who we are\n\n- first\n- second\nAfter the list\n\n\n## Rights",
    );
    expect(blocks).toEqual([
      { kind: "paragraph", text: "Intro line one\nline two" },
      { kind: "heading", text: "Who we are" },
      { kind: "list", items: ["first", "second"] },
      { kind: "paragraph", text: "After the list" },
      { kind: "heading", text: "Rights" },
    ]);
    expect(headingsOf(blocks)).toEqual(["Who we are", "Rights"]);
  });

  it("keeps markup as plain text and handles Windows line endings", () => {
    expect(parseRichText("<script>alert(1)</script>\r\n\r\n- <b>x</b>")).toEqual([
      { kind: "paragraph", text: "<script>alert(1)</script>" },
      { kind: "list", items: ["<b>x</b>"] },
    ]);
  });

  it("returns nothing for empty text", () => {
    expect(parseRichText("  \n\n ")).toEqual([]);
  });
});

describe("headingId", () => {
  it("makes stable ids", () => {
    expect(headingId("Who we are")).toBe("who-we-are");
    expect(headingId("Your rights!")).toBe("your-rights");
    expect(headingId("???")).toBe("section");
  });
});
