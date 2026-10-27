"use client";

import Link from "next/link";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button, buttonClasses } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusTag } from "@/components/ui/tag";
import { api } from "@/lib/api/client";
import { openFileLink } from "@/lib/api/file-link";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { outcomeLabel, type Outcome } from "@/lib/staff/results";
import { Loaded, Page, useLoad } from "./pages";

// The student's own released results (Stage 13b, AC-09): only what the
// registrar has released, as released. Nothing from a sheet still in
// marking or approval reaches this page.

type StudentResult = components["schemas"]["StudentResult"];
type Transcript = components["schemas"]["Transcript"];

function byTerm(items: StudentResult[]): { term: string; items: StudentResult[] }[] {
  const terms: { term: string; items: StudentResult[] }[] = [];
  for (const r of items) {
    const t = terms.find((x) => x.term === r.term_name);
    if (t) t.items.push(r);
    else terms.push({ term: r.term_name, items: [r] });
  }
  return terms;
}

export function StudentResults() {
  const state = useLoad<{ items: StudentResult[]; transcripts: Transcript[] }>(() =>
    api.GET("/api/v1/student/results"),
  );
  const [linkError, setLinkError] = useState<string | null>(null);

  return (
    <Page
      title="Results"
      intro="Your results appear here once your Head of School approves them and the registrar releases them."
    >
      <Loaded state={state} label="Loading your results">
        {({ items, transcripts }) => (
          <>
            {items.length === 0 ? (
              <EmptyState
                title="No results yet"
                description="When results for your units are released, they appear here."
              />
            ) : (
              byTerm(items).map((t, i) => (
                <section key={t.term} aria-labelledby={`term-${i}`} className="flex flex-col gap-2">
                  <h2 id={`term-${i}`} className="text-xl font-semibold">
                    {t.term}
                  </h2>
                  <ul className="flex flex-col gap-2">
                    {t.items.map((r) => (
                      <li
                        key={r.unit_code}
                        className="flex flex-col gap-2 rounded-card border border-stone bg-white p-4 sm:flex-row sm:items-center sm:justify-between"
                      >
                        <span className="flex flex-col">
                          <span className="font-semibold">
                            <span className="font-mono">{r.unit_code}</span> {r.unit_title}
                          </span>
                          <span className="text-sm text-ink-muted">
                            Released {formatNairobi(r.released_at)}
                            {r.version > 1 ? " (corrected)" : ""}
                          </span>
                        </span>
                        <span className="flex flex-wrap items-center gap-x-4 gap-y-2">
                          {r.outcome ? null : (
                            <dl className="flex gap-4">
                              <div className="flex gap-1">
                                <dt>Mark</dt>
                                <dd className="font-semibold tabular-nums">{r.total}</dd>
                              </div>
                              <div className="flex gap-1">
                                <dt>Grade</dt>
                                <dd className="font-semibold">{r.grade}</dd>
                              </div>
                            </dl>
                          )}
                          <StatusTag status={r.passed ? "success" : "danger"}>
                            {r.outcome ? outcomeLabel[r.outcome as Outcome] : r.passed ? "Pass" : "Fail"}
                          </StatusTag>
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              ))
            )}

            {items.length > 0 ? (
              <Card className="flex flex-col gap-3">
                <h2 className="text-xl font-semibold">Results slip</h2>
                <p>
                  A PDF of the results on this page, for your own records. It is not an official transcript.
                </p>
                <a
                  href="/api/v1/student/results/slip"
                  download
                  className={buttonClasses("outline", "self-start")}
                >
                  Download results slip (PDF)
                </a>
              </Card>
            ) : null}

            <section aria-labelledby="transcripts-title" className="flex flex-col gap-3">
              <h2 id="transcripts-title" className="text-xl font-semibold">
                Official transcripts
              </h2>
              {transcripts.length === 0 ? (
                <p>
                  None issued yet. To get one,{" "}
                  <Link href="/student/requests" className="text-royal underline">
                    send a request for a transcript
                  </Link>
                  .
                </p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {transcripts.map((t) => (
                    <li
                      key={t.id}
                      className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4 sm:flex-row sm:items-center sm:justify-between"
                    >
                      <span>
                        <span className="font-mono">{t.number}</span>
                        <span className="block text-sm text-ink-muted">
                          Issued {formatNairobi(t.issued_at)}
                        </span>
                      </span>
                      <Button
                        variant="secondary"
                        onClick={async () =>
                          setLinkError(
                            await openFileLink(() =>
                              api.POST("/api/v1/student/transcripts/{id}/link", {
                                params: { path: { id: t.id } },
                              }),
                            ),
                          )
                        }
                      >
                        Download<span className="sr-only"> transcript {t.number}</span>
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
              {linkError ? (
                <Alert tone="danger" title="Could not open the transcript" live>
                  {linkError}
                </Alert>
              ) : null}
            </section>
          </>
        )}
      </Loaded>
    </Page>
  );
}
