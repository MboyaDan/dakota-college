"use client";

import Link from "next/link";
import { useId, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select, Textarea } from "@/components/ui/field";
import { StatusTag, type Status } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { parseAmount } from "@/lib/finance";
import { formatKes } from "@/lib/money";
import { formatNairobi } from "@/lib/staff/datetime";
import { LoadState, useLoaded } from "./finance-common";
import { todayNairobi } from "./finance-payments";
import { useSession } from "./session";
import { useAction } from "./use-action";

// M-Pesa on the college paybill (Stage 15a, D-169 to D-172). Payments whose
// account number is a student's number are credited by themselves; the
// rest wait here for Finance to assign or return.

type Transaction = components["schemas"]["MpesaTransaction"];
type TxStatus = Transaction["status"];

export const txStatus: Record<TxStatus, { label: string; tone: Status }> = {
  matched: { label: "Credited", tone: "success" },
  assigned: { label: "Assigned by Finance", tone: "success" },
  unmatched: { label: "Waiting for Finance", tone: "warning" },
  refund: { label: "To be returned", tone: "info" },
  rejected: { label: "Not for this paybill", tone: "danger" },
};

export function FindStudent({ onPick }: { onPick: (id: string, label: string) => void }) {
  const fieldId = useId();
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");
  const { data } = useLoaded(
    () =>
      search
        ? api.GET("/api/v1/staff/students", { params: { query: { q: search } } })
        : Promise.resolve({ data: undefined, error: undefined }),
    [search],
  );
  return (
    <div className="flex flex-col gap-2">
      <div className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[1fr_auto]">
        <Input
          id={`find-${fieldId}`}
          name="q"
          type="search"
          label="Find the student"
          required
          hint="Student number, name or email."
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              setSearch(q.trim());
            }
          }}
        />
        <Button variant="outline" onClick={() => setSearch(q.trim())}>
          Find
        </Button>
      </div>
      {search && data ? (
        data.items.length === 0 ? (
          <p role="status">No student matches.</p>
        ) : (
          <ul className="flex flex-col gap-1">
            {data.items.slice(0, 6).map((s) => (
              <li key={s.id}>
                <Button variant="ghost" onClick={() => onPick(s.id, `${s.student_number} ${s.name}`)}>
                  Choose <span className="ml-1 font-mono">{s.student_number}</span>&nbsp;{s.name}
                </Button>
              </li>
            ))}
          </ul>
        )
      ) : null}
    </div>
  );
}

/** Assign an unmatched provider payment to a student, or mark it to be returned. */
export function Settle({
  t,
  provider,
  onDone,
}: {
  t: Pick<Transaction, "id" | "trans_id">;
  provider: "mpesa" | "kcb";
  onDone: () => void;
}) {
  const [mode, setMode] = useState<"assign" | "refund" | null>(null);
  const [student, setStudent] = useState<{ id: string; label: string } | null>(null);
  const [note, setNote] = useState("");
  const { busy, errors, setErrors, run } = useAction();
  const path = { params: { path: { id: t.id } } };

  if (!mode) {
    return (
      <div className="flex flex-wrap gap-3">
        <Button variant="secondary" onClick={() => setMode("assign")}>
          Assign to a student<span className="sr-only"> {t.trans_id}</span>
        </Button>
        <Button variant="outline" onClick={() => setMode("refund")}>
          To be returned<span className="sr-only"> {t.trans_id}</span>
        </Button>
      </div>
    );
  }
  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        if (mode === "assign" && !student) {
          setErrors({ student_id: "Find and choose the student first." });
          return;
        }
        if (note.trim().length < 5) {
          setErrors({ note: "Write at least 5 characters." });
          return;
        }
        const assign = { ...path, body: { student_id: student?.id ?? "", note } };
        const refund = { ...path, body: { note } };
        const ok =
          mode === "assign"
            ? await run(
                () =>
                  provider === "mpesa"
                    ? api.POST("/api/v1/staff/finance/mpesa/transactions/{id}/assign", assign)
                    : api.POST("/api/v1/staff/finance/kcb/transactions/{id}/assign", assign),
                "Assigned and receipted",
              )
            : await run(
                () =>
                  provider === "mpesa"
                    ? api.POST("/api/v1/staff/finance/mpesa/transactions/{id}/refund", refund)
                    : api.POST("/api/v1/staff/finance/kcb/transactions/{id}/refund", refund),
                "Marked to be returned",
              );
        if (ok) onDone();
      }}
    >
      {mode === "assign" ? (
        student ? (
          <p>
            Credit to <span className="font-semibold">{student.label}</span>{" "}
            <button
              type="button"
              className="min-h-touch text-royal underline"
              onClick={() => setStudent(null)}
            >
              Change
            </button>
          </p>
        ) : (
          <>
            <FindStudent onPick={(id, label) => setStudent({ id, label })} />
            {errors.student_id ? <p className="font-medium text-danger">{errors.student_id}</p> : null}
          </>
        )
      ) : null}
      <Textarea
        id={`settle-note-${t.id}`}
        name="note"
        label={mode === "assign" ? "How do you know whose payment it is?" : "Why is it being returned?"}
        required
        rows={2}
        maxLength={1000}
        hint={
          mode === "assign"
            ? provider === "mpesa"
              ? "For example: the parent phoned with the M-Pesa code."
              : "For example: found on the KCB statement; the sponsor gave the student's name."
            : provider === "mpesa"
              ? "Return it from the M-Pesa portal; this only records the decision."
              : "Return it through the bank; this only records the decision."
        }
        value={note}
        error={errors.note}
        onChange={(e) => setNote(e.target.value)}
      />
      <div className="flex flex-wrap gap-3">
        <Button type="submit" variant={mode === "assign" ? "secondary" : "danger"} loading={busy}>
          {mode === "assign" ? "Assign and issue receipt" : "Mark to be returned"}
        </Button>
        <Button variant="ghost" onClick={() => setMode(null)}>
          Cancel
        </Button>
      </div>
    </form>
  );
}

