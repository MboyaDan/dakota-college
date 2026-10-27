"use client";

import Link from "next/link";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select, Textarea } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { api } from "@/lib/api/client";
import { formatNairobi, formatDate } from "@/lib/staff/datetime";
import {
  amountHint,
  methodLabel,
  newIdempotencyKey,
  normaliseMpesaCode,
  parseAmount,
  type Method,
} from "@/lib/finance";
import { formatKes } from "@/lib/money";
import { LoadState, PaymentStatusTag, PdfLink, useLoaded, type Payment } from "./finance-common";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Payments (Stage 14b, D-159): recording money received, and the queue of
// payments waiting to be checked against the bank or M-Pesa statement.
// Each action keeps one Idempotency-Key until it succeeds, so a retry
// after a lost connection never records or confirms twice (NN-04).

/** Today in Nairobi as YYYY-MM-DD. */
export function todayNairobi(now = new Date()): string {
  return new Date(now.getTime() + 3 * 60 * 60 * 1000).toISOString().slice(0, 10);
}

/** One key per action, kept until that action succeeds. */
function useKeys() {
  const keys = useRef(new Map<string, string>());
  return {
    get(action: string) {
      let k = keys.current.get(action);
      if (!k) {
        k = newIdempotencyKey();
        keys.current.set(action, k);
      }
      return k;
    },
    done(action: string) {
      keys.current.delete(action);
    },
  };
}

// ---------- recording ----------

export function RecordPayment({ studentId, onDone }: { studentId: string; onDone: () => void }) {
  const { can } = useSession();
  const [method, setMethod] = useState<Method>("mpesa");
  const [amount, setAmount] = useState("");
  const [paidOn, setPaidOn] = useState(todayNairobi());
  const [reference, setReference] = useState("");
  const [payer, setPayer] = useState("");
  const [note, setNote] = useState("");
  const keys = useKeys();
  const { show } = useToast();
  const { busy, errors, setErrors, run } = useAction();
  const receiptedAtOnce = method === "cash" && can("finance.payment.confirm");

  async function submit() {
    const cents = parseAmount(amount);
    if (cents === null) {
      setErrors({ amount_cents: `Enter the amount received. ${amountHint}` });
      return;
    }
    const body = {
      student_id: studentId,
      method,
      amount_cents: cents,
      paid_on: paidOn,
      reference: method === "mpesa" ? normaliseMpesaCode(reference) : reference,
      payer_name: payer,
      note,
    };
    const p = await run(
      () =>
        api.POST("/api/v1/staff/finance/payments", {
          params: { header: { "Idempotency-Key": keys.get("record") } },
          body,
        }),
      "",
    );
    if (!p) return;
    keys.done("record");
    setErrors({});
    setAmount("");
    setReference("");
    setPayer("");
    setNote("");
    // Say what happened: receipted at once, or waiting for confirmation.
    show({
      tone: "success",
      title:
        p.status === "confirmed"
          ? `Receipted: ${p.receipt_number}`
          : "Payment recorded. It counts once it is confirmed.",
    });
    onDone();
  }

  return (
    <form
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Select
          id="payment-method"
          name="method"
          label="How was it paid?"
          required
          value={method}
          error={errors.method}
          onChange={(e) => setMethod(e.target.value as Method)}
        >
          {(Object.keys(methodLabel) as Method[]).map((m) => (
            <option key={m} value={m}>
              {methodLabel[m]}
            </option>
          ))}
        </Select>
        <Input
          id="payment-amount"
          name="amount"
          label="Amount (KES)"
          required
          inputMode="decimal"
          autoComplete="off"
          hint={amountHint}
          value={amount}
          error={errors.amount_cents}
          onChange={(e) => setAmount(e.target.value)}
        />
        <Input
          id="payment-date"
          name="paid_on"
          type="date"
          label="Date paid"
          required
          max={todayNairobi()}
          value={paidOn}
          error={errors.paid_on}
          onChange={(e) => setPaidOn(e.target.value)}
        />
        {method === "cash" ? null : (
          <Input
            id="payment-reference"
            name="reference"
            label={
              method === "mpesa"
                ? "M-Pesa transaction code"
                : method === "bank"
                  ? "Bank slip or transfer reference"
                  : "Reference (if any)"
            }
            required={method === "mpesa" || method === "bank"}
            autoComplete="off"
            maxLength={60}
            hint={method === "mpesa" ? "The 10 letters and numbers on the M-Pesa message." : undefined}
            className={method === "mpesa" ? "font-mono uppercase" : undefined}
            value={reference}
            error={errors.reference}
            onChange={(e) => setReference(e.target.value)}
          />
        )}
        {method === "sponsor" ? (
          <Input
            id="payment-payer"
            name="payer_name"
            label="Sponsor's name"
            required
            maxLength={120}
            value={payer}
            error={errors.payer_name}
            onChange={(e) => setPayer(e.target.value)}
          />
        ) : null}
      </div>
      <Textarea
        id="payment-note"
        name="note"
        label="Note"
        rows={2}
        maxLength={1000}
        value={note}
        error={errors.note}
        onChange={(e) => setNote(e.target.value)}
      />
      <p className="text-sm">
        {receiptedAtOnce
          ? "Cash you record is confirmed and receipted at once."
          : "This payment waits until someone with confirmation access checks it against the statement. It does not count until then."}
      </p>
      <Button type="submit" variant="secondary" loading={busy} className="self-start">
        Record payment
      </Button>
    </form>
  );
}

