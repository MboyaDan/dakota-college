"use client";

import Link from "next/link";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select } from "@/components/ui/field";
import type { Status } from "@/components/ui/tag";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatKes } from "@/lib/money";
import { formatNairobi } from "@/lib/staff/datetime";
import { LoadState, useLoaded } from "./finance-common";
import { Settle, TransactionItem } from "./finance-mpesa";
import { todayNairobi } from "./finance-payments";
import { useSession } from "./session";

// KCB bank payments (Stage 15b, D-174 to D-176). KCB tells us when the
// college account is credited. Until KCB's signature can be checked, a
// notification is not proof of payment: a matched one waits in Payments
// for Finance to confirm against the KCB statement.

type Transaction = components["schemas"]["KcbTransaction"];
type TxStatus = Transaction["status"];

function label(t: Transaction): { label: string; tone: Status } | undefined {
  if (t.status === "matched" && t.payment_status === "pending")
    return { label: "Waiting for confirmation", tone: "warning" };
  if (t.status === "matched" && t.payment_status === "confirmed")
    return { label: "Confirmed", tone: "success" };
  if (t.status === "matched" && t.payment_status === "rejected")
    return { label: "Rejected in Payments", tone: "danger" };
  if (t.status === "rejected") return { label: "Not for the college account", tone: "danger" };
  return undefined;
}

const filters: { value: "" | TxStatus; label: string }[] = [
  { value: "", label: "Every notification" },
  { value: "matched", label: "Matched to a student" },
  { value: "assigned", label: "Assigned by Finance" },
  { value: "refund", label: "To be returned" },
  { value: "rejected", label: "Not for the college account" },
];

function DayCheck() {
  const [day, setDay] = useState(todayNairobi());
  const { data, error } = useLoaded(
    () => api.GET("/api/v1/staff/finance/kcb/reconciliation", { params: { query: { day } } }),
    [day],
  );
  return (
    <section aria-labelledby="kcb-day-title" className="flex flex-col gap-3">
      <h2 id="kcb-day-title" className="text-xl font-semibold">
        Check a day against the KCB statement
      </h2>
      <div className="sm:max-w-xs">
        <Input
          id="kcb-day"
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
        <Card>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 min-[420px]:grid-cols-[auto_1fr]">
            <dt className="text-ink-muted">Notified by KCB</dt>
            <dd className="font-semibold tabular-nums">
              {formatKes(data.received_cents)} ({data.received_count} payment
              {data.received_count === 1 ? "" : "s"})
            </dd>
            <dt className="text-ink-muted">Matched or assigned</dt>
            <dd className="tabular-nums">{formatKes(data.credited_cents)}</dd>
            <dt className="text-ink-muted">Waiting for Finance</dt>
            <dd className="tabular-nums">{formatKes(data.unmatched_cents)}</dd>
          </dl>
        </Card>
      )}
    </section>
  );
}

export function KcbPage() {
  const { can } = useSession();
  const status = useLoaded(() => api.GET("/api/v1/staff/finance/kcb"), []);
  const waiting = useLoaded(
    () => api.GET("/api/v1/staff/finance/kcb/transactions", { params: { query: { status: "unmatched" } } }),
    [],
  );
  const [filter, setFilter] = useState<"" | TxStatus>("");
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");
  const all = useLoaded(
    () =>
      api.GET("/api/v1/staff/finance/kcb/transactions", {
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
        <h1 className="text-3xl font-semibold">KCB bank</h1>
        <EmptyState
          title="KCB notifications are not switched on"
          description="Bank payments are recorded by hand until the ICT office sets up KCB notifications (KCB_IPN_ENABLED)."
        />
      </div>
    );
  }
  const s = status.data;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">KCB bank</h1>
        <p className="max-w-prose text-ink-muted">
          KCB tells us when the college account is credited. When the payer&apos;s reference is a student
          number, a payment waits in{" "}
          <Link href="/staff/finance/payments" className="text-royal underline">
            Payments
          </Link>{" "}
          for you to confirm against the KCB statement; anything else waits below.
        </p>
      </div>

      {s && !s.signature_verified ? (
        <Alert tone="info" title="Every KCB payment is confirmed by Finance">
          KCB has not yet given us a way to check that a notification really comes from them, so none is
          counted until you find it on the statement.
        </Alert>
      ) : null}

      {!s ? (
        <LoadState error={null} label="Loading" />
      ) : (
        <Card>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 min-[420px]:grid-cols-[auto_1fr]">
            <dt className="text-ink-muted">College account</dt>
            <dd className="font-mono font-semibold">
              {s.credit_account || "Not set (any account accepted)"}
            </dd>
            <dt className="text-ink-muted">Last notification</dt>
            <dd>{s.last_notification_at ? formatNairobi(s.last_notification_at) : "None yet"}</dd>
            <dt className="text-ink-muted">Notification path</dt>
            <dd className="font-mono text-sm break-all">{s.notification_url}</dd>
          </dl>
        </Card>
      )}

      <section aria-labelledby="kcb-waiting-title" className="flex flex-col gap-3">
        <h2 id="kcb-waiting-title" className="text-xl font-semibold">
          Waiting for you
        </h2>
        {!waiting.data ? (
          <LoadState error={waiting.error} label="Loading" />
        ) : waiting.data.items.length === 0 ? (
          <p>Nothing is waiting: every notification matched a student.</p>
        ) : (
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {waiting.data.items.map((t) => (
              <TransactionItem key={t.id} t={t} referenceLabel="reference">
                {can("finance.payment.confirm") ? <Settle t={t} provider="kcb" onDone={reload} /> : null}
              </TransactionItem>
            ))}
          </ul>
        )}
      </section>

      <DayCheck />

      <section aria-labelledby="kcb-all-title" className="flex flex-col gap-3">
        <h2 id="kcb-all-title" className="text-xl font-semibold">
          Notifications received
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
            id="kcb-filter"
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
            id="kcb-q"
            name="q"
            type="search"
            label="Search"
            hint="KCB reference, payer's reference or name."
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
          <p>No notifications match.</p>
        ) : (
          <>
            <p role="status">{all.data.total === 1 ? "1 notification" : `${all.data.total} notifications`}</p>
            <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
              {all.data.items.map((t) => (
                <TransactionItem key={t.id} t={t} statusLabel={label(t)} referenceLabel="reference" />
              ))}
            </ul>
          </>
        )}
      </section>
    </div>
  );
}
