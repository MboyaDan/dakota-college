"use client";

import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusTag } from "@/components/ui/tag";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import {
  categoryLabel,
  describeBalance,
  describePlan,
  invoiceFor,
  methodLabel,
  type Category,
} from "@/lib/finance";
import { formatKes } from "@/lib/money";
import { formatDate } from "@/lib/staff/datetime";
import { Loaded, Page, useLoad } from "./pages";

// The student's own fees (Stage 14b, brief 7.1): where they stand, how to
// pay, invoices, payments with receipts, the statement and the fee
// structure. Everything is the signed-in student's own (AC-08); payments
// count only once the finance office confirms them (AC-11).

type Fees = components["schemas"]["StudentFees"];

const download = "inline-flex min-h-touch items-center text-royal underline";

function Standing({ s }: { s: Fees["summary"] }) {
  return (
    <Card className="flex flex-col gap-3">
      <h2 className="text-xl font-semibold">Where you stand</h2>
      <p className="text-2xl font-semibold tabular-nums">{describeBalance(s.balance_cents)}</p>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-2 min-[420px]:grid-cols-[auto_1fr]">
        {s.overdue_cents > 0 ? (
          <>
            <dt className="text-ink-muted">Overdue now</dt>
            <dd className="font-semibold text-danger tabular-nums">{formatKes(s.overdue_cents)}</dd>
          </>
        ) : null}
        <dt className="text-ink-muted">Next due</dt>
        <dd className="tabular-nums">
          {s.next_due_on
            ? `${formatKes(s.next_due_cents)} by ${formatDate(s.next_due_on)}`
            : "Nothing more is due yet"}
        </dd>
        {s.pending_cents > 0 ? (
          <>
            <dt className="text-ink-muted">Being confirmed</dt>
            <dd className="tabular-nums">
              {formatKes(s.pending_cents)}. It counts once the finance office has checked it.
            </dd>
          </>
        ) : null}
      </dl>
      <a href="/api/v1/student/fees/statement" download className={download}>
        Download your fee statement (PDF)
      </a>
    </Card>
  );
}

