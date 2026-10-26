"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Checkbox, Input, Select } from "@/components/ui/field";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { Table } from "@/components/ui/table";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { sortTimetable, weekdayName, weekdays } from "@/lib/staff/academics";
import { formatDate } from "@/lib/staff/datetime";
import { useAction } from "./use-action";

type Offering = components["schemas"]["Offering"];
type OfferingItem = components["schemas"]["OfferingItem"];
type Term = components["schemas"]["Term"];
type Unit = components["schemas"]["Unit"];
type Cohort = components["schemas"]["Cohort"];
type Person = components["schemas"]["Person"];
type Entry = components["schemas"]["TimetableEntry"];

/** "Tuesday 08:30 to 10:00, Lab 1" or "Exam 3 March 2027, 09:00 to 11:00". */
export function describeEntry(e: Entry): string {
  const when = e.kind === "class" ? weekdayName(e.weekday) : `Exam ${formatDate(e.on_date)},`;
  return `${when} ${e.starts_at} to ${e.ends_at}${e.room ? `, ${e.room}` : ""}`;
}

function useList<T>(load: () => Promise<{ data?: { items: T[] } }>, deps: unknown[] = []) {
  const [items, setItems] = useState<T[] | null>(null);
  useEffect(() => {
    let active = true;
    void load().then(({ data }) => {
      if (active) setItems(data?.items ?? []);
    });
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- callers pass what the load depends on
  }, deps);
  return [items, setItems] as const;
}

// ---------- list ----------