export function TransactionItem({
  t,
  children,
  statusLabel,
  referenceLabel = "account number",
}: {
  t: Transaction;
  children?: React.ReactNode;
  /** Replaces the status label (KCB: credited as pending). */
  statusLabel?: { label: string; tone: Status };
  /** What the payer's reference is called ("account number" for M-Pesa). */
  referenceLabel?: string;
}) {
  const shown = statusLabel ?? txStatus[t.status];
  return (
    <li className="flex flex-col gap-3 p-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col">
          <span className="font-semibold tabular-nums">
            {formatKes(t.amount_cents)} · <span className="font-mono">{t.trans_id}</span>
          </span>
          <span className="text-sm text-ink-muted">
            {formatNairobi(t.trans_time)} · {referenceLabel}{" "}
            <span className="font-mono">{t.account_number || "(none)"}</span>
            {t.payer_name ? ` · ${t.payer_name}` : ""}
          </span>
          {t.student_id ? (
            <Link href={`/staff/finance/students/${t.student_id}`} className="text-royal underline">
              <span className="font-mono">{t.student_number}</span> {t.student_name}
            </Link>
          ) : null}
          {t.reason ? <span className="text-sm">{t.reason}</span> : null}
          {t.note ? (
            <span className="text-sm">
              {t.handled_by_name}: {t.note}
            </span>
          ) : null}
        </div>
        <div className="flex flex-col items-start gap-1 sm:items-end">
          <StatusTag status={shown.tone}>{shown.label}</StatusTag>
          {t.receipt_number ? <span className="font-mono text-sm">{t.receipt_number}</span> : null}
        </div>
      </div>
      {children}
    </li>
  );
}

