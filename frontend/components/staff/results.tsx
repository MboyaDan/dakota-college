"use client";

import Link from "next/link";
import { useEffect, useId, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select, Textarea } from "@/components/ui/field";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { Table, type Column } from "@/components/ui/table";
import { StatusTag } from "@/components/ui/tag";
import { api, errorMessage } from "@/lib/api/client";
import { openFileLink } from "@/lib/api/file-link";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import {
  cellKey,
  changedEntries,
  describeEvent,
  describeResult,
  describeScheme,
  initialCells,
  outcomeLabel,
  passCount,
  sheetStatus,
  type Outcome,
  type Sheet,
  type SheetStatus,
} from "@/lib/staff/results";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Results (Stage 13b, D-151): the trainer enters marks for a class they
// teach, the Head of School approves or returns them, and the registrar
// releases them to students and issues official transcripts. The backend
// enforces every step; these screens offer only what the viewer may do.

type SheetItem = components["schemas"]["ResultSheetItem"];
type StudentResult = components["schemas"]["StudentResult"];
type Transcript = components["schemas"]["Transcript"];
type Row = Sheet["rows"][number];

function SheetStatusTag({ status }: { status: SheetStatus }) {
  return <StatusTag status={sheetStatus[status].tone}>{sheetStatus[status].label}</StatusTag>;
}

function SheetHeader({ sheet, crumbs }: { sheet: Sheet; crumbs: { label: string; href?: string }[] }) {
  const title = `${sheet.unit_code} ${sheet.unit_title}`;
  return (
    <>
      <Breadcrumb items={[...crumbs, { label: title }]} />
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-3xl font-semibold break-words">Results: {title}</h1>
          <SheetStatusTag status={sheet.status} />
        </div>
        <p className="text-ink-muted">
          {sheet.term_name}
          {sheet.released_version > 0
            ? ` · released version ${sheet.released_version}`
            : " · not released yet"}
        </p>
        <p>{describeScheme(sheet.scheme)}</p>
        {sheet.scheme.kind === "marks" ? (
          <p className="text-sm text-ink-muted">
            Grades: {sheet.scheme.grade_bands.map((b) => `${b.grade} from ${b.min}`).join(", ")}
          </p>
        ) : null}
      </div>
      {sheet.scheme.is_default ? (
        <Alert tone="warning" title="Placeholder assessment">
          The college has not set how this programme is assessed, so these results use a placeholder. Ask the
          registrar or your Head of School to set it under Units and terms before results are released.
        </Alert>
      ) : null}
    </>
  );
}

function markText(sheet: Sheet, row: Row, key: string): string {
  const m = row.marks[key];
  if (sheet.scheme.kind === "competency")
    return m?.outcome ? outcomeLabel[m.outcome as Outcome] : "Not entered";
  return m?.score == null ? "Not entered" : String(m.score);
}

function ResultTag({ r }: { r: Pick<Row["result"], "complete" | "passed" | "outcome"> }) {
  if (!r.complete) return <StatusTag status="warning">Marks missing</StatusTag>;
  const label = r.outcome ? outcomeLabel[r.outcome as Outcome] : r.passed ? "Pass" : "Fail";
  return <StatusTag status={r.passed ? "success" : "danger"}>{label}</StatusTag>;
}

/** The whole sheet, read only: every mark, the total, grade and result. */
export function ResultTable({ sheet }: { sheet: Sheet }) {
  const marks = sheet.scheme.kind === "marks";
  const columns: Column<Row>[] = [
    {
      key: "number",
      header: "Student number",
      cell: (r) => <span className="font-mono">{r.student_number}</span>,
    },
    { key: "name", header: "Name", cell: (r) => r.name },
    ...(marks
      ? sheet.scheme.components.map((c) => ({
          key: `c-${c.key}`,
          header: `${c.name} (${c.weight}%)`,
          numeric: true,
          cell: (r: Row) => markText(sheet, r, c.key ?? ""),
        }))
      : []),
    ...(marks
      ? [
          { key: "total", header: "Total", numeric: true, cell: (r: Row) => r.result.total ?? "None" },
          { key: "grade", header: "Grade", cell: (r: Row) => r.result.grade || "None" },
        ]
      : []),
    { key: "result", header: "Result", cell: (r) => <ResultTag r={r.result} /> },
  ];
  const { passed, complete } = passCount(sheet);
  return (
    <div className="flex flex-col gap-2">
      <Table
        caption={`Results (${sheet.rows.length} students)`}
        rows={sheet.rows}
        rowKey={(r) => r.student_id}
        empty="Nobody is registered for this class."
        columns={columns}
      />
      {sheet.rows.length > 0 ? (
        <p className="text-sm text-ink-muted">
          {passed} of {complete} with complete results passed
          {complete < sheet.rows.length ? `; ${sheet.rows.length - complete} still have marks missing` : ""}.
        </p>
      ) : null}
    </div>
  );
}