// ---------- confirming and rejecting ----------

export function PaymentDecision({ payment, onDone }: { payment: Payment; onDone: (p: Payment) => void }) {
  const keys = useKeys();
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const { busy, errors, setErrors, run } = useAction();
  const path = { id: payment.id };
  const what = `${formatKes(payment.amount_cents)} by ${methodLabel[payment.method]}${payment.reference ? ` (${payment.reference})` : ""}`;
  const fieldId = `reject-${payment.id}`;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-3">
        <Dialog
          title="Confirm this payment?"
          description={`${what} for ${payment.student_name ?? "this student"}, paid on ${formatDate(payment.paid_on)}.`}
          triggerLabel="Confirm"
          triggerVariant="secondary"
          actions={
            <>
              <Button type="submit" variant="outline">
                Cancel
              </Button>
              <Button
                type="submit"
                onClick={async () => {
                  const p = await run(
                    () =>
                      api.POST("/api/v1/staff/finance/payments/{id}/confirm", {
                        params: { path, header: { "Idempotency-Key": keys.get("confirm") } },
                        body: {},
                      }),
                    "Payment confirmed and receipted",
                  );
                  if (p) {
                    keys.done("confirm");
                    onDone(p);
                  }
                }}
              >
                Confirm and issue receipt
              </Button>
            </>
          }
        >
          <p>
            Confirm only after you have found it on the{" "}
            {payment.method === "mpesa" ? "M-Pesa" : payment.method === "cash" ? "cash book" : "bank"}{" "}
            statement. A confirmed payment cannot be undone; a mistake is corrected with an adjustment.
          </p>
        </Dialog>
        {rejecting ? null : (
          <Button variant="outline" onClick={() => setRejecting(true)}>
            Reject<span className="sr-only"> {what}</span>
          </Button>
        )}
      </div>
      {rejecting ? (
        <form
          noValidate
          className="flex flex-col gap-3"
          onSubmit={async (e) => {
            e.preventDefault();
            if (reason.trim().length < 5) {
              setErrors({ reason: "Say why it is rejected, in at least 5 characters." });
              return;
            }
            const p = await run(
              () =>
                api.POST("/api/v1/staff/finance/payments/{id}/reject", {
                  params: { path, header: { "Idempotency-Key": keys.get("reject") } },
                  body: { reason },
                }),
              "Payment rejected",
            );
            if (p) {
              keys.done("reject");
              onDone(p);
            }
          }}
        >
          <Textarea
            id={fieldId}
            name="reason"
            label="Why is it rejected?"
            required
            rows={2}
            maxLength={1000}
            hint="For example: not on the bank statement for that date."
            value={reason}
            error={errors.reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <div className="flex flex-wrap gap-3">
            <Button type="submit" variant="danger" loading={busy}>
              Reject payment
            </Button>
            <Button variant="ghost" onClick={() => setRejecting(false)}>
              Cancel
            </Button>
          </div>
        </form>
      ) : null}
    </div>
  );
}