function Reconciliation() {
  const [day, setDay] = useState(todayNairobi());
  const { data, error } = useLoaded(
    () => api.GET("/api/v1/staff/finance/mpesa/reconciliation", { params: { query: { day } } }),
    [day],
  );
  return (
    <section aria-labelledby="recon-title" className="flex flex-col gap-3">
      <h2 id="recon-title" className="text-xl font-semibold">
        Check a day against the M-Pesa statement
      </h2>
      <div className="sm:max-w-xs">
        <Input
          id="recon-day"
          name="day"
          type="date"
          label="Day"
          required
          max={todayNairobi()}
          value={day}
          onChange={(e) => e.target.value && setDay(e.target.value)}
        />
      </div>
      {!data ? (
        <LoadState error={error} label="Loading" />
      ) : (
        <Card className="flex flex-col gap-3">
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 min-[420px]:grid-cols-[auto_1fr]">
            <dt className="text-ink-muted">Received from Safaricom</dt>
            <dd className="font-semibold tabular-nums">
              {formatKes(data.received_cents)} ({data.received_count} payment
              {data.received_count === 1 ? "" : "s"})
            </dd>
            <dt className="text-ink-muted">Credited to students</dt>
            <dd className="tabular-nums">{formatKes(data.credited_cents)}</dd>
            <dt className="text-ink-muted">Waiting for Finance</dt>
            <dd className="tabular-nums">{formatKes(data.unmatched_cents)}</dd>
          </dl>
          <p className="text-sm">
            &ldquo;Received from Safaricom&rdquo; should equal the paybill&apos;s receipts on the M-Pesa
            statement for this day. If it does not, a confirmation may not have reached us: ask the ICT office
            to check.
          </p>
          {data.manual_count > 0 ? (
            <Alert tone="warning" title="Recorded by hand, not reported by Safaricom">
              {data.manual_count} M-Pesa payment{data.manual_count === 1 ? "" : "s"} for this day (
              {formatKes(data.manual_cents)}) {data.manual_count === 1 ? "was" : "were"} recorded by hand
              {data.manual_pending ? `, ${data.manual_pending} still waiting for confirmation` : ""}. Find
              each on the statement before confirming it.
            </Alert>
          ) : null}
        </Card>
      )}
    </section>
  );
}

function Setup({ sandbox, onDone }: { sandbox: boolean; onDone: () => void }) {
  const { show } = useToast();
  const [amount, setAmount] = useState("");
  const [phone, setPhone] = useState("");
  const [account, setAccount] = useState("");
  const { busy, errors, setErrors, run } = useAction();
  return (
    <Card className="flex flex-col gap-4">
      <h2 className="text-xl font-semibold">Setup</h2>
      <p>
        Register the callback URLs with Safaricom once for the paybill, and again if the API&apos;s address
        changes.
      </p>
      <div className="flex flex-wrap gap-3">
        <Button
          variant="outline"
          loading={busy}
          onClick={async () => {
            const r = await run(() => api.POST("/api/v1/staff/finance/mpesa/register-urls"), "");
            if (r) show({ tone: "success", title: `Safaricom answered: ${r.message}` });
          }}
        >
          Register callback URLs
        </Button>
        <Button
          variant="outline"
          onClick={async () => {
            const r = await run(() => api.POST("/api/v1/staff/finance/mpesa/reprocess"), "");
            if (r) {
              show({ tone: "success", title: `${r.processed} stored confirmation(s) recorded` });
              onDone();
            }
          }}
        >
          Record stored confirmations again
        </Button>
      </div>
      {sandbox ? (
        <form
          noValidate
          className="flex flex-col gap-3 border-t border-stone pt-4"
          onSubmit={async (e) => {
            e.preventDefault();
            const cents = parseAmount(amount);
            if (cents === null) {
              setErrors({ amount_cents: "Enter whole shillings." });
              return;
            }
            const r = await run(
              () =>
                api.POST("/api/v1/staff/finance/mpesa/simulate", {
                  body: { amount_cents: cents, msisdn: phone, account },
                }),
              "",
            );
            if (r)
              show({ tone: "success", title: `Sandbox: ${r.message}. The payment arrives in a moment.` });
          }}
        >
          <h3 className="text-lg font-semibold">Send a sandbox test payment</h3>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <Input
              id="sim-amount"
              name="amount"
              label="Amount (KES)"
              required
              inputMode="numeric"
              value={amount}
              error={errors.amount_cents}
              onChange={(e) => setAmount(e.target.value)}
            />
            <Input
              id="sim-phone"
              name="msisdn"
              label="Sandbox phone number"
              required
              inputMode="numeric"
              hint="The test number from the Daraja portal, starting 254."
              value={phone}
              error={errors.msisdn}
              onChange={(e) => setPhone(e.target.value)}
            />
            <Input
              id="sim-account"
              name="account"
              label="Account number"
              required
              hint="A student number."
              value={account}
              error={errors.account}
              onChange={(e) => setAccount(e.target.value)}
            />
          </div>
          <Button type="submit" variant="outline" className="self-start" loading={busy}>
            Send test payment
          </Button>
        </form>
      ) : null}
    </Card>
  );
}

const filters: { value: "" | TxStatus; label: string }[] = [
  { value: "", label: "Every payment" },
  { value: "matched", label: "Credited" },
  { value: "assigned", label: "Assigned by Finance" },
  { value: "refund", label: "To be returned" },
  { value: "rejected", label: "Not for this paybill" },
];