function History({ sheet }: { sheet: Sheet }) {
  return (
    <section aria-labelledby="sheet-history-title" className="flex flex-col gap-2">
      <h2 id="sheet-history-title" className="text-xl font-semibold">
        History
      </h2>
      {sheet.events.length === 0 ? <p>Nothing has been sent for approval yet.</p> : null}
      <ol className="flex flex-col gap-2">
        {[...sheet.events].reverse().map((e, i) => (
          <li key={i} className="border-l-4 border-stone pl-3">
            <span className="font-semibold">{describeEvent(e.from, e.to)}</span> by {e.actor_name},{" "}
            {formatNairobi(e.at)}
            {e.reason ? <span className="block whitespace-pre-line text-ink-muted">{e.reason}</span> : null}
          </li>
        ))}
      </ol>
    </section>
  );
}

function useSheet(load: () => Promise<{ data?: Sheet; error?: unknown }>, deps: unknown[]) {
  const [sheet, setSheet] = useState<Sheet | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    void load().then(({ data, error: err }) => {
      if (!active) return;
      if (data) setSheet(data);
      else setError(errorMessage(err));
    });
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reload when the id changes
  }, deps);
  return { sheet, setSheet, error };
}

function SheetLoading({ error }: { error: string | null }) {
  if (error) {
    return (
      <Alert tone="danger" title="Could not load the results">
        {error}
      </Alert>
    );
  }
  return (
    <Loading label="Loading the results">
      <SkeletonText lines={10} />
    </Loading>
  );
}

// ---------- trainer: marks entry ----------

