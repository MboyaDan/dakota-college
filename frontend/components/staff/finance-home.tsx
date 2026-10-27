"use client";

import Link from "next/link";
import { useState } from "react";
import { Button, LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox, Input } from "@/components/ui/field";
import { Table } from "@/components/ui/table";
import { api } from "@/lib/api/client";
import { formatKes } from "@/lib/money";
import { LoadState, useLoaded } from "./finance-common";
import { useSession } from "./session";

// The finance office's start page (Stage 14b): what waits, a student
// search to open an account, and who owes money.

function FindStudent() {
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
    <div className="flex flex-col gap-3">
      <form
        role="search"
        className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[1fr_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          setSearch(q.trim());
        }}
      >
        <Input
          id="finance-find"
          name="q"
          type="search"
          label="Find a student"
          required
          hint="Student number, name or email."
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <Button type="submit" variant="outline">
          Find
        </Button>
      </form>
      {search && data ? (
        data.items.length === 0 ? (
          <p role="status">No student matches.</p>
        ) : (
          <>
            <p role="status">{data.total === 1 ? "1 student" : `${data.total} students`}</p>
            <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
              {data.items.slice(0, 10).map((s) => (
                <li key={s.id}>
                  <Link
                    href={`/staff/finance/students/${s.id}`}
                    className="flex min-h-touch flex-col justify-center px-4 py-2 hover:bg-ivory"
                  >
                    <span className="font-semibold text-royal">
                      <span className="font-mono">{s.student_number}</span> {s.name}
                    </span>
                    <span className="text-sm text-ink-muted">{s.programme_title}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </>
        )
      ) : null}
    </div>
  );
}

export function FinanceHome() {
  const { can } = useSession();
  const [overdueOnly, setOverdueOnly] = useState(true);
  const summary = useLoaded(() => api.GET("/api/v1/staff/finance/summary"), []);
  const arrears = useLoaded(
    () =>
      api.GET("/api/v1/staff/finance/arrears", { params: { query: overdueOnly ? { overdue: "1" } : {} } }),
    [overdueOnly],
  );
  const s = summary.data;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Finance</h1>
        <p className="max-w-prose text-ink-muted">
          Balances are worked out from invoices, confirmed payments and adjustments each time; nothing is
          typed in.
        </p>
      </div>

      {!s ? (
        <LoadState error={summary.error} label="Loading the summary" />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Card className="flex flex-col gap-2">
            <h2 className="text-lg font-semibold">Payments to confirm</h2>
            <p className="text-3xl font-semibold tabular-nums">{s.pending_payments}</p>
            <p className="text-sm text-ink-muted">{formatKes(s.pending_cents)} waiting</p>
            <LinkButton href="/staff/finance/payments" variant="outline" className="mt-auto self-start">
              {can("finance.payment.confirm") ? "Check payments" : "See payments"}
            </LinkButton>
          </Card>
          <Card className="flex flex-col gap-2">
            <h2 className="text-lg font-semibold">Draft invoices</h2>
            <p className="text-3xl font-semibold tabular-nums">{s.draft_invoices}</p>
            <p className="text-sm text-ink-muted">Not seen by students until issued</p>
            <LinkButton href="/staff/finance/invoices" variant="outline" className="mt-auto self-start">
              See invoices
            </LinkButton>
          </Card>
          <Card className="flex flex-col gap-2">
            <h2 className="text-lg font-semibold">Owed to the college</h2>
            <p className="text-3xl font-semibold tabular-nums">{formatKes(s.net_outstanding_cents)}</p>
            <p className="text-sm text-ink-muted">All balances added up, less credit</p>
          </Card>
        </div>
      )}

      <Card className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold">Open a student&apos;s account</h2>
        <FindStudent />
      </Card>

      <section aria-labelledby="arrears-title" className="flex flex-col gap-3">
        <h2 id="arrears-title" className="text-xl font-semibold">
          Who owes
        </h2>
        <Checkbox
          id="arrears-overdue"
          name="overdue"
          label="Only students with an amount overdue"
          checked={overdueOnly}
          onChange={(e) => setOverdueOnly(e.target.checked)}
        />
        {!arrears.data ? (
          <LoadState error={arrears.error} label="Loading" />
        ) : (
          <Table
            caption={overdueOnly ? "Students with an amount overdue" : "Students who owe"}
            captionHidden
            rows={arrears.data.items}
            rowKey={(r) => r.student.id}
            empty={overdueOnly ? "Nobody has an amount overdue." : "Nobody owes anything."}
            columns={[
              {
                key: "student",
                header: "Student",
                cell: (r) => (
                  <>
                    <Link href={`/staff/finance/students/${r.student.id}`} className="text-royal underline">
                      <span className="font-mono">{r.student.student_number}</span> {r.student.name}
                    </Link>
                    <span className="block text-sm text-ink-muted">
                      {r.cohort_name ?? r.student.programme_title}
                    </span>
                  </>
                ),
              },
              { key: "overdue", header: "Overdue", numeric: true, cell: (r) => formatKes(r.overdue_cents) },
              { key: "balance", header: "Balance", numeric: true, cell: (r) => formatKes(r.balance_cents) },
            ]}
          />
        )}
      </section>
    </div>
  );
}
