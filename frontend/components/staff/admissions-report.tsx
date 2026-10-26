"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { buttonClasses } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { Table, type Column } from "@/components/ui/table";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDate } from "@/lib/staff/datetime";

type Report = components["schemas"]["AdmissionsReport"];
type Row = Report["rows"][number] | (Report["totals"] & { total: true });
type Figure = keyof Report["totals"];

const figures: { key: Figure; header: string }[] = [
  { key: "submitted", header: "Submitted" },
  { key: "in_review", header: "In review" },
  { key: "offered", header: "Offered" },
  { key: "accepted", header: "Accepted" },
  { key: "enrolled", header: "Enrolled" },
  { key: "rejected", header: "Rejected" },
  { key: "withdrawn", header: "Withdrawn" },
  { key: "expired", header: "Expired" },
];

/** Applications submitted in a period, by programme and intake (admissions.view). */
export function AdmissionsReport() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const [report, setReport] = useState<Report | null>(null);
  const [error, setError] = useState<{ message: string; fields: Record<string, string> } | null>(null);

  useEffect(() => {
    let active = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- show the loading state for each period
    setReport(null);
    api
      .GET("/api/v1/staff/applications/report", {
        params: { query: { from: from || undefined, to: to || undefined } },
      })
      .then(({ data, error: err }) => {
        if (!active) return;
        if (data) {
          setReport(data);
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

  const columns: Column<Row>[] = [
    {
      key: "programme",
      header: "Programme and intake",
      cell: (r) =>
        "total" in r ? (
          <strong>All programmes</strong>
        ) : (
          <>
            <span className="block">{r.programme_title}</span>
            <span className="text-sm text-ink-muted">{r.intake_name ?? "No intake"}</span>
          </>
        ),
    },
    ...figures.map((f) => ({
      key: f.key,
      header: f.header,
      numeric: true,
      cell: (r: Row) => ("total" in r ? <strong>{r[f.key]}</strong> : r[f.key]),
    })),
  ];

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "Applications", href: "/staff/applications" }, { label: "Report" }]} />
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Admissions report</h1>
        <p className="max-w-prose text-ink-muted">
          Applications submitted in the period, and where each one is now. &ldquo;Offered&rdquo; counts every
          application that received an offer, whatever happened next.
        </p>
      </div>
      <form
        onSubmit={show}
        className="grid items-end gap-4 sm:grid-cols-[1fr_1fr_auto]"
        key={`${from}|${to}`}
      >
        <Input
          id="report-from"
          name="from"
          type="date"
          label="Submitted from"
          defaultValue={from || report?.from}
          error={error?.fields.from}
        />
        <Input
          id="report-to"
          name="to"
          type="date"
          label="Submitted to"
          defaultValue={to || report?.to}
          error={error?.fields.to}
        />
        <button type="submit" className={buttonClasses("primary")}>
          Show
        </button>
      </form>
      {error ? (
        <Alert tone="danger" title="Could not make the report">
          {error.message}
        </Alert>
      ) : null}
      {!report && !error ? (
        <Loading label="Loading the report">
          <Skeleton className="h-48 w-full" />
        </Loading>
      ) : null}
      {report ? (
        <Table
          caption={`Applications submitted ${formatDate(report.from)} to ${formatDate(report.to)}`}
          columns={columns}
          rows={report.rows.length ? [...report.rows, { ...report.totals, total: true }] : []}
          rowKey={(r) => ("total" in r ? "total" : `${r.programme_id}/${r.intake_id ?? ""}`)}
          empty="No applications were submitted in this period."
        />
      ) : null}
    </div>
  );
}