export function OfferingList() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const termParam = params.get("term") ?? "";
  const [terms] = useList<Term>(() => api.GET("/api/v1/staff/terms"));
  const [units] = useList<Unit>(() => api.GET("/api/v1/staff/units"));
  const [offerings, setOfferings] = useList<OfferingItem>(
    () => api.GET("/api/v1/staff/offerings", { params: { query: { term: termParam || undefined } } }),
    [termParam],
  );
  const { busy, errors, run } = useAction();
  const [newTerm, setNewTerm] = useState("");
  const [newUnit, setNewUnit] = useState("");
  const term = termParam || "";

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Classes</h1>
        <p className="max-w-prose text-ink-muted">
          A class is a unit taught in a term: who teaches it, when, and who is registered.
        </p>
      </div>

      <form
        noValidate
        className="grid grid-cols-1 items-end gap-4 sm:grid-cols-[1fr_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          const v = String(new FormData(e.currentTarget).get("term") ?? "");
          router.push(v ? `${pathname}?term=${v}` : pathname);
        }}
      >
        <Select id="offering-term" name="term" label="Term" defaultValue={term} key={term} required>
          <option value="">All terms</option>
          {(terms ?? []).map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </Select>
        <Button type="submit" variant="secondary">
          Show
        </Button>
      </form>

      {offerings === null ? (
        <Loading label="Loading classes">
          <Skeleton className="h-40 w-full" />
        </Loading>
      ) : offerings.length === 0 ? (
        <EmptyState title="No classes yet" description="Add a class below: choose a term and a unit." />
      ) : (
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
          {offerings.map((o) => (
            <li key={o.id}>
              <Link
                href={`/staff/offerings/${o.id}`}
                className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:gap-4"
              >
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="font-semibold text-royal">
                    <span className="font-mono">{o.unit_code}</span> {o.unit_title}
                  </span>
                  <span className="text-sm text-ink-muted">{o.term_name}</span>
                </span>
                <span className="text-sm sm:w-56">{o.trainer_names || "No trainer yet"}</span>
                <span className="text-sm text-ink-muted sm:w-28">
                  {o.registered === 1 ? "1 student" : `${o.registered} students`}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <Card>
        <h2 className="text-xl font-semibold">Add a class</h2>
        <form
          noValidate
          className="grid grid-cols-1 items-end gap-4 sm:grid-cols-[1fr_2fr_auto]"
          onSubmit={async (e) => {
            e.preventDefault();
            const created = await run(
              () => api.POST("/api/v1/staff/offerings", { body: { term_id: newTerm, unit_id: newUnit } }),
              "Class added",
            );
            if (created) {
              const { data } = await api.GET("/api/v1/staff/offerings", {
                params: { query: { term: termParam || undefined } },
              });
              setOfferings(data?.items ?? []);
            }
          }}
        >
          <Select
            id="new-offering-term"
            name="term_id"
            label="Term"
            required
            value={newTerm}
            error={errors.term_id}
            onChange={(e) => setNewTerm(e.target.value)}
          >
            <option value="">Choose a term</option>
            {(terms ?? []).map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
          <Select
            id="new-offering-unit"
            name="unit_id"
            label="Unit"
            required
            value={newUnit}
            error={errors.unit_id}
            onChange={(e) => setNewUnit(e.target.value)}
          >
            <option value="">Choose a unit</option>
            {(units ?? [])
              .filter((u) => u.active)
              .map((u) => (
                <option key={u.id} value={u.id}>
                  {u.code} {u.title}
                </option>
              ))}
          </Select>
          <Button type="submit" loading={busy}>
            Add class
          </Button>
        </form>
      </Card>
    </div>
  );
}

// ---------- one class ----------

export function OfferingView({ id }: { id: string }) {
  const [offering, setOffering] = useState<Offering | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [trainers] = useList<Person>(() => api.GET("/api/v1/staff/trainers"));
  const [cohorts] = useList<Cohort>(() => api.GET("/api/v1/staff/cohorts"));
  const { busy, errors, run } = useAction();
  const [chosen, setChosen] = useState<string[] | null>(null);
  const [entry, setEntry] = useState({
    kind: "class",
    weekday: "1",
    on_date: "",
    starts_at: "",
    ends_at: "",
    room: "",
  });
  const [cohort, setCohort] = useState("");
  const [result, setResult] = useState<string | null>(null);

  useEffect(() => {
    api.GET("/api/v1/staff/offerings/{id}", { params: { path: { id } } }).then(({ data, error: err }) => {
      if (data) setOffering(data);
      else setError(errorMessage(err));
    });
  }, [id]);

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the class">
        {error}
      </Alert>
    );
  }
  if (!offering) {
    return (
      <Loading label="Loading the class">
        <SkeletonText lines={10} />
      </Loading>
    );
  }
  const path = { params: { path: { id } } };
  const done = (o: Offering | null) => {
    if (o) setOffering(o);
    return o !== null;
  };
  const assigned = chosen ?? offering.trainers.map((t) => t.id);
  const registered = offering.class_list.filter((c) => c.status === "registered");
  const title = `${offering.unit_code} ${offering.unit_title}`;

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "Classes", href: "/staff/offerings" }, { label: title }]} />
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold break-words">{title}</h1>
        <p className="text-ink-muted">
          {offering.term_name}: {formatDate(offering.starts_on)} to {formatDate(offering.ends_on)}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="flex min-w-0 flex-col gap-6">
          <section aria-labelledby="class-list-title" className="flex flex-col gap-3">
            <h2 id="class-list-title" className="text-xl font-semibold">
              Class list ({registered.length})
            </h2>
            <Table
              caption="Class list"
              captionHidden
              rows={offering.class_list}
              rowKey={(c) => c.registration_id}
              empty="Nobody is registered yet. Register a cohort below."
              columns={[
                {
                  key: "number",
                  header: "Student number",
                  cell: (c) => <span className="font-mono">{c.student_number}</span>,
                },
                {
                  key: "name",
                  header: "Name",
                  cell: (c) => (
                    <Link href={`/staff/students/${c.student_id}`} className="text-royal underline">
                      {c.name}
                    </Link>
                  ),
                },
                { key: "cohort", header: "Cohort", cell: (c) => c.cohort_name ?? "None" },
                {
                  key: "status",
                  header: "Status",
                  cell: (c) =>
                    c.status === "registered" ? (
                      <Button
                        variant="ghost"
                        onClick={async () =>
                          done(
                            await run(
                              () =>
                                api.POST("/api/v1/staff/offerings/{id}/registrations/{item}/drop", {
                                  params: { path: { id, item: c.registration_id } },
                                }),
                              `${c.name} dropped`,
                            ),
                          )
                        }
                      >
                        Drop<span className="sr-only"> {c.name}</span>
                      </Button>
                    ) : (
                      "Dropped"
                    ),
                },
              ]}
            />
            <form
              noValidate
              className="grid grid-cols-1 items-end gap-4 sm:grid-cols-[1fr_auto]"
              onSubmit={async (e) => {
                e.preventDefault();
                setResult(null);
                const res = await run(
                  () =>
                    api.POST("/api/v1/staff/offerings/{id}/registrations", {
                      ...path,
                      body: { cohort_id: cohort || null },
                    }),
                  "",
                  "register.",
                );
                if (res) {
                  done(res.offering);
                  const r = res.result;
                  setResult(
                    `${r.registered} registered, ${r.already_registered} already on the list, ${r.not_eligible} not eligible (not active, or the unit is not in their programme).`,
                  );
                }
              }}
            >
              <Select
                id="register-cohort"
                name="cohort"
                label="Register a cohort"
                required
                value={cohort}
                error={
                  errors["register.cohort_id"] ?? errors["register.student_ids"] ?? errors["register.term"]
                }
                onChange={(e) => setCohort(e.target.value)}
              >
                <option value="">Choose a cohort</option>
                {(cohorts ?? []).map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.active_students} active)
                  </option>
                ))}
              </Select>
              <Button type="submit" variant="secondary" loading={busy}>
                Register
              </Button>
            </form>
            <div role="status">
              {result ? (
                <Alert tone="success" title="Registration done">
                  {result}
                </Alert>
              ) : null}
            </div>
          </section>

          <section aria-labelledby="timetable-title" className="flex flex-col gap-3">
            <h2 id="timetable-title" className="text-xl font-semibold">
              Timetable
            </h2>
            {offering.timetable.length === 0 ? <p>No classes or exams scheduled yet.</p> : null}
            <ul className="flex flex-col gap-2">
              {sortTimetable(offering.timetable).map((e) => (
                <li
                  key={e.id}
                  className="flex flex-col gap-1 rounded-control border border-stone bg-white px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
                >
                  <span>{describeEntry(e)}</span>
                  <Button
                    variant="ghost"
                    onClick={async () =>
                      done(
                        await run(
                          () =>
                            api.DELETE("/api/v1/staff/offerings/{id}/timetable/{item}", {
                              params: { path: { id, item: e.id } },
                            }),
                          "Removed from the timetable",
                        ),
                      )
                    }
                  >
                    Remove<span className="sr-only"> {describeEntry(e)}</span>
                  </Button>
                </li>
              ))}
            </ul>
            <Card>
              <h3 className="text-lg font-semibold">Add to the timetable</h3>
              <form
                noValidate
                className="grid grid-cols-1 gap-4 sm:grid-cols-2"
                onSubmit={async (e) => {
                  e.preventDefault();
                  const ok = done(
                    await run(
                      () =>
                        api.POST("/api/v1/staff/offerings/{id}/timetable", {
                          ...path,
                          body: {
                            kind: entry.kind as "class" | "exam",
                            weekday: entry.kind === "class" ? Number(entry.weekday) : null,
                            on_date: entry.kind === "exam" ? entry.on_date : "",
                            starts_at: entry.starts_at,
                            ends_at: entry.ends_at,
                            room: entry.room,
                          },
                        }),
                      "Added to the timetable",
                      "tt.",
                    ),
                  );
                  if (ok) setEntry({ ...entry, starts_at: "", ends_at: "", room: "" });
                }}
              >
                <Select
                  id="tt-kind"
                  name="kind"
                  label="What"
                  required
                  value={entry.kind}
                  onChange={(e) => setEntry({ ...entry, kind: e.target.value })}
                >
                  <option value="class">Weekly class</option>
                  <option value="exam">Exam</option>
                </Select>
                {entry.kind === "class" ? (
                  <Select
                    id="tt-weekday"
                    name="weekday"
                    label="Day"
                    required
                    value={entry.weekday}
                    error={errors["tt.weekday"]}
                    onChange={(e) => setEntry({ ...entry, weekday: e.target.value })}
                  >
                    {weekdays.map((d, i) => (
                      <option key={d} value={String(i + 1)}>
                        {d}
                      </option>
                    ))}
                  </Select>
                ) : (
                  <Input
                    id="tt-date"
                    name="on_date"
                    type="date"
                    label="Date"
                    required
                    min={offering.starts_on}
                    max={offering.ends_on}
                    value={entry.on_date}
                    error={errors["tt.on_date"]}
                    onChange={(e) => setEntry({ ...entry, on_date: e.target.value })}
                  />
                )}
                <Input
                  id="tt-starts"
                  name="starts_at"
                  type="time"
                  label="Starts"
                  required
                  value={entry.starts_at}
                  error={errors["tt.starts_at"]}
                  onChange={(e) => setEntry({ ...entry, starts_at: e.target.value })}
                />
                <Input
                  id="tt-ends"
                  name="ends_at"
                  type="time"
                  label="Ends"
                  required
                  value={entry.ends_at}
                  error={errors["tt.ends_at"]}
                  onChange={(e) => setEntry({ ...entry, ends_at: e.target.value })}
                />
                <Input
                  id="tt-room"
                  name="room"
                  label="Room"
                  maxLength={60}
                  value={entry.room}
                  error={errors["tt.room"]}
                  onChange={(e) => setEntry({ ...entry, room: e.target.value })}
                />
                <div className="flex items-end">
                  <Button type="submit" variant="secondary" loading={busy}>
                    Add
                  </Button>
                </div>
              </form>
            </Card>
          </section>
        </div>

        <div className="flex min-w-0 flex-col gap-6">
          <Card>
            <h2 className="text-xl font-semibold">Trainers</h2>
            <p>{offering.trainers.map((t) => t.full_name).join(", ") || "No trainer yet."}</p>
            <form
              className="flex flex-col gap-1"
              onSubmit={async (e) => {
                e.preventDefault();
                if (
                  done(
                    await run(
                      () =>
                        api.PUT("/api/v1/staff/offerings/{id}/trainers", {
                          ...path,
                          body: { trainer_ids: assigned },
                        }),
                      "Trainers saved",
                    ),
                  )
                )
                  setChosen(null);
              }}
            >
              <fieldset className="flex flex-col">
                <legend className="font-semibold">Who teaches this class</legend>
                {(trainers ?? []).length === 0 ? (
                  <p className="text-sm text-ink-muted">No one with the trainer permission yet.</p>
                ) : null}
                {(trainers ?? []).map((t) => (
                  <Checkbox
                    key={t.id}
                    id={`trainer-${t.id}`}
                    name="trainer"
                    label={t.full_name}
                    checked={assigned.includes(t.id)}
                    onChange={(e) =>
                      setChosen(e.target.checked ? [...assigned, t.id] : assigned.filter((x) => x !== t.id))
                    }
                  />
                ))}
              </fieldset>
              <Button
                type="submit"
                variant="secondary"
                loading={busy}
                className="self-start"
                disabled={chosen === null}
              >
                Save trainers
              </Button>
            </form>
          </Card>
        </div>
      </div>
    </div>
  );
}
