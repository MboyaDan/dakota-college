"use client";

import Link from "next/link";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select, Textarea } from "@/components/ui/field";
import { api } from "@/lib/api/client";
import { categoryLabel, describePlan, invoiceFor, type Category } from "@/lib/finance";
import { formatKes } from "@/lib/money";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { InvoiceStatusTag, LoadState, PdfLink, useLoaded, type Invoice } from "./finance-common";
import {
  LinesEditor,
  PlanEditor,
  linesToSend,
  linesTotal,
  planProblems,
  planToSend,
  toLineDrafts,
  toPlanDrafts,
  type LineDraft,
  type PlanDraft,
} from "./finance-lines";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Invoices (Stage 14b, D-160, D-161): bill a class for a term, check and
// issue drafts (one or many), and correct a draft before it is issued. An
// issued invoice never changes; it is corrected from the student's account
// with an adjustment.

type Status = "draft" | "issued" | "void";

function BillClass({ onBilled }: { onBilled: (term: string, cohort: string) => void }) {
  const [cohort, setCohort] = useState("");
  const [term, setTerm] = useState("");
  const [result, setResult] = useState<string | null>(null);
  const cohorts = useLoaded(() => api.GET("/api/v1/staff/cohorts"), []);
  const terms = useLoaded(() => api.GET("/api/v1/staff/terms"), []);
  const { busy, errors, run } = useAction();
  return (
    <form
      noValidate
      className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[2fr_1fr_auto]"
      onSubmit={async (e) => {
        e.preventDefault();
        setResult(null);
        const r = await run(
          () => api.POST("/api/v1/staff/finance/billing", { body: { cohort_id: cohort, term_id: term } }),
          "",
          "bill.",
        );
        if (!r) return;
        setResult(
          r.created === 0
            ? `Nobody new to bill: ${r.skipped} already had an invoice for this term.`
            : `${r.created} draft invoice${r.created === 1 ? "" : "s"} made${r.skipped ? `; ${r.skipped} already billed were skipped` : ""}. Check them below, then issue them.`,
        );
        onBilled(term, cohort);
      }}
    >
      <Select
        id="bill-cohort"
        name="cohort_id"
        label="Class (cohort)"
        required
        value={cohort}
        error={errors["bill.cohort_id"]}
        onChange={(e) => setCohort(e.target.value)}
      >
        <option value="">Choose a class</option>
        {(cohorts.data?.items ?? []).map((c) => (
          <option key={c.id} value={c.id}>
            {c.name} ({c.active_students} active)
          </option>
        ))}
      </Select>
      <Select
        id="bill-term"
        name="term_id"
        label="Term"
        required
        value={term}
        error={errors["bill.term_id"]}
        onChange={(e) => setTerm(e.target.value)}
      >
        <option value="">Choose a term</option>
        {(terms.data?.items ?? []).map((t) => (
          <option key={t.id} value={t.id}>
            {t.name}
          </option>
        ))}
      </Select>
      <Button type="submit" variant="secondary" loading={busy}>
        Make draft invoices
      </Button>
      {result ? (
        <p role="status" className="font-semibold sm:col-span-3">
          {result}
        </p>
      ) : null}
    </form>
  );
}

const statusOptions: { value: Status; label: string }[] = [
  { value: "draft", label: "Drafts" },
  { value: "issued", label: "Issued" },
  { value: "void", label: "Voided" },
];