export function MpesaPage() {
  const { can } = useSession();
  const status = useLoaded(() => api.GET("/api/v1/staff/finance/mpesa"), []);
  const waiting = useLoaded(
    () => api.GET("/api/v1/staff/finance/mpesa/transactions", { params: { query: { status: "unmatched" } } }),
    [],
  );
  const [filter, setFilter] = useState<"" | TxStatus>("");
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");
  const all = useLoaded(
    () =>
      api.GET("/api/v1/staff/finance/mpesa/transactions", {
        params: { query: { status: filter || undefined, q: search || undefined } },
      }),
    [filter, search],
  );
  const reload = () => {
    status.reload();
    waiting.reload();
    all.reload();
  };

  if (status.error && !status.data) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-3xl font-semibold">M-Pesa</h1>
        <EmptyState
          title="M-Pesa is not switched on"
          description="Payments to the paybill are recorded by hand until the ICT office sets up M-Pesa (MPESA_ENABLED)."
        />
      </div>
    );
  }
  const s = status.data;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">M-Pesa</h1>
        <p className="max-w-prose text-ink-muted">
          Payments to the college paybill arrive here from Safaricom. When the account number is a
          student&apos;s number, the payment is credited and receipted by itself; anything else waits for you
          below.
        </p>
      </div>

      {!s ? (
        <LoadState error={null} label="Loading" />
      ) : (
        <Card>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 min-[420px]:grid-cols-[auto_1fr]">
            <dt className="text-ink-muted">Paybill</dt>
            <dd className="font-mono font-semibold">{s.short_code}</dd>
            <dt className="text-ink-muted">Environment</dt>
            <dd>{s.environment === "sandbox" ? "Sandbox (test money)" : "Production"}</dd>
            <dt className="text-ink-muted">Last call from Safaricom</dt>
            <dd>{s.last_callback_at ? formatNairobi(s.last_callback_at) : "None yet"}</dd>
            <dt className="text-ink-muted">Confirmation URL</dt>
            <dd className="font-mono text-sm break-all">{s.confirmation_url}</dd>
          </dl>
        </Card>
      )}

      <section aria-labelledby="waiting-title" className="flex flex-col gap-3">
        <h2 id="waiting-title" className="text-xl font-semibold">
          Waiting for you
        </h2>
        {!waiting.data ? (
          <LoadState error={waiting.error} label="Loading" />
        ) : waiting.data.items.length === 0 ? (
          <p>Nothing is waiting: every payment matched a student.</p>
        ) : (
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {waiting.data.items.map((t) => (
              <TransactionItem key={t.id} t={t}>
                {can("finance.payment.confirm") ? <Settle t={t} provider="mpesa" onDone={reload} /> : null}
              </TransactionItem>
            ))}
          </ul>
        )}
      </section>

      <Reconciliation />

      <section aria-labelledby="all-title" className="flex flex-col gap-3">
        <h2 id="all-title" className="text-xl font-semibold">
          Payments received
        </h2>
        <form
          role="search"
          className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[1fr_2fr_auto]"
          onSubmit={(e) => {
            e.preventDefault();
            setSearch(q.trim());
          }}
        >
          <Select
            id="mpesa-filter"
            name="status"
            label="Show"
            required
            value={filter}
            onChange={(e) => setFilter(e.target.value as typeof filter)}
          >
            {filters.map((f) => (
              <option key={f.value} value={f.value}>
                {f.label}
              </option>
            ))}
          </Select>
          <Input
            id="mpesa-q"
            name="q"
            type="search"
            label="Search"
            hint="M-Pesa code, account number or payer."
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <Button type="submit" variant="outline">
            Search
          </Button>
        </form>
        {!all.data ? (
          <LoadState error={all.error} label="Loading" />
        ) : all.data.items.length === 0 ? (
          <p>No payments match.</p>
        ) : (
          <>
            <p role="status">{all.data.total === 1 ? "1 payment" : `${all.data.total} payments`}</p>
            <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
              {all.data.items.map((t) => (
                <TransactionItem key={t.id} t={t} />
              ))}
            </ul>
          </>
        )}
      </section>

      {can("finance.mpesa.setup") && s ? (
        <Setup sandbox={s.environment === "sandbox"} onDone={reload} />
      ) : null}
    </div>
  );
}