/** One payment in a list: what, whose, and its state. */
export function PaymentItem({
  p,
  showStudent,
  receiptHref,
  children,
}: {
  p: Payment;
  showStudent: boolean;
  receiptHref: string;
  children?: React.ReactNode;
}) {
  return (
    <li className="flex flex-col gap-3 p-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col">
          <span className="font-semibold tabular-nums">
            {formatKes(p.amount_cents)} · {methodLabel[p.method]}
            {p.reference ? <span className="font-mono"> {p.reference}</span> : null}
          </span>
          {showStudent ? (
            <Link href={`/staff/finance/students/${p.student_id}`} className="text-royal underline">
              <span className="font-mono">{p.student_number}</span> {p.student_name}
            </Link>
          ) : null}
          <span className="text-sm text-ink-muted">
            Paid {formatDate(p.paid_on)}
            {p.payer_name ? ` by ${p.payer_name}` : ""} · recorded {formatNairobi(p.recorded_at)}
            {p.recorded_by_name ? ` by ${p.recorded_by_name}` : ""}
          </span>
          {p.decision_note && p.status === "rejected" ? (
            <span className="text-sm">Reason: {p.decision_note}</span>
          ) : null}
        </div>
        <div className="flex flex-col items-start gap-1 sm:items-end">
          <PaymentStatusTag status={p.status} />
          {p.receipt_number ? (
            <PdfLink href={receiptHref}>
              Receipt <span className="ml-1 font-mono">{p.receipt_number}</span>
            </PdfLink>
          ) : null}
        </div>
      </div>
      {children}
    </li>
  );
}

// ---------- the queue ----------

const statuses = [
  { value: "pending", label: "Waiting for confirmation" },
  { value: "confirmed", label: "Confirmed" },
  { value: "rejected", label: "Rejected" },
] as const;

export function PaymentQueue() {
  const { can } = useSession();
  const [status, setStatus] = useState<(typeof statuses)[number]["value"]>("pending");
  const [method, setMethod] = useState<"" | Method>("");
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const { data, setData, error } = useLoaded(
    () =>
      api.GET("/api/v1/staff/finance/payments", {
        params: { query: { status, method: method || undefined, q: search || undefined, page } },
      }),
    [status, method, search, page],
  );
  const pages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Payments</h1>
        <p className="max-w-prose text-ink-muted">
          Bank, M-Pesa and sponsor payments wait here until they are checked against the statement. They count
          towards a student&apos;s balance only once confirmed. Record a payment from the student&apos;s
          account.
        </p>
      </div>
      <form
        role="search"
        className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[1fr_1fr_2fr_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          setSearch(q.trim());
        }}
      >
        <Select
          id="payments-status"
          name="status"
          label="Show"
          required
          value={status}
          onChange={(e) => {
            setPage(1);
            setStatus(e.target.value as typeof status);
          }}
        >
          {statuses.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </Select>
        <Select
          id="payments-method"
          name="method"
          label="Method"
          value={method}
          onChange={(e) => {
            setPage(1);
            setMethod(e.target.value as typeof method);
          }}
        >
          <option value="">Any</option>
          {(Object.keys(methodLabel) as Method[]).map((m) => (
            <option key={m} value={m}>
              {methodLabel[m]}
            </option>
          ))}
        </Select>
        <Input
          id="payments-q"
          name="q"
          type="search"
          label="Search"
          hint="Reference, receipt number, student number or name."
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <Button type="submit" variant="outline">
          Search
        </Button>
      </form>

      {!data ? (
        <LoadState error={error} label="Loading payments" />
      ) : data.items.length === 0 ? (
        <EmptyState
          title="No payments here"
          description={
            status === "pending" && !search && !method
              ? "Nothing is waiting for confirmation."
              : "No payments match. Try another filter."
          }
        />
      ) : (
        <>
          <p role="status">{data.total === 1 ? "1 payment" : `${data.total} payments`}</p>
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {data.items.map((p) => (
              <PaymentItem
                key={p.id}
                p={p}
                showStudent
                receiptHref={`/api/v1/staff/finance/payments/${p.id}/receipt`}
              >
                {p.status === "pending" && can("finance.payment.confirm") ? (
                  <PaymentDecision
                    payment={p}
                    onDone={(next) =>
                      setData({
                        ...data,
                        items: data.items.map((x) => (x.id === next.id ? { ...x, ...next } : x)),
                      })
                    }
                  />
                ) : null}
              </PaymentItem>
            ))}
          </ul>
          {pages > 1 ? (
            <nav aria-label="Pages of payments" className="flex flex-wrap items-center gap-3">
              <Button variant="outline" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                Previous
              </Button>
              <span>
                Page {page} of {pages}
              </span>
              <Button variant="outline" disabled={page >= pages} onClick={() => setPage(page + 1)}>
                Next
              </Button>
            </nav>
          ) : null}
        </>
      )}
    </div>
  );
}
