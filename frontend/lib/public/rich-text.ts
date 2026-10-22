// The Pages text format (D-82): blank lines separate blocks, "## " starts a
// heading, lines starting "- " form a list. Everything is shown as text, so
// nothing a staff member types can become markup.

export type Block =
  { kind: "heading"; text: string } | { kind: "list"; items: string[] } | { kind: "paragraph"; text: string };

export function parseRichText(input: string): Block[] {
  const blocks: Block[] = [];
  let paragraph: string[] = [];
  let list: string[] = [];

  const flushParagraph = () => {
    if (paragraph.length) blocks.push({ kind: "paragraph", text: paragraph.join("\n") });
    paragraph = [];
  };
  const flushList = () => {
    if (list.length) blocks.push({ kind: "list", items: list });
    list = [];
  };

  for (const raw of input.replace(/\r\n?/g, "\n").split("\n")) {
    const line = raw.trim();
    if (!line) {
      flushParagraph();
      flushList();
    } else if (line.startsWith("## ")) {
      flushParagraph();
      flushList();
      blocks.push({ kind: "heading", text: line.slice(3).trim() });
    } else if (line.startsWith("- ")) {
      flushParagraph();
      list.push(line.slice(2).trim());
    } else {
      flushList();
      paragraph.push(line);
    }
  }
  flushParagraph();
  flushList();
  return blocks;
}

/** Headings, for an "On this page" list on long pages. */
export function headingsOf(blocks: Block[]): string[] {
  return blocks.flatMap((b) => (b.kind === "heading" ? [b.text] : []));
}

/** A stable id for a heading, e.g. "Who we are" -> "who-we-are". */
export function headingId(text: string): string {
  return (
    text
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "") || "section"
  );
}
