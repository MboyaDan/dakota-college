"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input, Select, Textarea } from "@/components/ui/field";
import { Table } from "@/components/ui/table";
import { api } from "@/lib/api/client";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { amountHint, describeBalance, invoiceFor, newIdempotencyKey, parseAmount } from "@/lib/finance";
import { formatKes } from "@/lib/money";
import {
  InvoiceStatusTag,
  LoadState,
  PdfLink,
  useLoaded,
  type Account,
  type StatementEntry,
} from "./finance-common";
import { PaymentDecision, PaymentItem, RecordPayment } from "./finance-payments";
import { useSession } from "./session";
import { useAction } from "./use-action";

// A student's account for finance staff (Stage 14b, AC-10): where it
// stands, the statement made from records, and the actions the viewer's
// permissions allow.

export function AccountSummary({ s }: { s: Account["summary"] }) {
  return (
    <dl className="grid grid-cols-1 gap-x-6 gap-y-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
      <div>
        <dt className="text-sm text-ink-muted">Balance</dt>
        <dd className="text-xl font-semibold tabular-nums">{describeBalance(s.balance_cents)}</dd>
      </div>
      <div>
        <dt className="text-sm text-ink-muted">Overdue now</dt>
        <dd
          className={
            s.overdue_cents > 0 ? "text-xl font-semibold text-danger tabular-nums" : "text-xl tabular-nums"
          }
        >
          {formatKes(s.overdue_cents)}
        </dd>
      </div>
      <div>
        <dt className="text-sm text-ink-muted">Next due</dt>
        <dd className="text-xl tabular-nums">
          {s.next_due_on ? `${formatKes(s.next_due_cents)} by ${formatDate(s.next_due_on)}` : "Nothing"}
        </dd>
      </div>
      <div>
        <dt className="text-sm text-ink-muted">Waiting for confirmation</dt>
        <dd className="text-xl tabular-nums">{formatKes(s.pending_cents)}</dd>
      </div>
    </dl>
  );
}

export function StatementTable({ rows }: { rows: StatementEntry[] }) {
  return (
    <Table
      caption="Statement"
      captionHidden
      rows={rows}
      rowKey={(r) => `${r.date}-${r.kind}-${r.reference}-${r.balance_cents}-${r.description}`}
      empty="Nothing has been charged or paid yet."
      columns={[
        {
          key: "date",
          header: "Date",
          cell: (r) => <span className="whitespace-nowrap">{formatDate(r.date)}</span>,
        },
        {
          key: "details",
          header: "Details",
          cell: (r) => (
            <>
              {r.description}
              {r.reference && !r.description.includes(r.reference) ? (
                <span className="block font-mono text-sm">{r.reference}</span>
              ) : null}
            </>
          ),
        },
        {
          key: "debit",
          header: "Charged",
          numeric: true,
          cell: (r) => (r.debit_cents ? formatKes(r.debit_cents) : ""),
        },
        {
          key: "credit",
          header: "Paid",
          numeric: true,
          cell: (r) => (r.credit_cents ? formatKes(r.credit_cents) : ""),
        },
        { key: "balance", header: "Balance", numeric: true, cell: (r) => formatKes(r.balance_cents) },
      ]}
    />
  );
}

function NewInvoice({ studentId }: { studentId: string }) {
  const router = useRouter();
  const [kind, setKind] = useState<"term" | "other">("term");
  const [term, setTerm] = useState("");
  const terms = useLoaded(() => api.GET("/api/v1/staff/terms"), []);
  const { busy, errors, run } = useAction();
  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        const inv = await run(
          () =>
            api.POST("/api/v1/staff/finance/students/{id}/invoices", {
              params: { path: { id: studentId } },
              body: { kind, term_id: kind === "term" ? term || null : null },
            }),
          "Draft invoice opened",
          "invoice.",
        );
        if (inv) router.push(`/staff/finance/invoices/${inv.id}`);
      }}
    >
      <Select
        id="new-invoice-kind"
        name="kind"
        label="Invoice for"
        required
        value={kind}
        error={errors["invoice.kind"]}
        onChange={(e) => setKind(e.target.value as "term" | "other")}
      >
        <option value="term">A term</option>
        <option value="other">Something else</option>
      </Select>
      {kind === "term" ? (
        <Select
          id="new-invoice-term"
          name="term_id"
          label="Term"
          required
          value={term}
          error={errors["invoice.term_id"]}
          onChange={(e) => setTerm(e.target.value)}
        >
          <option value="">Choose a term</option>
          {(terms.data?.items ?? []).map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </Select>
      ) : null}
      <Button type="submit" variant="outline" loading={busy} className="self-start">
        Open a draft invoice
      </Button>
    </form>
  );
}