export function ClassResults({ id }: { id: string }) {
  const path = { params: { path: { id } } };
  const { sheet, setSheet, error } = useSheet(
    () => api.GET("/api/v1/staff/teaching/{id}/results", path),
    [id],
  );
  // Only what the trainer has typed; every other field shows the saved mark.
  const [edits, setEdits] = useState<Record<string, string>>({});
  const [problems, setProblems] = useState<Record<string, string>>({});
  const [sent, setSent] = useState<string[]>([]);
  const [correcting, setCorrecting] = useState(false);
  const [reason, setReason] = useState("");
  const { busy, errors, setErrors, run } = useAction();

  if (!sheet) return <SheetLoading error={error} />;

  const editable = sheet.status === "draft" || correcting;
  const saved = initialCells(sheet);
  const cells = { ...saved, ...edits };
  const pending = changedEntries(sheet, cells);
  const unsaved = pending.entries.length > 0 || Object.keys(pending.problems).length > 0;
  const competency = sheet.scheme.kind === "competency";
  const cellError = (key: string) => {
    const i = sent.indexOf(key);
    return problems[key] ?? (i >= 0 ? errors[`entries.${i}`] : undefined);
  };
  const loaded = (s: Sheet | null) => {
    if (!s) return false;
    setSheet(s);
    setEdits({});
    setProblems({});
    setSent([]);
    return true;
  };

  async function save() {
    const { entries, keys, problems: found } = changedEntries(sheet!, cells);
    setProblems(found);
    setErrors({});
    if (Object.keys(found).length > 0) return;
    if (entries.length === 0) {
      setErrors({ entries: "Nothing has changed since the last save." });
      return;
    }
    if (correcting && !reason.trim()) {
      setErrors({ reason: "Say why you are correcting approved results." });
      return;
    }
    setSent(keys);
    const result = await run(
      () =>
        api.PUT("/api/v1/staff/teaching/{id}/results/marks", {
          ...path,
          body: { entries, reason: correcting ? reason : undefined },
        }),
      correcting ? "Correction saved. Send it for approval when it is complete." : "Marks saved",
    );
    if (loaded(result)) {
      setCorrecting(false);
      setReason("");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <SheetHeader
        sheet={sheet}
        crumbs={[
          { label: "My teaching", href: "/staff/teaching" },
          { label: sheet.unit_code, href: `/staff/teaching/${id}` },
        ]}
      />

      {sheet.status === "submitted" ? (
        <Alert tone="info" title="Waiting for the Head of School">
          You can change marks again only if they return the results to you.
        </Alert>
      ) : null}
      {(sheet.status === "approved" || sheet.status === "released") && !correcting ? (
        <Alert tone="info" title={sheet.status === "released" ? "Released to students" : "Approved"}>
          <p>
            To correct a mark, give the reason. The results go back through approval and release, and students
            keep seeing {sheet.released_version > 0 ? "the released version" : "nothing"} until then.
          </p>
          <Button variant="outline" className="mt-3" onClick={() => setCorrecting(true)}>
            Correct marks
          </Button>
        </Alert>
      ) : null}

      {editable ? (
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          {sheet.rows.length === 0 ? (
            <EmptyState
              title="Nobody is registered"
              description="Students appear here once they are registered for this class."
            />
          ) : (
            <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
              {sheet.rows.map((row) => (
                <StudentMarks
                  key={row.student_id}
                  sheet={sheet}
                  row={row}
                  cells={cells}
                  setCell={(k, v) => setEdits({ ...edits, [k]: v })}
                  cellError={cellError}
                  unsaved={sheet.scheme.components.some((c) => {
                    const k = cellKey(row.student_id, c.key ?? "");
                    return (cells[k] ?? "") !== (saved[k] ?? "");
                  })}
                />
              ))}
            </ul>
          )}
          {errors.entries ? (
            <Alert tone="danger" title="Not saved" live>
              {errors.entries}
            </Alert>
          ) : null}
          {Object.keys(problems).length > 0 ? (
            <Alert tone="danger" title="Some marks need fixing" live>
              The fields with a message below each say what to change.
            </Alert>
          ) : null}
          {correcting ? (
            <Textarea
              id="correction-reason"
              name="reason"
              label="Why are you correcting these results?"
              required
              rows={2}
              maxLength={500}
              value={reason}
              error={errors.reason}
              hint="The Head of School and the registrar see this. For example: exam script for 0003 was marked twice."
              onChange={(e) => setReason(e.target.value)}
            />
          ) : null}
          <div className="flex flex-wrap gap-3">
            <Button type="submit" variant={sheet.complete && !unsaved ? "outline" : "primary"} loading={busy}>
              {correcting ? "Save the correction" : "Save marks"}
            </Button>
            {correcting ? (
              <Button
                variant="ghost"
                onClick={() => {
                  setCorrecting(false);
                  setEdits({});
                  setProblems({});
                }}
              >
                Cancel the correction
              </Button>
            ) : null}
          </div>
        </form>
      ) : (
        <ResultTable sheet={sheet} />
      )}

      {sheet.status === "draft" && sheet.rows.length > 0 ? (
        <Card className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold">Send for approval</h2>
          {sheet.complete ? (
            <p>
              Every student has {competency ? "an outcome" : "all their marks"}. Your Head of School checks
              them next; you cannot change them while they wait.
            </p>
          ) : (
            <p>Enter and save {competency ? "an outcome" : "every mark"} for every student first.</p>
          )}
          {unsaved ? <p className="font-semibold">Save your changes first.</p> : null}
          {errors["submit.entries"] ? (
            <Alert tone="danger" title="Not sent" live>
              {errors["submit.entries"]}
            </Alert>
          ) : null}
          <Button
            className="self-start"
            disabled={!sheet.complete || unsaved || busy}
            onClick={async () => {
              const s = await run(
                () => api.POST("/api/v1/staff/teaching/{id}/results/submit", path),
                "Sent to the Head of School",
                "submit.",
              );
              loaded(s);
            }}
          >
            Send for approval
          </Button>
        </Card>
      ) : null}

      <History sheet={sheet} />
    </div>
  );
}

function StudentMarks({
  sheet,
  row,
  cells,
  setCell,
  cellError,
  unsaved,
}: {
  sheet: Sheet;
  row: Row;
  cells: Record<string, string>;
  setCell: (key: string, value: string) => void;
  cellError: (key: string) => string | undefined;
  unsaved: boolean;
}) {
  const nameId = useId();
  const short = row.student_number.replace(/[^A-Za-z0-9]+/g, "-");
  return (
    <li>
      <div
        role="group"
        aria-labelledby={nameId}
        className="flex flex-col gap-3 p-4 lg:flex-row lg:items-start lg:gap-6"
      >
        <div id={nameId} className="flex flex-col lg:w-52 lg:shrink-0 lg:pt-8">
          <span className="font-mono">{row.student_number}</span>
          <span className="font-semibold">{row.name}</span>
        </div>
        <div className="grid flex-1 grid-cols-1 items-end gap-3 min-[400px]:grid-cols-2 sm:grid-cols-3">
          {sheet.scheme.components.map((c) => {
            const key = cellKey(row.student_id, c.key ?? "");
            const fieldId = `mark-${short}-${c.key}`;
            return sheet.scheme.kind === "competency" ? (
              <Select
                key={key}
                id={fieldId}
                name={fieldId}
                label="Outcome"
                required
                value={cells[key] ?? ""}
                error={cellError(key)}
                onChange={(e) => setCell(key, e.target.value)}
              >
                <option value="">Not entered</option>
                <option value="competent">Competent</option>
                <option value="not_yet_competent">Not yet competent</option>
              </Select>
            ) : (
              <Input
                key={key}
                id={fieldId}
                name={fieldId}
                label={`${c.name} (${c.weight}%)`}
                required
                inputMode="numeric"
                autoComplete="off"
                maxLength={3}
                value={cells[key] ?? ""}
                error={cellError(key)}
                onChange={(e) => setCell(key, e.target.value)}
              />
            );
          })}
        </div>
        <div className="flex flex-col gap-1 lg:w-44 lg:shrink-0 lg:pt-8">
          {unsaved ? (
            <span className="text-sm font-semibold">Not saved yet</span>
          ) : (
            <>
              <ResultTag r={row.result} />
              {row.result.complete && sheet.scheme.kind === "marks" ? (
                <span className="text-sm">
                  Total {row.result.total}, grade {row.result.grade}
                </span>
              ) : null}
            </>
          )}
        </div>
      </div>
    </li>
  );
}

// ---------- Head of School and registrar: queues ----------

type Step = "approve" | "release";

const steps = {
  approve: {
    title: "Results to approve",
    intro:
      "Results your trainers have sent from classes in your schools. Approve them, or return them with a reason.",
    base: "/staff/result-approvals",
    statuses: [
      { value: "submitted", label: "Waiting for approval" },
      { value: "approved", label: "Approved, not released" },
      { value: "released", label: "Released" },
      { value: "draft", label: "Being marked" },
    ],
  },
  release: {
    title: "Results to release",
    intro:
      "Approved results. Releasing them shows them to students, and a correction released again replaces them.",
    base: "/staff/result-releases",
    statuses: [
      { value: "approved", label: "Approved, not released" },
      { value: "released", label: "Released" },
    ],
  },
} as const;

export function ResultQueue({ step }: { step: Step }) {
  const s = steps[step];
  const [status, setStatus] = useState<string>(s.statuses[0].value);
  const [items, setItems] = useState<SheetItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    const call =
      step === "approve"
        ? api.GET("/api/v1/staff/result-approvals", {
            params: { query: { status: status as "submitted" | "approved" | "released" | "draft" } },
          })
        : api.GET("/api/v1/staff/result-releases", {
            params: { query: { status: status as "approved" | "released" } },
          });
    void call.then(({ data, error: err }) => {
      if (!active) return;
      if (data) {
        setItems(data.items);
        setError(null);
      } else setError(errorMessage(err));
    });
    return () => {
      active = false;
    };
  }, [step, status]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">{s.title}</h1>
        <p className="max-w-prose text-ink-muted">{s.intro}</p>
      </div>
      <div className="sm:max-w-xs">
        <Select
          id="sheet-status"
          name="status"
          label="Show"
          required
          value={status}
          onChange={(e) => {
            setItems(null);
            setStatus(e.target.value);
          }}
        >
          {s.statuses.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </Select>
      </div>
      {error ? (
        <Alert tone="danger" title="Could not load the results">
          {error}
        </Alert>
      ) : items === null ? (
        <Loading label="Loading the results">
          <Skeleton className="h-32 w-full" />
        </Loading>
      ) : items.length === 0 ? (
        <EmptyState
          title="Nothing here"
          description={
            status === s.statuses[0].value
              ? "When results reach this step, they appear here."
              : "No classes have results at this step."
          }
        />
      ) : (
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
          {items.map((i) => (
            <li key={i.id}>
              <Link
                href={`${s.base}/${i.id}`}
                className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:gap-4"
              >
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="font-semibold text-royal">
                    <span className="font-mono">{i.unit_code}</span> {i.unit_title}
                  </span>
                  <span className="text-sm text-ink-muted">
                    {i.term_name} · {i.students === 1 ? "1 student" : `${i.students} students`} · updated{" "}
                    {formatNairobi(i.updated_at)}
                  </span>
                </span>
                <SheetStatusTag status={i.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

// ---------- Head of School and registrar: one sheet ----------

export function ResultReview({ id, step }: { id: string; step: Step }) {
  const s = steps[step];
  const path = { params: { path: { id } } };
  const { sheet, setSheet, error } = useSheet(
    () =>
      step === "approve"
        ? api.GET("/api/v1/staff/result-approvals/{id}", path)
        : api.GET("/api/v1/staff/result-releases/{id}", path),
    [id, step],
  );
  const [reason, setReason] = useState("");
  const { busy, errors, setErrors, run } = useAction();

  if (!sheet) return <SheetLoading error={error} />;
  const done = (x: Sheet | null) => {
    if (x) setSheet(x);
  };
  const correction = sheet.released_version > 0;

  return (
    <div className="flex flex-col gap-6">
      <SheetHeader sheet={sheet} crumbs={[{ label: s.title, href: s.base }]} />
      <ResultTable sheet={sheet} />

      {step === "approve" && sheet.status === "submitted" ? (
        <Card className="flex flex-col gap-4">
          <h2 className="text-xl font-semibold">Your decision</h2>
          <p>Approving sends these results to the registrar to release. Students see nothing until then.</p>
          <Button
            className="self-start"
            loading={busy}
            onClick={async () =>
              done(
                await run(
                  () => api.POST("/api/v1/staff/result-approvals/{id}/approve", { ...path, body: {} }),
                  "Results approved",
                ),
              )
            }
          >
            Approve results
          </Button>
          <form
            noValidate
            className="flex flex-col gap-3 border-t border-stone pt-4"
            onSubmit={async (e) => {
              e.preventDefault();
              if (!reason.trim()) {
                setErrors({ "return.reason": "Say what the trainer needs to change." });
                return;
              }
              done(
                await run(
                  () => api.POST("/api/v1/staff/result-approvals/{id}/return", { ...path, body: { reason } }),
                  "Returned to the trainer",
                  "return.",
                ),
              );
              setReason("");
            }}
          >
            <Textarea
              id="return-reason"
              name="reason"
              label="Or return them to the trainer"
              required
              rows={2}
              maxLength={500}
              value={reason}
              error={errors["return.reason"]}
              hint="The trainer sees this. For example: check the exam marks for DTC/SAMPLE/0002."
              onChange={(e) => setReason(e.target.value)}
            />
            <Button type="submit" variant="outline" className="self-start" disabled={busy}>
              Return with this reason
            </Button>
          </form>
        </Card>
      ) : null}

      {step === "release" && sheet.status === "approved" ? (
        <Card className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold">
            {correction ? "Release the correction" : "Release to students"}
          </h2>
          <p>
            {correction
              ? `Students now see version ${sheet.released_version}. Releasing makes this version ${sheet.released_version + 1} what they see.`
              : "Each student will see their own result in the student portal."}
          </p>
          <div>
            <Dialog
              title={correction ? "Release the correction?" : "Release these results?"}
              description={`${sheet.rows.length} students in ${sheet.unit_code} will see these results. This cannot be undone; a later correction is released as a new version.`}
              triggerLabel={correction ? "Release the correction" : "Release results"}
              triggerVariant="primary"
              actions={
                <>
                  <Button type="submit" variant="outline">
                    Cancel
                  </Button>
                  <Button
                    type="submit"
                    onClick={async () =>
                      done(
                        await run(
                          () => api.POST("/api/v1/staff/result-releases/{id}/release", { ...path, body: {} }),
                          "Results released to students",
                        ),
                      )
                    }
                  >
                    Release
                  </Button>
                </>
              }
            >
              <p>Check the marks above before you release them.</p>
            </Dialog>
          </div>
        </Card>
      ) : null}

      {step === "approve" && sheet.status !== "submitted" ? (
        <p className="text-ink-muted">
          {sheet.status === "draft"
            ? "The trainer is still entering marks. They come to you once sent."
            : "You have nothing to decide on these results."}
        </p>
      ) : null}
      {step === "release" && sheet.status === "released" ? (
        <p className="text-ink-muted">Released. A correction comes back here once it is approved.</p>
      ) : null}

      <History sheet={sheet} />
    </div>
  );
}

// ---------- the student record: released results and transcripts ----------

export function StudentResultsPanel({ studentId }: { studentId: string }) {
  const { can } = useSession();
  const release = can("academics.results.release");
  const path = { params: { path: { id: studentId } } };
  const [data, setData] = useState<{ results: StudentResult[]; transcripts: Transcript[] } | null>(null);
  const [linkError, setLinkError] = useState<string | null>(null);
  const { busy, run } = useAction();

  useEffect(() => {
    api
      .GET("/api/v1/staff/students/{id}/results", { params: { path: { id: studentId } } })
      .then(({ data: d }) => {
        setData(d ?? { results: [], transcripts: [] });
      });
  }, [studentId]);

  return (
    <section aria-labelledby="results-title" className="flex flex-col gap-3">
      <h2 id="results-title" className="text-xl font-semibold">
        Results
      </h2>
      {data === null ? (
        <Loading label="Loading results">
          <Skeleton className="h-20 w-full" />
        </Loading>
      ) : (
        <>
          <Table
            caption="Released results"
            captionHidden
            rows={data.results}
            rowKey={(r) => `${r.term_name}-${r.unit_code}`}
            empty="No results released yet."
            columns={[
              { key: "term", header: "Term", cell: (r) => r.term_name },
              {
                key: "unit",
                header: "Unit",
                cell: (r) => (
                  <>
                    <span className="font-mono">{r.unit_code}</span> {r.unit_title}
                  </>
                ),
              },
              { key: "result", header: "Result", cell: (r) => describeResult({ ...r, complete: true }) },
              {
                key: "released",
                header: "Released",
                cell: (r) =>
                  `${formatNairobi(r.released_at)}${r.version > 1 ? ` (version ${r.version})` : ""}`,
              },
            ]}
          />
          <h3 className="text-lg font-semibold">Official transcripts</h3>
          {data.transcripts.length === 0 ? <p>None issued.</p> : null}
          <ul className="flex flex-col gap-2">
            {data.transcripts.map((t) => (
              <li
                key={t.id}
                className="flex flex-col gap-1 rounded-control border border-stone bg-white px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
              >
                <span>
                  <span className="font-mono">{t.number}</span>
                  <span className="block text-sm text-ink-muted">
                    Issued {formatNairobi(t.issued_at)} by {t.issued_by_name}
                  </span>
                </span>
                <Button
                  variant="ghost"
                  onClick={async () =>
                    setLinkError(
                      await openFileLink(() =>
                        api.POST("/api/v1/staff/students/{id}/transcripts/{item}/link", {
                          params: { path: { id: studentId, item: t.id } },
                        }),
                      ),
                    )
                  }
                >
                  Open<span className="sr-only"> transcript {t.number}</span>
                </Button>
              </li>
            ))}
          </ul>
          {linkError ? (
            <Alert tone="danger" title="Could not open the transcript" live>
              {linkError}
            </Alert>
          ) : null}
          {release && data.results.length > 0 ? (
            <div>
              <Dialog
                title="Issue an official transcript?"
                description="It lists every released result and takes the next transcript number. The student can download it from their portal."
                triggerLabel="Issue an official transcript"
                triggerVariant="outline"
                actions={
                  <>
                    <Button type="submit" variant="outline">
                      Cancel
                    </Button>
                    <Button
                      type="submit"
                      loading={busy}
                      onClick={async () => {
                        const d = await run(
                          () => api.POST("/api/v1/staff/students/{id}/transcripts", { ...path, body: {} }),
                          "Transcript issued",
                        );
                        if (d) setData(d);
                      }}
                    >
                      Issue transcript
                    </Button>
                  </>
                }
              >
                <p>
                  If the student asked for it, issue it from Student requests instead, so their request is
                  closed too.
                </p>
              </Dialog>
            </div>
          ) : null}
        </>
      )}
    </section>
  );
}
