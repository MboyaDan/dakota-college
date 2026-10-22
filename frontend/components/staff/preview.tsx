import { formatKes } from "@/lib/money";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import type { Doc } from "@/lib/staff/doc";
import type { ContentKind } from "@/lib/staff/kinds";

const levelLabel: Record<string, string> = {
  short_course: "Short course",
  artisan: "Artisan",
  certificate: "Certificate",
  diploma: "Diploma",
  higher_diploma: "Higher diploma",
};
const frequencyLabel: Record<string, string> = {
  once: "once",
  per_term: "per term",
  per_year: "per year",
  whole_programme: "for the whole programme",
};

function Paragraphs({ text }: { text: unknown }) {
  if (typeof text !== "string" || !text.trim()) return null;
  return (
    <>
      {text.split(/\n\s*\n/).map((p, i) => (
        <p key={i}>{p}</p>
      ))}
    </>
  );
}

function Missing({ what }: { what: string }) {
  return <p className="text-danger">Missing: {what}</p>;
}

type Row = Record<string, unknown>;

/** How the item will read on the site. Programmes show the five answers an applicant looks for. */
export function Preview({ kind, doc }: { kind: ContentKind; doc: Doc }) {
  if (kind === "programmes") {
    const fees = (doc.fees as Row[]) ?? [];
    const requirements = (doc.requirements as Row[]) ?? [];
    const intakes = (doc.intakes as Row[]) ?? [];
    return (
      <article className="flex flex-col gap-4">
        <p className="text-sm font-semibold text-gold-text">{levelLabel[doc.level as string] ?? ""}</p>
        <h2 className="text-3xl font-semibold">{(doc.title as string) || "Untitled programme"}</h2>
        <Paragraphs text={doc.summary} />
        <dl className="grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="font-semibold">What will I learn?</dt>
            <dd>
              {doc.what_you_learn ? (
                <Paragraphs text={doc.what_you_learn} />
              ) : (
                <Missing what="what you will learn" />
              )}
            </dd>
          </div>
          <div>
            <dt className="font-semibold">How long?</dt>
            <dd>
              {doc.duration_months ? `${doc.duration_months} months` : <Missing what="duration" />}{" "}
              {doc.duration_note as string}
            </dd>
          </div>
          <div>
            <dt className="font-semibold">What do I need?</dt>
            <dd>
              {requirements.length ? (
                <ul className="list-disc pl-5">
                  {requirements.map((r, i) => (
                    <li key={i}>{r.requirement as string}</li>
                  ))}
                </ul>
              ) : (
                <Missing what="entry requirements" />
              )}
            </dd>
          </div>
          <div>
            <dt className="font-semibold">What does it cost?</dt>
            <dd>
              {fees.length ? (
                <ul>
                  {fees.map((f, i) => (
                    <li key={i} className="tabular">
                      {f.label as string}: {formatKes(f.amount_cents as number)}{" "}
                      {frequencyLabel[f.frequency as string]}
                    </li>
                  ))}
                </ul>
              ) : (
                <Missing what="fees" />
              )}
            </dd>
          </div>
          <div>
            <dt className="font-semibold">When do I start?</dt>
            <dd>
              {intakes.length ? (
                <ul>
                  {intakes.map((n, i) => (
                    <li key={i}>
                      {n.name as string}: starts {formatDate(n.starts_on as string)}
                      {n.application_deadline
                        ? `, apply by ${formatDate(n.application_deadline as string)}`
                        : ""}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-ink-muted">No intake dates yet. The page will say dates are coming.</p>
              )}
            </dd>
          </div>
          <div>
            <dt className="font-semibold">How do I apply?</dt>
            <dd>The page shows an “Apply now” button.</dd>
          </div>
        </dl>
      </article>
    );
  }

  const title = (doc.title ??
    doc.name ??
    doc.question ??
    doc.message ??
    doc.body ??
    doc.label ??
    "") as string;
  return (
    <article className="flex flex-col gap-3">
      {kind === "statistics" ? (
        <p className="font-serif text-5xl font-semibold text-navy-900">{(doc.value as string) || "—"}</p>
      ) : null}
      <h2 className="text-2xl font-semibold">{title || "Untitled"}</h2>
      {kind === "events" && doc.starts_at ? (
        <p className="font-semibold">{formatNairobi(doc.starts_at as string)}</p>
      ) : null}
      {kind === "accreditations" ? <p className="font-mono">{doc.reference as string}</p> : null}
      <Paragraphs text={doc.summary} />
      <Paragraphs text={doc.answer ?? doc.description ?? doc.body} />
      {kind === "banners" && doc.link_label ? (
        <p className="font-semibold underline">{doc.link_label as string}</p>
      ) : null}
      {kind === "statistics" || kind === "accreditations" ? (
        doc.verified_on ? (
          <p className="text-sm text-ink-muted">Checked on {formatDate(doc.verified_on as string)}</p>
        ) : (
          <Missing what="verification date: it will not be shown" />
        )
      ) : null}
    </article>
  );
}