function NewAdjustment({ account, onDone }: { account: Account; onDone: () => void }) {
  const [kind, setKind] = useState<"charge" | "credit">("credit");
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [link, setLink] = useState("");
  const key = useRef<string | null>(null);
  const { busy, errors, setErrors, run } = useAction();
  const issued = account.invoices.filter((i) => i.status === "issued");
  const confirmed = account.payments.filter((p) => p.status === "confirmed");

  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        const cents = parseAmount(amount);
        if (cents === null) {
          setErrors({ "adjust.amount_cents": `Enter the amount. ${amountHint}` });
          return;
        }
        key.current ??= newIdempotencyKey();
        const [type, id] = link.split(":");
        const done = await run(
          () =>
            api.POST("/api/v1/staff/finance/students/{id}/adjustments", {
              params: { path: { id: account.student.id }, header: { "Idempotency-Key": key.current! } },
              body: {
                kind,
                amount_cents: cents,
                reason,
                invoice_id: type === "invoice" ? id : null,
                payment_id: type === "payment" ? id : null,
              },
            }),
          kind === "credit" ? "Credit recorded" : "Charge recorded",
          "adjust.",
        );
        if (done) {
          key.current = null;
          setAmount("");
          setReason("");
          setLink("");
          onDone();
        }
      }}
    >
      <Select
        id="adjust-kind"
        name="kind"
        label="Kind"
        required
        value={kind}
        error={errors["adjust.kind"]}
        onChange={(e) => setKind(e.target.value as "charge" | "credit")}
      >
        <option value="credit">Credit: the student owes less</option>
        <option value="charge">Charge: the student owes more</option>
      </Select>
      <Input
        id="adjust-amount"
        name="amount"
        label="Amount (KES)"
        required
        inputMode="decimal"
        autoComplete="off"
        hint={amountHint}
        value={amount}
        error={errors["adjust.amount_cents"]}
        onChange={(e) => setAmount(e.target.value)}
      />
      <Textarea
        id="adjust-reason"
        name="reason"
        label="Reason"
        required
        rows={2}
        maxLength={1000}
        hint="The student sees this on their statement. For example: bursary for Term 1, or cheque returned by the bank."
        value={reason}
        error={errors["adjust.reason"]}
        onChange={(e) => setReason(e.target.value)}
      />
      <Select
        id="adjust-link"
        name="link"
        label="What it corrects"
        value={link}
        error={errors["adjust.invoice_id"] ?? errors["adjust.payment_id"]}
        onChange={(e) => setLink(e.target.value)}
      >
        <option value="">Nothing in particular</option>
        {issued.map((i) => (
          <option key={i.id} value={`invoice:${i.id}`}>
            Invoice {i.number}
          </option>
        ))}
        {confirmed.map((p) => (
          <option key={p.id} value={`payment:${p.id}`}>
            Payment {p.receipt_number} ({formatKes(p.amount_cents)})
          </option>
        ))}
      </Select>
      <p className="text-sm">
        An adjustment cannot be edited or removed. A mistake is corrected with another one.
      </p>
      <Button type="submit" variant="outline" loading={busy} className="self-start">
        Record {kind === "credit" ? "credit" : "charge"}
      </Button>
    </form>
  );
}

