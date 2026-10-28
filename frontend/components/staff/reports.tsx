"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button, buttonClasses } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { Table, type Column } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { cn } from "@/lib/cn";
import { formatKes } from "@/lib/money";
import { formatDate } from "@/lib/staff/datetime";
import { attachmentName, monthLabel, officeLabels, percent, type ExportKind } from "@/lib/staff/reports";
import { useSession } from "./session";

type Dashboard = components["schemas"]["Dashboard"];
type Section<K extends keyof Dashboard> = NonNullable<Dashboard[K]>;
type Period = { from: string; to: string };

/**
 * Figures for a period (reports.view). The API returns only the sections the
 * viewer may see (D-185), so the page shows whatever comes back; an export
 * button appears on a section when the viewer also holds reports.export.
 */
export function Reports() {
  const { can } = useSession();
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [error, setError] = useState<{ message: string; fields: Record<string, string> } | null>(null);

  useEffect(() => {
    let active = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- show the loading state for each period
    setDashboard(null);
    api
      .GET("/api/v1/staff/reports/dashboard", {
        params: { query: { from: from || undefined, to: to || undefined } },
      })
      .then(({ data, error: err }) => {
        if (!active) return;
        if (data) {
          setDashboard(data);
          setError(null);
          return;
        }
        const fields: Record<string, string> = {};
        if (isApiError(err)) for (const f of err.error.fields ?? []) fields[f.field] ??= f.message;
        setError({ message: errorMessage(err), fields });
      });
    return () => {
      active = false;
    };
  }, [from, to]);

  function show(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    const next = new URLSearchParams();
    for (const k of ["from", "to"]) {
      const v = String(data.get(k) ?? "");
      if (v) next.set(k, v);
    }
    const s = next.toString();
    router.push(s ? `${pathname}?${s}` : pathname);
  }

  const canExport = can("reports.export");
  const d = dashboard;
  const period = d ? { from: d.from, to: d.to } : null;
  const nothing = d && !d.admissions && !d.students && !d.results && !d.finance && !d.enquiries;

  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Reports</h1>
        <p className="max-w-prose text-ink-muted">
          Figures for the period you choose, from the records themselves. Figures marked &ldquo;now&rdquo; are
          as they stand today, whatever the period. You see the sections your role may see.
        </p>
      </div>
      <form
        onSubmit={show}
        className="grid items-end gap-4 sm:grid-cols-[1fr_1fr_auto]"
        key={`${from}|${to}`}
      >
        <Input
          id="reports-from"
          name="from"
          type="date"
          label="From"
          hint="Default: 1 January this year."
          defaultValue={from || d?.from}
          error={error?.fields.from}
        />
        <Input
          id="reports-to"
          name="to"
          type="date"
          label="To"
          hint="At most two years after From."
          defaultValue={to || d?.to}
          error={error?.fields.to}
        />
        <button type="submit" className={buttonClasses("primary")}>
          Show
        </button>
      </form>
      {error ? (
        <Alert tone="danger" title="Could not make the reports">
          {error.message}
        </Alert>
      ) : null}
      {!d && !error ? (
        <Loading label="Loading the reports">
          <div className="flex flex-col gap-4">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-48 w-full" />
          </div>
        </Loading>
      ) : null}
      {d && period ? (
        <>
          <p className="font-semibold text-navy-900">
            {formatDate(d.from)} to {formatDate(d.to)}
          </p>
          {nothing ? (
            <EmptyState
              title="No reports for your role"
              description="Your account may open reports but cannot see admissions, students, results, finance or enquiries. Ask the ICT office if you need one of these."
            />
          ) : null}
          {d.admissions ? (
            <AdmissionsSection data={d.admissions} period={period} canExport={canExport} />
          ) : null}
          {d.students ? <StudentsSection data={d.students} period={period} canExport={canExport} /> : null}
          {d.results ? <ResultsSection data={d.results} period={period} canExport={canExport} /> : null}
          {d.finance ? <FinanceSection data={d.finance} period={period} canExport={canExport} /> : null}
          {d.enquiries ? <EnquiriesSection data={d.enquiries} /> : null}
        </>
      ) : null}
    </div>
  );
}