export function InvoiceList() {
  const { can } = useSession();
  const [status, setStatus] = useState<Status>("draft");
  const [term, setTerm] = useState("");
  const [cohort, setCohort] = useState("");
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<string[]>([]);
  const terms = useLoaded(() => api.GET("/api/v1/staff/terms"), []);
  const { data, error, reload } = useLoaded(
    () =>
      api.GET("/api/v1/staff/finance/invoices", {
        params: {
          query: {
            status,
            term_id: term || undefined,
            cohort_id: cohort || undefined,
            q: search || undefined,
            page,
          },
        },
      }),
    [status, term, cohort, search, page],
  );
  const { errors, run } = useAction();
  const pages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1;
  const issuable = can("finance.invoice") && status === "draft";
  const pageIds = data?.items.map((i) => i.id) ?? [];
  const chosen = selected.filter((id) => pageIds.includes(id));

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Invoices</h1>
        <p className="max-w-prose text-ink-muted">
          Drafts can be checked and changed; issuing gives an invoice its number and fixes its due dates.
          Students see an invoice only once it is issued.
        </p>
      </div>

      {can("finance.invoice") ? (
        <Card className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold">Bill a class for a term</h2>
          <p className="text-sm text-ink-muted">
            Makes a draft for every active student in the class who has no invoice for that term yet, from
            their fee structure&apos;s per-term fees. Running it again bills nobody twice.
          </p>
          <BillClass
            onBilled={(t, c) => {
              setStatus("draft");
              setTerm(t);
              setCohort(c);
              setPage(1);
              setSelected([]);
              reload();
            }}
          />
        </Card>
      ) : null}

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
          id="invoices-status"
          name="status"
          label="Show"
          required
          value={status}
          onChange={(e) => {
            setPage(1);
            setSelected([]);
            setStatus(e.target.value as Status);
          }}
        >
          {statusOptions.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </Select>
        <Select
          id="invoices-term"
          name="term_id"
          label="Term"
          value={term}
          onChange={(e) => {
            setPage(1);
            setTerm(e.target.value);
          }}
        >
          <option value="">Any</option>
          {(terms.data?.items ?? []).map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </Select>
        <Input
          id="invoices-q"
          name="q"
          type="search"
          label="Search"
          hint="Invoice number, student number or name."
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <Button type="submit" variant="outline">
          Search
        </Button>
      </form>
      {cohort ? (
        <p>
          Showing one class only.{" "}
          <button
            type="button"
            className="min-h-touch text-royal underline"
            onClick={() => {
              setPage(1);
              setCohort("");
            }}
          >
            Show every class
          </button>
        </p>
      ) : null}

      {!data ? (
        <LoadState error={error} label="Loading invoices" />
      ) : data.items.length === 0 ? (
        <EmptyState
          title="No invoices here"
          description={
            status === "draft"
              ? "No drafts are waiting. Bill a class or open one from a student's account."
              : "No invoices match."
          }
        />
      ) : (
        <>
          <p role="status">{data.total === 1 ? "1 invoice" : `${data.total} invoices`}</p>
          {issuable ? (
            <div className="flex flex-wrap items-center gap-3">
              <Button
                variant="outline"
                onClick={() => setSelected(chosen.length === pageIds.length ? [] : pageIds)}
              >
                {chosen.length === pageIds.length ? "Clear the selection" : "Select every draft on this page"}
              </Button>
              {chosen.length > 0 ? (
                <Dialog
                  title={`Issue ${chosen.length} invoice${chosen.length === 1 ? "" : "s"}?`}
                  description="Each gets the next invoice number and its due dates are fixed from today. Students can then see them. Issued invoices cannot be changed."
                  triggerLabel={`Issue selected (${chosen.length})`}
                  triggerVariant="primary"
                  actions={
                    <>
                      <Button type="submit" variant="outline">
                        Cancel
                      </Button>
                      <Button
                        type="submit"
                        onClick={async () => {
                          const r = await run(
                            () => api.POST("/api/v1/staff/finance/invoices/issue", { body: { ids: chosen } }),
                            `${chosen.length} invoice${chosen.length === 1 ? "" : "s"} issued`,
                            "issue.",
                          );
                          if (r) {
                            setSelected([]);
                            reload();
                          }
                        }}
                      >
                        Issue
                      </Button>
                    </>
                  }
                >
                  <p>If one of them is not ready, nothing is issued and you are told which.</p>
                </Dialog>
              ) : null}
            </div>
          ) : null}
          {errors["issue.ids"] ? (
            <Alert tone="danger" title="Nothing was issued" live>
              {errors["issue.ids"]}
            </Alert>
          ) : null}
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {data.items.map((i) => (
              <li key={i.id} className="flex flex-col gap-1 px-4 py-2 sm:flex-row sm:items-center sm:gap-4">
                {issuable ? (
                  <label className="inline-flex min-h-touch min-w-touch items-center">
                    <input
                      type="checkbox"
                      className="size-6 accent-royal"
                      checked={selected.includes(i.id)}
                      onChange={(e) =>
                        setSelected(
                          e.target.checked ? [...selected, i.id] : selected.filter((x) => x !== i.id),
                        )
                      }
                    />
                    <span className="sr-only">Select the draft for {i.student_name}</span>
                  </label>
                ) : null}
                <Link
                  href={`/staff/finance/invoices/${i.id}`}
                  className="flex min-h-touch min-w-0 flex-1 flex-col justify-center hover:underline"
                >
                  <span className="font-semibold text-royal">
                    <span className="font-mono">{i.student_number}</span> {i.student_name}
                  </span>
                  <span className="text-sm text-ink-muted">
                    {i.number ? <span className="font-mono">{i.number} · </span> : null}
                    {invoiceFor(i.kind, i.term_name)} · {formatKes(i.total_cents)} ·{" "}
                    {i.issued_at
                      ? `issued ${formatNairobi(i.issued_at)}`
                      : `opened ${formatNairobi(i.created_at)}`}
                  </span>
                </Link>
                <InvoiceStatusTag status={i.status} />
              </li>
            ))}
          </ul>
          {pages > 1 ? (
            <nav aria-label="Pages of invoices" className="flex flex-wrap items-center gap-3">
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

// ---------- one invoice ----------

function IssuedInvoice({ inv }: { inv: Invoice }) {
  return (
    <>
      <section aria-labelledby="fees-title" className="flex flex-col gap-2">
        <h2 id="fees-title" className="text-xl font-semibold">
          Fees on this invoice
        </h2>
        {inv.lines.length === 0 ? <p>No fees.</p> : null}
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white px-4 empty:hidden">
          {inv.lines.map((l, i) => (
            <li key={i} className="flex flex-wrap justify-between gap-x-4 py-2">
              <span>
                {l.label}{" "}
                <span className="text-sm text-ink-muted">({categoryLabel[l.category as Category]})</span>
              </span>
              <span className="tabular-nums">{formatKes(l.amount_cents)}</span>
            </li>
          ))}
        </ul>
      </section>
      <p className="text-lg font-semibold">Total {formatKes(inv.total_cents)}</p>
      {inv.status === "issued" ? (
        <section aria-labelledby="due-title" className="flex flex-col gap-2">
          <h2 id="due-title" className="text-xl font-semibold">
            Due
          </h2>
          <ul className="flex flex-col gap-1">
            {inv.instalments.map((n) => (
              <li key={n.seq}>
                {n.amount_cents !== null ? formatKes(n.amount_cents) : ""} by {formatDate(n.due_on)} (
                {n.percent}%)
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </>
  );
}

function DraftEditor({ inv, onSaved }: { inv: Invoice; onSaved: (i: Invoice) => void }) {
  const path = { params: { path: { id: inv.id } } };
  const [lines, setLines] = useState<LineDraft[]>(toLineDrafts(inv.lines));
  const [plan, setPlan] = useState<PlanDraft[]>(toPlanDrafts(inv.instalments));
  const [note, setNote] = useState(inv.note);
  const [voiding, setVoiding] = useState(false);
  const [reason, setReason] = useState("");
  const { busy, errors, setErrors, run } = useAction();
  const total = linesTotal(lines);
  const savedLines = JSON.stringify(toLineDrafts(inv.lines));
  const unsaved =
    JSON.stringify(lines) !== savedLines ||
    JSON.stringify(plan) !== JSON.stringify(toPlanDrafts(inv.instalments)) ||
    note !== inv.note;

  async function save() {
    const sent = linesToSend(lines, "");
    const problems = { ...sent.problems, ...planProblems(plan, "") };
    if (Object.keys(problems).length > 0) {
      setErrors(problems);
      return;
    }
    const saved = await run(
      () =>
        api.PUT("/api/v1/staff/finance/invoices/{id}", {
          ...path,
          body: {
            note,
            lines: sent.lines.map(({ label, category, amount_cents }) => ({ label, category, amount_cents })),
            instalments: planToSend(plan),
          },
        }),
      "Draft saved",
    );
    if (saved) onSaved(saved);
  }

  return (
    <div className="flex flex-col gap-6">
      {inv.lines.length === 0 ? (
        <Alert tone="warning" title="No fees yet">
          {inv.has_structure && inv.kind !== "other"
            ? "Fill it from the fee structure, or add the fees by hand."
            : "Add the fees by hand. There is no fee structure for this student's programme and intake yet."}
        </Alert>
      ) : null}
      {inv.has_structure && inv.kind !== "other" ? (
        <div>
          <Dialog
            title="Fill from the fee structure?"
            description={`The lines and instalments are replaced with the fee structure's ${inv.kind === "enrolment" ? "fees (every fee, as this is the first invoice)" : "per-term fees"}.`}
            triggerLabel="Fill from the fee structure"
            triggerVariant="outline"
            actions={
              <>
                <Button type="submit" variant="outline">
                  Cancel
                </Button>
                <Button
                  type="submit"
                  onClick={async () => {
                    const i = await run(
                      () => api.POST("/api/v1/staff/finance/invoices/{id}/fill", path),
                      "Filled from the fee structure",
                    );
                    if (i) onSaved(i);
                  }}
                >
                  Replace the lines
                </Button>
              </>
            }
          >
            <p>Anything you changed by hand on this draft is lost.</p>
          </Dialog>
        </div>
      ) : null}

      <form
        noValidate
        className="flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <LinesEditor lines={lines} setLines={setLines} errors={errors} form="" withCharge={false} />
        <p className="text-lg font-semibold">Total {formatKes(total)}</p>
        <PlanEditor plan={plan} setPlan={setPlan} errors={errors} form="" total={total} />
        <Textarea
          id="invoice-note"
          name="note"
          label="Note"
          rows={2}
          maxLength={1000}
          value={note}
          error={errors.note}
          onChange={(e) => setNote(e.target.value)}
        />
        <Button type="submit" variant="secondary" loading={busy} className="self-start">
          Save draft
        </Button>
      </form>

      <Card className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold">Issue</h2>
        <p>
          Issuing gives it the next invoice number and fixes the due dates from today:{" "}
          {describePlan(inv.instalments)} The student can then see it. It cannot be changed afterwards.
        </p>
        {unsaved ? <p className="font-semibold">Save your changes first.</p> : null}
        {errors.lines ? (
          <Alert tone="danger" title="Not issued" live>
            {errors.lines}
          </Alert>
        ) : null}
        <div>
          {unsaved || inv.lines.length === 0 ? (
            <Button disabled>Issue invoice</Button>
          ) : (
            <Dialog
              title="Issue this invoice?"
              description={`${formatKes(inv.total_cents)} for ${inv.student_name}. Issued invoices cannot be changed; a mistake is corrected with an adjustment.`}
              triggerLabel="Issue invoice"
              triggerVariant="primary"
              actions={
                <>
                  <Button type="submit" variant="outline">
                    Cancel
                  </Button>
                  <Button
                    type="submit"
                    onClick={async () => {
                      const i = await run(
                        () => api.POST("/api/v1/staff/finance/invoices/{id}/issue", path),
                        "Invoice issued",
                      );
                      if (i) onSaved(i);
                    }}
                  >
                    Issue
                  </Button>
                </>
              }
            >
              <p>Check the fees and instalments above first.</p>
            </Dialog>
          )}
        </div>
      </Card>

      <Card className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold">Not needed?</h2>
        {voiding ? (
          <form
            noValidate
            className="flex flex-col gap-3"
            onSubmit={async (e) => {
              e.preventDefault();
              if (reason.trim().length < 5) {
                setErrors({ reason: "Say why this draft is not needed, in at least 5 characters." });
                return;
              }
              const i = await run(
                () => api.POST("/api/v1/staff/finance/invoices/{id}/void", { ...path, body: { reason } }),
                "Draft voided",
              );
              if (i) onSaved(i);
            }}
          >
            <Textarea
              id="void-reason"
              name="reason"
              label="Why is it not needed?"
              required
              rows={2}
              maxLength={500}
              value={reason}
              error={errors.reason}
              onChange={(e) => setReason(e.target.value)}
            />
            <div className="flex flex-wrap gap-3">
              <Button type="submit" variant="danger" loading={busy}>
                Void draft
              </Button>
              <Button variant="ghost" onClick={() => setVoiding(false)}>
                Cancel
              </Button>
            </div>
          </form>
        ) : (
          <>
            <p>
              A draft that should not be issued can be voided. It keeps no number and is never shown to the
              student.
            </p>
            <Button variant="outline" className="self-start" onClick={() => setVoiding(true)}>
              Void this draft
            </Button>
          </>
        )}
      </Card>
    </div>
  );
}

export function InvoiceView({ id }: { id: string }) {
  const { can } = useSession();
  const {
    data: inv,
    setData,
    error,
  } = useLoaded(() => api.GET("/api/v1/staff/finance/invoices/{id}", { params: { path: { id } } }), [id]);
  // Remount the editor after each save, so it starts from what was saved.
  const [version, setVersion] = useState(0);
  if (!inv) return <LoadState error={error} label="Loading the invoice" />;
  const title = inv.number ?? "Draft invoice";

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb
        items={[
          { label: "Invoices", href: "/staff/finance/invoices" },
          { label: inv.student_name ?? "Student", href: `/staff/finance/students/${inv.student_id}` },
          { label: title },
        ]}
      />
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-3xl font-semibold break-words">
            {inv.number ? <span className="font-mono">{inv.number}</span> : "Draft invoice"}
          </h1>
          <InvoiceStatusTag status={inv.status} />
        </div>
        <p className="text-ink-muted">
          <Link href={`/staff/finance/students/${inv.student_id}`} className="text-royal underline">
            <span className="font-mono">{inv.student_number}</span> {inv.student_name}
          </Link>{" "}
          · {inv.programme_title} · {invoiceFor(inv.kind, inv.term_name)}
        </p>
        <p className="text-sm text-ink-muted">
          Opened {formatNairobi(inv.created_at)} by {inv.created_by_name}
          {inv.issued_at ? ` · issued ${formatNairobi(inv.issued_at)} by ${inv.issued_by_name}` : ""}
        </p>
        {inv.status === "issued" ? (
          <PdfLink href={`/api/v1/staff/finance/invoices/${inv.id}/pdf`}>Download invoice (PDF)</PdfLink>
        ) : null}
      </div>
      {inv.note ? <p className="whitespace-pre-line">{inv.note}</p> : null}
      {inv.status === "void" ? (
        <Alert tone="info" title="Voided">
          This draft was set aside: {inv.void_reason}
        </Alert>
      ) : null}
      {inv.status === "draft" && can("finance.invoice") ? (
        <DraftEditor
          key={version}
          inv={inv}
          onSaved={(i) => {
            setData(i);
            setVersion((v) => v + 1);
          }}
        />
      ) : (
        <IssuedInvoice inv={inv} />
      )}
    </div>
  );
}