export function StudentAccount({ id }: { id: string }) {
  const { can } = useSession();
  const {
    data: account,
    error,
    reload,
  } = useLoaded(() => api.GET("/api/v1/staff/finance/students/{id}", { params: { path: { id } } }), [id]);
  if (!account) return <LoadState error={error} label="Loading the account" />;
  const st = account.student;

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "Finance", href: "/staff/finance" }, { label: st.name }]} />
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold break-words">Fees: {st.name}</h1>
        <p className="text-ink-muted">
          <span className="font-mono">{st.student_number}</span> · {st.programme_title}
          {st.intake_name ? ` · ${st.intake_name}` : ""}
        </p>
        <div className="flex flex-wrap gap-x-6">
          <PdfLink href={`/api/v1/staff/finance/students/${id}/statement`}>Download statement (PDF)</PdfLink>
          {can("students.view") ? (
            <Link
              href={`/staff/students/${id}`}
              className="inline-flex min-h-touch items-center text-royal underline"
            >
              Student record
            </Link>
          ) : null}
        </div>
      </div>

      <Card>
        <h2 className="mb-3 text-xl font-semibold">Where the account stands</h2>
        <AccountSummary s={account.summary} />
      </Card>

      <section aria-labelledby="statement-title" className="flex flex-col gap-2">
        <h2 id="statement-title" className="text-xl font-semibold">
          Statement
        </h2>
        <p className="text-sm text-ink-muted">
          Issued invoices, confirmed payments and adjustments, made from the records each time you open it.
        </p>
        <StatementTable rows={account.statement} />
      </section>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {can("finance.payment.record") ? (
          <Card className="flex flex-col gap-3">
            <h2 className="text-xl font-semibold">Record a payment</h2>
            <RecordPayment studentId={id} onDone={reload} />
          </Card>
        ) : null}
        {can("finance.adjust") ? (
          <Card className="flex flex-col gap-3">
            <h2 className="text-xl font-semibold">Adjust the account</h2>
            <NewAdjustment account={account} onDone={reload} />
          </Card>
        ) : null}
      </div>

      <section aria-labelledby="invoices-title" className="flex flex-col gap-2">
        <h2 id="invoices-title" className="text-xl font-semibold">
          Invoices
        </h2>
        {account.invoices.length === 0 ? <p>No invoices yet.</p> : null}
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white empty:hidden">
          {account.invoices.map((i) => (
            <li key={i.id}>
              <Link
                href={`/staff/finance/invoices/${i.id}`}
                className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:justify-between"
              >
                <span className="flex flex-col">
                  <span className="font-semibold text-royal">
                    {i.number ? <span className="font-mono">{i.number}</span> : "Draft"} ·{" "}
                    {invoiceFor(i.kind, i.term_name)}
                  </span>
                  <span className="text-sm text-ink-muted">
                    {formatKes(i.total_cents)} ·{" "}
                    {i.issued_at
                      ? `issued ${formatNairobi(i.issued_at)}`
                      : `opened ${formatNairobi(i.created_at)}`}
                  </span>
                </span>
                <InvoiceStatusTag status={i.status} />
              </Link>
            </li>
          ))}
        </ul>
        {can("finance.invoice") ? (
          <Card className="mt-2 flex flex-col gap-3">
            <h3 className="text-lg font-semibold">New invoice</h3>
            <NewInvoice studentId={id} />
          </Card>
        ) : null}
      </section>

      <section aria-labelledby="payments-title" className="flex flex-col gap-2">
        <h2 id="payments-title" className="text-xl font-semibold">
          Payments
        </h2>
        {account.payments.length === 0 ? (
          <p>No payments recorded.</p>
        ) : (
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {account.payments.map((p) => (
              <PaymentItem
                key={p.id}
                p={p}
                showStudent={false}
                receiptHref={`/api/v1/staff/finance/payments/${p.id}/receipt`}
              >
                {p.status === "pending" && can("finance.payment.confirm") ? (
                  <PaymentDecision payment={{ ...p, student_name: st.name }} onDone={reload} />
                ) : null}
              </PaymentItem>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="adjustments-title" className="flex flex-col gap-2">
        <h2 id="adjustments-title" className="text-xl font-semibold">
          Adjustments
        </h2>
        {account.adjustments.length === 0 ? (
          <p>None.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {account.adjustments.map((a) => (
              <li key={a.id} className="rounded-control border border-stone bg-white px-3 py-2">
                <span className="font-semibold tabular-nums">
                  {a.kind === "credit" ? "Credit" : "Charge"} {formatKes(a.amount_cents)}
                </span>{" "}
                {a.invoice_number ? (
                  <span className="font-mono text-sm">(invoice {a.invoice_number})</span>
                ) : a.receipt_number ? (
                  <span className="font-mono text-sm">(receipt {a.receipt_number})</span>
                ) : null}
                <span className="block">{a.reason}</span>
                <span className="block text-sm text-ink-muted">
                  {formatNairobi(a.created_at)} by {a.created_by_name}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
