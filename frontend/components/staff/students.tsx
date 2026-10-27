"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button, buttonClasses } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Checkbox, Input, Select, Textarea } from "@/components/ui/field";
import { Pagination } from "@/components/ui/pagination";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { Table } from "@/components/ui/table";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { relationships, studentStatus, studentStatuses, type StudentStatus } from "@/lib/staff/academics";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { useSession } from "./session";
import { StudentResultsPanel } from "./results";
import { useAction } from "./use-action";

type Page = components["schemas"]["StudentPage"];
type Student = components["schemas"]["Student"];
type Cohort = components["schemas"]["Cohort"];
type Guardian = components["schemas"]["Guardian"];
type GuardianInput = components["schemas"]["GuardianInput"];

export function StudentStatusTag({ status }: { status: StudentStatus }) {
  return <StatusTag status={studentStatus[status].tone}>{studentStatus[status].label}</StatusTag>;
}

function useCohorts() {
  const [cohorts, setCohorts] = useState<Cohort[]>([]);
  useEffect(() => {
    api.GET("/api/v1/staff/cohorts").then(({ data }) => {
      if (data) setCohorts(data.items);
    });
  }, []);
  return cohorts;
}

// ---------- list ----------

export function StudentList() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const cohorts = useCohorts();
  const [page, setPage] = useState<Page | null>(null);
  const [error, setError] = useState<string | null>(null);
  const status = params.get("status") ?? "";
  const cohort = params.get("cohort") ?? "";
  const q = params.get("q") ?? "";
  const pageNumber = Math.max(1, Number(params.get("page")) || 1);

  const hrefWith = useCallback(
    (changes: Record<string, string>) => {
      const next = new URLSearchParams(params.toString());
      for (const [k, v] of Object.entries(changes)) {
        if (v) next.set(k, v);
        else next.delete(k);
      }
      if (!("page" in changes)) next.delete("page");
      const s = next.toString();
      return s ? `${pathname}?${s}` : pathname;
    },
    [params, pathname],
  );

  useEffect(() => {
    let active = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- show the loading state for each new query
    setPage(null);
    api
      .GET("/api/v1/staff/students", {
        params: {
          query: {
            status: (status || undefined) as StudentStatus | undefined,
            cohort: cohort || undefined,
            q: q || undefined,
            page: pageNumber,
          },
        },
      })
      .then(({ data, error: err }) => {
        if (!active) return;
        if (data) {
          setPage(data);
          setError(null);
        } else setError(errorMessage(err));
      });
    return () => {
      active = false;
    };
  }, [status, cohort, q, pageNumber]);

  function search(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    router.push(
      hrefWith({
        status: String(data.get("status") ?? ""),
        cohort: String(data.get("cohort") ?? ""),
        q: String(data.get("q") ?? "").trim(),
      }),
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Students</h1>
        <p className="max-w-prose text-ink-muted">
          Enrolled students in the schools you work in. New students appear here when the registrar enrols
          them.
        </p>
      </div>

      {/* noValidate: "Any" is an empty choice, which a required select would refuse. */}
      <form
        noValidate
        onSubmit={search}
        className="grid grid-cols-1 items-end gap-4 sm:grid-cols-2 lg:grid-cols-[1fr_2fr_2fr_auto]"
        key={`${status}|${cohort}|${q}`}
      >
        <Select id="student-status" name="status" label="Status" defaultValue={status} required>
          <option value="">Any status</option>
          {studentStatuses.map((s) => (
            <option key={s} value={s}>
              {studentStatus[s].label}
            </option>
          ))}
        </Select>
        <Select id="student-cohort" name="cohort" label="Cohort" defaultValue={cohort} required>
          <option value="">Any cohort</option>
          {cohorts.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="student-q" className="font-semibold">
            Search
          </label>
          <input
            id="student-q"
            name="q"
            type="search"
            defaultValue={q}
            placeholder="Student number, name or email"
            className="block min-h-touch w-full rounded-control border border-ink-muted bg-white px-3 py-2"
          />
        </div>
        <button type="submit" className={buttonClasses("primary")}>
          Show
        </button>
      </form>

      {error ? (
        <Alert tone="danger" title="Could not load students">
          {error}
        </Alert>
      ) : null}
      {!page && !error ? (
        <Loading label="Loading students">
          <div className="flex flex-col gap-2">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        </Loading>
      ) : null}
      {page && page.items.length === 0 ? (
        <EmptyState
          title="No students here"
          description={
            status || cohort || q
              ? "Nothing matches these filters."
              : "Students appear once enrolled. If you expected some, ask the ICT administrator which schools you are assigned to."
          }
        />
      ) : null}
      {page && page.items.length > 0 ? (
        <>
          <p className="text-sm text-ink-muted" role="status">
            {page.total === 1 ? "1 student" : `${page.total} students`}
          </p>
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {page.items.map((s) => (
              <li key={s.id}>
                <Link
                  href={`/staff/students/${s.id}`}
                  className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:gap-4"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="font-semibold text-royal">{s.name}</span>
                    <span className="text-sm break-words text-ink-muted">
                      <span className="font-mono">{s.student_number}</span> · {s.programme_title}
                    </span>
                  </span>
                  <StudentStatusTag status={s.status} />
                  <span className="text-sm text-ink-muted sm:w-56">{s.cohort_name ?? "No cohort"}</span>
                </Link>
              </li>
            ))}
          </ul>
          <Pagination
            page={page.page}
            totalPages={page.total_pages}
            hrefFor={(p) => hrefWith({ page: p === 1 ? "" : String(p) })}
            label="Student pages"
          />
        </>
      ) : null}
    </div>
  );
}

// ---------- one student ----------

const blankGuardian: GuardianInput = {
  full_name: "",
  relationship: "mother",
  phone: "",
  email: "",
  is_primary: false,
};

function GuardianForm({
  initial,
  submitLabel,
  errors,
  busy,
  onSubmit,
  onCancel,
  idPrefix,
}: {
  initial: GuardianInput;
  submitLabel: string;
  errors: Record<string, string>;
  busy: boolean;
  onSubmit: (g: GuardianInput) => void;
  onCancel?: () => void;
  idPrefix: string;
}) {
  const [g, setG] = useState(initial);
  const e = (f: string) => errors[`${idPrefix}.${f}`];
  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={(ev) => {
        ev.preventDefault();
        onSubmit(g);
      }}
    >
      <Input
        id={`${idPrefix}-name`}
        name="full_name"
        label="Full name"
        required
        maxLength={120}
        value={g.full_name}
        error={e("full_name")}
        onChange={(ev) => setG({ ...g, full_name: ev.target.value })}
      />
      <Select
        id={`${idPrefix}-relationship`}
        name="relationship"
        label="Relationship to the student"
        required
        value={g.relationship}
        error={e("relationship")}
        onChange={(ev) => setG({ ...g, relationship: ev.target.value as Guardian["relationship"] })}
      >
        {relationships.map((r) => (
          <option key={r.value} value={r.value}>
            {r.label}
          </option>
        ))}
      </Select>
      <Input
        id={`${idPrefix}-phone`}
        name="phone"
        type="tel"
        label="Phone"
        value={g.phone}
        error={e("phone")}
        hint="A phone number or an email address is needed."
        onChange={(ev) => setG({ ...g, phone: ev.target.value })}
      />
      <Input
        id={`${idPrefix}-email`}
        name="email"
        type="email"
        label="Email"
        value={g.email}
        error={e("email")}
        onChange={(ev) => setG({ ...g, email: ev.target.value })}
      />
      <Checkbox
        id={`${idPrefix}-primary`}
        name="is_primary"
        label="Main contact"
        checked={g.is_primary}
        onChange={(ev) => setG({ ...g, is_primary: ev.target.checked })}
      />
      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="secondary" loading={busy}>
          {submitLabel}
        </Button>
        {onCancel ? (
          <Button variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
      </div>
    </form>
  );
}

export function StudentRecord({ id }: { id: string }) {
  const { can } = useSession();
  const update = can("students.update");
  const cohorts = useCohorts();
  const { busy, errors, setErrors, run } = useAction();
  const [student, setStudent] = useState<Student | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<StudentStatus>("active");
  const [reason, setReason] = useState("");
  const [editing, setEditing] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  useEffect(() => {
    api.GET("/api/v1/staff/students/{id}", { params: { path: { id } } }).then(({ data, error: err }) => {
      if (data) setStudent(data);
      else setError(errorMessage(err));
    });
  }, [id]);

  const path = { params: { path: { id } } };
  const done = (s: Student | null) => {
    if (s) setStudent(s);
    return s !== null;
  };

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the student">
        {error}
      </Alert>
    );
  }
  if (!student) {
    return (
      <Loading label="Loading the student">
        <SkeletonText lines={10} />
      </Loading>
    );
  }
  const name = [student.first_name, student.middle_name, student.last_name].filter(Boolean).join(" ");
  const ownCohorts = cohorts.filter((c) => c.programme_id === student.programme_id);
  // The current status is not offered, so start on the first one that is.
  const nextStatus = status === student.status ? studentStatuses.find((s) => s !== student.status)! : status;

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "Students", href: "/staff/students" }, { label: name }]} />
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-3xl font-semibold break-words">{name}</h1>
          <StudentStatusTag status={student.status} />
        </div>
        <p className="text-ink-muted">
          <span className="font-mono">{student.student_number}</span> · {student.programme_title}
          {student.intake_name ? ` · ${student.intake_name}` : ""} · enrolled{" "}
          {formatNairobi(student.enrolled_at)}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="flex min-w-0 flex-col gap-6">
          <Card>
            <h2 className="text-xl font-semibold">Details</h2>
            <dl className="grid grid-cols-1 gap-x-4 gap-y-2 sm:grid-cols-[12rem_1fr]">
              <dt className="text-sm text-ink-muted">Email</dt>
              <dd className="break-all">{student.email}</dd>
              <dt className="text-sm text-ink-muted">Phone</dt>
              <dd>{student.phone ?? "Not given"}</dd>
              <dt className="text-sm text-ink-muted">Date of birth</dt>
              <dd>{student.date_of_birth ? formatDate(student.date_of_birth) : "Not given"}</dd>
              <dt className="text-sm text-ink-muted">Cohort</dt>
              <dd>{student.cohort_name ?? "None"}</dd>
            </dl>
            <Link
              href={`/staff/applications/${student.application_id}`}
              className="min-h-touch content-center self-start text-royal underline"
            >
              Application as submitted
            </Link>
            {can("finance.view") ? (
              <Link
                href={`/staff/finance/students/${student.id}`}
                className="min-h-touch content-center self-start text-royal underline"
              >
                Fees account
              </Link>
            ) : null}
          </Card>

          <section aria-labelledby="units-title" className="flex flex-col gap-3">
            <h2 id="units-title" className="text-xl font-semibold">
              Units registered
            </h2>
            <Table
              caption="Units registered"
              captionHidden
              rows={student.registrations}
              rowKey={(r) => r.id}
              empty="Not registered for any unit yet."
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
                {
                  key: "status",
                  header: "Status",
                  cell: (r) =>
                    r.status === "registered" ? "Registered" : `Dropped ${formatNairobi(r.dropped_at)}`,
                },
              ]}
            />
          </section>

          <StudentResultsPanel studentId={student.id} />

          <section aria-labelledby="guardians-title" className="flex flex-col gap-3">
            <h2 id="guardians-title" className="text-xl font-semibold">
              Parents, guardians and next of kin
            </h2>
            {student.guardians.length === 0 ? <p>None recorded.</p> : null}
            <ul className="flex flex-col gap-2">
              {student.guardians.map((g) => (
                <li key={g.id} className="rounded-card border border-stone bg-white p-4">
                  {editing === g.id ? (
                    <GuardianForm
                      idPrefix={`edit-${g.id.slice(0, 8)}`}
                      initial={{ ...g }}
                      submitLabel="Save changes"
                      errors={errors}
                      busy={busy}
                      onCancel={() => setEditing(null)}
                      onSubmit={async (body) => {
                        const s = await run(
                          () =>
                            api.PUT("/api/v1/staff/students/{id}/guardians/{item}", {
                              params: { path: { id, item: g.id } },
                              body,
                            }),
                          "Saved",
                          `edit-${g.id.slice(0, 8)}.`,
                        );
                        if (done(s)) setEditing(null);
                      }}
                    />
                  ) : (
                    <div className="flex flex-col gap-1">
                      <p className="font-semibold">
                        {g.full_name}{" "}
                        <span className="font-normal text-ink-muted">
                          ({relationships.find((r) => r.value === g.relationship)?.label})
                          {g.is_primary ? " · main contact" : ""}
                        </span>
                      </p>
                      <p className="text-sm break-all">{[g.phone, g.email].filter(Boolean).join(" · ")}</p>
                      {update ? (
                        <div className="flex flex-wrap gap-2">
                          <Button variant="ghost" onClick={() => setEditing(g.id)}>
                            Edit<span className="sr-only"> {g.full_name}</span>
                          </Button>
                          <Button
                            variant="ghost"
                            onClick={async () =>
                              done(
                                await run(
                                  () =>
                                    api.DELETE("/api/v1/staff/students/{id}/guardians/{item}", {
                                      params: { path: { id, item: g.id } },
                                    }),
                                  "Removed",
                                ),
                              )
                            }
                          >
                            Remove<span className="sr-only"> {g.full_name}</span>
                          </Button>
                        </div>
                      ) : null}
                    </div>
                  )}
                </li>
              ))}
            </ul>
            {update && !adding ? (
              <Button variant="outline" className="self-start" onClick={() => setAdding(true)}>
                Add a contact
              </Button>
            ) : null}
            {update && adding ? (
              <Card>
                <h3 className="text-lg font-semibold">Add a contact</h3>
                <GuardianForm
                  idPrefix="add"
                  initial={blankGuardian}
                  submitLabel="Add contact"
                  errors={errors}
                  busy={busy}
                  onCancel={() => setAdding(false)}
                  onSubmit={async (body) => {
                    const s = await run(
                      () => api.POST("/api/v1/staff/students/{id}/guardians", { ...path, body }),
                      "Contact added",
                      "add.",
                    );
                    if (done(s)) setAdding(false);
                  }}
                />
              </Card>
            ) : null}
          </section>
        </div>

        <div className="flex min-w-0 flex-col gap-6">
          {update && student.status !== "graduated" ? (
            <Card>
              <h2 className="text-xl font-semibold">Change status</h2>
              <form
                noValidate
                className="flex flex-col gap-3"
                onSubmit={async (e) => {
                  e.preventDefault();
                  if (!reason.trim()) {
                    setErrors({ "status.reason": "Write why the status is changing." });
                    return;
                  }
                  const s = await run(
                    () =>
                      api.POST("/api/v1/staff/students/{id}/status", {
                        ...path,
                        body: { status: nextStatus, reason },
                      }),
                    "Status changed",
                    "status.",
                  );
                  if (done(s)) setReason("");
                }}
              >
                <Select
                  id="new-status"
                  name="status"
                  label="New status"
                  required
                  value={nextStatus}
                  error={errors["status.status"]}
                  onChange={(e) => setStatus(e.target.value as StudentStatus)}
                >
                  {studentStatuses
                    .filter((s) => s !== student.status)
                    .map((s) => (
                      <option key={s} value={s}>
                        {studentStatus[s].label}
                      </option>
                    ))}
                </Select>
                <Textarea
                  id="status-reason"
                  name="reason"
                  label="Reason"
                  required
                  rows={3}
                  maxLength={2000}
                  value={reason}
                  error={errors["status.reason"]}
                  hint="Kept in the student's history. Graduated cannot be changed later."
                  onChange={(e) => setReason(e.target.value)}
                />
                <Button type="submit" variant="secondary" loading={busy} className="self-start">
                  Change status
                </Button>
              </form>
            </Card>
          ) : null}

          {update && ownCohorts.length > 1 ? (
            <Card>
              <h2 className="text-xl font-semibold">Cohort</h2>
              <form
                className="flex flex-col gap-3"
                onSubmit={async (e) => {
                  e.preventDefault();
                  const cohortId = String(new FormData(e.currentTarget).get("cohort") ?? "");
                  done(
                    await run(
                      () =>
                        api.POST("/api/v1/staff/students/{id}/cohort", {
                          ...path,
                          body: { cohort_id: cohortId },
                        }),
                      "Cohort changed",
                    ),
                  );
                }}
              >
                <Select
                  id="student-cohort-move"
                  name="cohort"
                  label="Move to cohort"
                  required
                  defaultValue={student.cohort_id ?? ""}
                  key={student.cohort_id ?? "none"}
                >
                  {ownCohorts.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </Select>
                <Button type="submit" variant="outline" loading={busy} className="self-start">
                  Move
                </Button>
              </form>
            </Card>
          ) : null}

          <section aria-labelledby="student-history-title" className="flex flex-col gap-2">
            <h2 id="student-history-title" className="text-xl font-semibold">
              Status history
            </h2>
            <ol className="flex flex-col gap-3 border-l-2 border-stone pl-4 text-sm">
              {[...student.history].reverse().map((h, i) => (
                <li key={i} className="flex flex-col">
                  <span className="font-semibold">{studentStatus[h.to].label}</span>
                  <span className="text-ink-muted">
                    {formatNairobi(h.at)} · {h.changed_by_name ?? "the system"}
                  </span>
                  {h.reason ? <span className="break-words">{h.reason}</span> : null}
                </li>
              ))}
            </ol>
          </section>
        </div>
      </div>
    </div>
  );
}
