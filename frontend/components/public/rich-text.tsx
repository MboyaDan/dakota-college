import { headingId, parseRichText } from "@/lib/public/rich-text";

/** Text from the content manager as headings, lists and paragraphs. Never HTML. */
export function RichText({ text, headingLevel = 2 }: { text: string; headingLevel?: 2 | 3 }) {
  const Heading = `h${headingLevel}` as const;
  const blocks = parseRichText(text);
  if (!blocks.length) return null;
  return (
    <div className="flex max-w-prose flex-col gap-4">
      {blocks.map((b, i) => {
        switch (b.kind) {
          case "heading":
            return (
              <Heading
                key={i}
                id={headingId(b.text)}
                className="mt-4 scroll-mt-4 text-2xl font-semibold text-navy-900"
              >
                {b.text}
              </Heading>
            );
          case "list":
            return (
              <ul key={i} className="flex list-disc flex-col gap-1 pl-6">
                {b.items.map((item, j) => (
                  <li key={j}>{item}</li>
                ))}
              </ul>
            );
          default:
            return (
              <p key={i} className="whitespace-pre-line">
                {b.text}
              </p>
            );
        }
      })}
    </div>
  );
}