function ReportSection({
  id,
  title,
  description,
  action,
  children,
}: {
  id: string;
  title: string;
  description: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section aria-labelledby={`${id}-heading`} className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h2 id={`${id}-heading`} className="text-2xl font-semibold">
            {title}
          </h2>
          <p className="max-w-prose text-ink-muted">{description}</p>
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

/** Headline figures; money tiles are wider so amounts fit at 360 px. */
function Tiles({ items, money }: { items: { label: string; value: string }[]; money?: boolean }) {
  return (
    <dl className={cn("grid gap-3", money ? "sm:grid-cols-2 lg:grid-cols-3" : "grid-cols-2 lg:grid-cols-4")}>
      {items.map((t) => (
        <div key={t.label} className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4">
          <dt className="text-sm text-ink-muted">{t.label}</dt>
          <dd className="tabular text-2xl font-semibold break-words text-navy-900">{t.value}</dd>
        </div>
      ))}
    </dl>
  );
}

/** Downloads a count-only CSV for the period on screen (reports.export). */
function ExportButton({ kind, label, period }: { kind: ExportKind; label: string; period: Period }) {
  const { show } = useToast();
  const [busy, setBusy] = useState(false);

  async function download() {
    setBusy(true);
    const { data, error, response } = await api.GET("/api/v1/staff/reports/exports/{kind}", {
      params: { path: { kind }, query: { from: period.from, to: period.to } },
      parseAs: "blob",
    });
    setBusy(false);
    if (!data) {
      show({ tone: "danger", title: "Not downloaded", description: errorMessage(error) });
      return;
    }
    const name = attachmentName(
      response.headers.get("Content-Disposition"),
      `dakota-${kind}-${period.from}-to-${period.to}.csv`,
    );
    const url = URL.createObjectURL(data as Blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    document.body.append(a);
    a.click();
    a.remove();
    // Some browsers read the URL after click returns.
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    show({ tone: "success", title: "Downloaded", description: name });
  }

  return (
    <Button variant="outline" loading={busy} onClick={() => void download()}>
      {label}
    </Button>
  );
}

const admissionsColumns: { key: keyof Section<"admissions">["totals"]; header: string }[] = [
  { key: "submitted", header: "Submitted" },
  { key: "in_review", header: "In review" },
  { key: "offered", header: "Offered" },
  { key: "accepted", header: "Accepted" },
  { key: "enrolled", header: "Enrolled" },
  { key: "rejected", header: "Rejected" },
  { key: "withdrawn", header: "Withdrawn" },
  { key: "expired", header: "Expired" },
];

function AdmissionsSection({
  data,
  period,
  canExport,
}: {
  data: Section<"admissions">;
  period: Period;
  canExport: boolean;
}) {
  type Row = Section<"admissions">["by_school"][number] | (Section<"admissions">["totals"] & { total: true });
  const t = data.totals;
  const columns: Column<Row>[] = [
    {
      key: "school",
      header: "School",
      cell: (r) => ("total" in r ? <strong>All schools</strong> : r.school_name),
    },
    ...admissionsColumns.map((c) => ({
      key: c.key,
      header: c.header,
      numeric: true,
      cell: (r: Row) => ("total" in r ? <strong>{r[c.key]}</strong> : r[c.key]),
    })),
  ];
  return (
    <ReportSection
      id="admissions"
      title="Admissions"
      description="Applications submitted in the period, and where each one is now."
      action={
        canExport ? (
          <ExportButton kind="admissions" label="Download admissions (CSV)" period={period} />
        ) : null
      }
    >
      <Tiles
        items={[
          { label: "Submitted", value: String(t.submitted) },
          { label: "Offered", value: String(t.offered) },
          { label: "Accepted", value: String(t.accepted) },
          { label: "Enrolled", value: String(t.enrolled) },
        ]}
      />
      <Table
        caption="Applications by school"
        columns={columns}
        rows={data.by_school.length ? [...data.by_school, { ...t, total: true }] : []}
        rowKey={(r) => ("total" in r ? "total" : r.school_id)}
        empty="No applications were submitted in this period."
      />
    </ReportSection>
  );
}

const statusColumns: { key: keyof Section<"students">["totals"]; header: string }[] = [
  { key: "active", header: "Active" },
  { key: "deferred", header: "Deferred" },
  { key: "suspended", header: "Suspended" },
  { key: "withdrawn", header: "Withdrawn" },
  { key: "graduated", header: "Graduated" },
];

function StudentsSection({
  data,
  period,
  canExport,
}: {
  data: Section<"students">;
  period: Period;
  canExport: boolean;
}) {
  type Row =
    | Section<"students">["by_school"][number]
    | (Section<"students">["totals"] & { joined: number; total: true });
  const columns: Column<Row>[] = [
    {
      key: "school",
      header: "School",
      cell: (r) => ("total" in r ? <strong>All schools</strong> : r.school_name),
    },
    ...statusColumns.map((c) => ({
      key: c.key,
      header: `${c.header} now`,
      numeric: true,
      cell: (r: Row) => ("total" in r ? <strong>{r[c.key]}</strong> : r[c.key]),
    })),
    {
      key: "joined",
      header: "Joined in period",
      numeric: true,
      cell: (r) => ("total" in r ? <strong>{r.joined}</strong> : r.joined),
    },
  ];
  return (
    <ReportSection
      id="students"
      title="Students"
      description="Every student by status now, and who enrolled in the period."
      action={
        canExport ? (
          <ExportButton kind="enrolments" label="Download enrolments (CSV)" period={period} />
        ) : null
      }
    >
      <Tiles
        items={[
          { label: "Active now", value: String(data.totals.active) },
          { label: "Joined in period", value: String(data.joined) },
          { label: "Open student requests now", value: String(data.open_requests) },
          { label: "Detail changes to check now", value: String(data.pending_changes) },
        ]}
      />
      <Table
        caption="Students by school"
        columns={columns}
        rows={
          data.by_school.length
            ? [...data.by_school, { ...data.totals, joined: data.joined, total: true }]
            : []
        }
        rowKey={(r) => ("total" in r ? "total" : r.school_id)}
        empty="No students yet."
      />
    </ReportSection>
  );
}

function ResultsSection({
  data,
  period,
  canExport,
}: {
  data: Section<"results">;
  period: Period;
  canExport: boolean;
}) {
  return (
    <ReportSection
      id="results"
      title="Results"
      description="Classes whose results were released in the period, and where every class's results are now."
      action={
        canExport ? <ExportButton kind="results" label="Download pass rates (CSV)" period={period} /> : null
      }
    >
      <Tiles
        items={[
          { label: "Classes released in period", value: String(data.released_classes) },
          { label: "Results released", value: String(data.results) },
          { label: "Passed or competent", value: String(data.passed) },
          { label: "Pass rate", value: percent(data.passed, data.results) || "None released" },
        ]}
      />
      <Tiles
        items={[
          { label: "Classes with marks being entered now", value: String(data.draft) },
          { label: "Waiting for approval now", value: String(data.submitted) },
          { label: "Approved, not released now", value: String(data.approved) },
          { label: "Released now", value: String(data.released) },
        ]}
      />
    </ReportSection>
  );
}

function FinanceSection({
  data,
  period,
  canExport,
}: {
  data: Section<"finance">;
  period: Period;
  canExport: boolean;
}) {
  type Month = Section<"finance">["by_month"][number];
  type Row = Month | (Omit<Month, "month"> & { total: true });
  const money = (
    key: "billed_cents" | "collected_cents" | "charges_cents" | "credits_cents",
    header: string,
  ) => ({
    key,
    header,
    numeric: true,
    cell: (r: Row) => ("total" in r ? <strong>{formatKes(r[key])}</strong> : formatKes(r[key])),
  });
  const columns: Column<Row>[] = [
    {
      key: "month",
      header: "Month",
      cell: (r) => ("total" in r ? <strong>Whole period</strong> : monthLabel(r.month)),
    },
    money("billed_cents", "Billed"),
    money("collected_cents", "Collected"),
    money("charges_cents", "Charges"),
    money("credits_cents", "Credits"),
  ];
  const total: Row = {
    billed_cents: data.billed_cents,
    collected_cents: data.collected_cents,
    charges_cents: data.charges_cents,
    credits_cents: data.credits_cents,
    total: true,
  };
  return (
    <ReportSection
      id="finance"
      title="Finance"
      description="Fees billed, payments confirmed and adjustments made in the period, and what is owed now."
      action={
        canExport ? (
          <ExportButton kind="fee-collection" label="Download fee collection (CSV)" period={period} />
        ) : null
      }
    >
      <Tiles
        money
        items={[
          { label: "Billed in period", value: formatKes(data.billed_cents) },
          { label: "Collected in period", value: formatKes(data.collected_cents) },
          // Net of credit, so it can be below zero: say so in words.
          data.outstanding_cents < 0
            ? { label: "Students in credit now, net", value: formatKes(-data.outstanding_cents) }
            : { label: "Owed now", value: formatKes(data.outstanding_cents) },
          { label: "Charges in period", value: formatKes(data.charges_cents) },
          { label: "Credits in period", value: formatKes(data.credits_cents) },
          {
            label: `${data.pending_payments === 1 ? "1 payment" : `${data.pending_payments} payments`} to confirm now`,
            value: formatKes(data.pending_payment_cents),
          },
        ]}
      />
      <Table
        caption="Money by month"
        columns={columns}
        rows={data.by_month.length ? [...data.by_month, total] : []}
        rowKey={(r) => ("total" in r ? "total" : r.month)}
        empty="Nothing was billed, collected or adjusted in this period."
      />
    </ReportSection>
  );
}

function EnquiriesSection({ data }: { data: Section<"enquiries"> }) {
  type Row = Section<"enquiries">["by_office"][number];
  const count = (key: "received" | "closed" | "new_now" | "open_now", header: string): Column<Row> => ({
    key,
    header,
    numeric: true,
    cell: (r) => r[key],
  });
  return (
    <ReportSection
      id="enquiries"
      title="Enquiries"
      description="Enquiries received and closed in the period, and those waiting now."
    >
      <Tiles
        items={[
          { label: "Received in period", value: String(data.received) },
          { label: "Closed in period", value: String(data.closed) },
          { label: "New now", value: String(data.new_now) },
          { label: "Open now", value: String(data.open_now) },
        ]}
      />
      <Table
        caption="Enquiries by office"
        columns={[
          { key: "office", header: "Office", cell: (r) => officeLabels[r.office] ?? r.office },
          count("received", "Received"),
          count("closed", "Closed"),
          count("new_now", "New now"),
          count("open_now", "Open now"),
        ]}
        rows={data.by_office}
        rowKey={(r) => r.office}
        empty="No enquiries."
      />
    </ReportSection>
  );
}