export function StudentFees() {
  const state = useLoad<Fees>(() => api.GET("/api/v1/student/fees"));
  return (
    <Page title="Fees" intro="Your invoices, the payments the finance office has recorded, and your balance.">
      <Loaded state={state} label="Loading your fees">
        {(f) => (
          <>
            <Standing s={f.summary} />

            {f.payment_instructions || f.mpesa_paybill ? (
              <section aria-labelledby="how-title" className="flex flex-col gap-2">
                <h2 id="how-title" className="text-xl font-semibold">
                  How to pay
                </h2>
                {f.mpesa_paybill ? (
                  <Card className="flex flex-col gap-1">
                    <p className="font-semibold">M-Pesa</p>
                    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                      <dt className="text-ink-muted">Paybill</dt>
                      <dd className="font-mono font-semibold">{f.mpesa_paybill}</dd>
                      <dt className="text-ink-muted">Account number</dt>
                      <dd className="font-mono font-semibold whitespace-nowrap">{f.account_number}</dd>
                    </dl>
                    <p className="text-sm text-ink-muted">
                      Use your student number exactly as shown. The payment shows here, with its receipt,
                      within a few minutes.
                    </p>
                  </Card>
                ) : null}
                {f.payment_instructions ? (
                  <p className="whitespace-pre-line">{f.payment_instructions}</p>
                ) : null}
                {f.payment_instructions && !f.mpesa_paybill ? (
                  <p className="text-sm text-ink-muted">
                    Use your student number as the reference. A payment shows here once the finance office
                    records it.
                  </p>
                ) : null}
              </section>
            ) : null}

            <section aria-labelledby="invoices-title" className="flex flex-col gap-3">
              <h2 id="invoices-title" className="text-xl font-semibold">
                Invoices
              </h2>
              {f.invoices.length === 0 ? (
                <EmptyState
                  title="No invoices yet"
                  description="When the finance office issues an invoice, it appears here."
                />
              ) : (
                f.invoices.map((inv) => (
                  <article
                    key={inv.id}
                    className="flex flex-col gap-3 rounded-card border border-stone bg-white p-4"
                  >
                    <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
                      <h3 className="text-lg font-semibold">{invoiceFor(inv.kind, inv.term_name)}</h3>
                      <span className="font-mono text-sm">{inv.number}</span>
                    </div>
                    <ul className="flex flex-col divide-y divide-stone">
                      {inv.lines.map((l, i) => (
                        <li key={i} className="flex flex-wrap justify-between gap-x-4 py-1">
                          <span>
                            {l.label}{" "}
                            <span className="text-sm text-ink-muted">
                              ({categoryLabel[l.category as Category]})
                            </span>
                          </span>
                          <span className="tabular-nums">{formatKes(l.amount_cents)}</span>
                        </li>
                      ))}
                    </ul>
                    <p className="flex justify-between font-semibold">
                      <span>Total</span>
                      <span className="tabular-nums">{formatKes(inv.total_cents)}</span>
                    </p>
                    <div>
                      <p className="font-semibold">Due</p>
                      <ul className="flex flex-col">
                        {inv.instalments.map((n) => (
                          <li key={n.seq} className="tabular-nums">
                            {n.amount_cents !== null ? formatKes(n.amount_cents) : ""} by{" "}
                            {formatDate(n.due_on)}
                          </li>
                        ))}
                      </ul>
                    </div>
                    <a href={`/api/v1/student/fees/invoices/${inv.id}`} download className={download}>
                      Download invoice<span className="sr-only"> {inv.number}</span>&nbsp;(PDF)
                    </a>
                  </article>
                ))
              )}
            </section>

            <section aria-labelledby="payments-title" className="flex flex-col gap-3">
              <h2 id="payments-title" className="text-xl font-semibold">
                Payments
              </h2>
              {f.payments.length === 0 ? (
                <p>No payments recorded yet.</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {f.payments.map((p) => (
                    <li
                      key={p.id}
                      className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4"
                    >
                      <span className="flex flex-wrap items-center justify-between gap-2">
                        <span className="font-semibold tabular-nums">
                          {formatKes(p.amount_cents)} · {methodLabel[p.method]}
                        </span>
                        {p.status === "confirmed" ? (
                          <StatusTag status="success">Confirmed</StatusTag>
                        ) : (
                          <StatusTag status="warning">Being confirmed</StatusTag>
                        )}
                      </span>
                      <span className="text-sm text-ink-muted">
                        Paid {formatDate(p.paid_on)}
                        {p.reference ? ` · ${p.reference}` : ""}
                      </span>
                      {p.receipt_number ? (
                        <a
                          href={`/api/v1/student/fees/payments/${p.id}/receipt`}
                          download
                          className="min-h-touch content-center self-start text-royal underline"
                        >
                          Download receipt{" "}
                          <span className="font-mono whitespace-nowrap">{p.receipt_number}</span>
                        </a>
                      ) : (
                        <span className="text-sm">The receipt is issued once it is confirmed.</span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section aria-labelledby="statement-title" className="flex flex-col gap-3">
              <h2 id="statement-title" className="text-xl font-semibold">
                Statement
              </h2>
              {f.statement.length === 0 ? (
                <p>Nothing has been charged or paid yet.</p>
              ) : (
                <ol className="flex flex-col gap-2">
                  {f.statement.map((e, i) => (
                    <li key={i} className="rounded-card border border-stone bg-white p-3">
                      <span className="flex flex-wrap justify-between gap-x-4">
                        <span>{e.description}</span>
                        <span className="font-semibold tabular-nums">
                          {e.debit_cents ? `+ ${formatKes(e.debit_cents)}` : `- ${formatKes(e.credit_cents)}`}
                        </span>
                      </span>
                      <span className="flex flex-wrap justify-between gap-x-4 text-sm text-ink-muted">
                        <span>
                          {formatDate(e.date)}
                          {e.reference && !e.description.includes(e.reference) ? ` · ${e.reference}` : ""}
                        </span>
                        <span className="tabular-nums">Balance {formatKes(e.balance_cents)}</span>
                      </span>
                    </li>
                  ))}
                </ol>
              )}
            </section>

            {f.structure ? (
              <section aria-labelledby="structure-title" className="flex flex-col gap-3">
                <h2 id="structure-title" className="text-xl font-semibold">
                  Your programme&apos;s fees
                </h2>
                <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white px-4">
                  {f.structure.lines.map((l, i) => (
                    <li key={i} className="flex flex-wrap justify-between gap-x-4 py-2">
                      <span>
                        {l.label}{" "}
                        <span className="text-sm text-ink-muted">
                          ({l.charge === "once" ? "once, at enrolment" : "every term"})
                        </span>
                      </span>
                      <span className="tabular-nums">{formatKes(l.amount_cents)}</span>
                    </li>
                  ))}
                </ul>
                <p className="text-sm text-ink-muted">
                  Each invoice is split: {describePlan(f.structure.instalments)}
                </p>
              </section>
            ) : null}
          </>
        )}
      </Loaded>
    </Page>
  );
}
