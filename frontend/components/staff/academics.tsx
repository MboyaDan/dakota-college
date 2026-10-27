"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox, Input, Select, Textarea } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { Table } from "@/components/ui/table";
import { Tabs } from "@/components/ui/tabs";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDate } from "@/lib/staff/datetime";
import { ProgrammeAssessment } from "./assessment";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Units and terms (academics.manage): the academic catalogue in four tabs.

type Term = components["schemas"]["Term"];
type TermInput = components["schemas"]["TermInput"];
type Unit = components["schemas"]["Unit"];
type ProgrammeUnit = components["schemas"]["ProgrammeUnit"];
type Cohort = components["schemas"]["Cohort"];
type Option = { id: string; label: string };

function useOptions(kind: "programmes" | "schools") {
  const [items, setItems] = useState<Option[]>([]);
  useEffect(() => {
    api.GET("/api/v1/staff/content/{kind}", { params: { path: { kind } } }).then(({ data }) => {
      if (data) setItems(data.items.map((i) => ({ id: i.id, label: i.label })));
    });
  }, [kind]);
  return items;
}

export function AcademicsCatalogue() {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Units and terms</h1>
        <p className="max-w-prose text-ink-muted">
          The terms of the year, the units the college teaches, which units make up each programme, how each
          programme is assessed, and cohorts.
        </p>
      </div>
      <Tabs
        label="Academic catalogue"
        items={[
          { id: "terms", label: "Terms", content: <Terms /> },
          { id: "units", label: "Units", content: <Units /> },
          { id: "programme-units", label: "Programme units", content: <ProgrammeUnits /> },
          { id: "assessment", label: "Assessment", content: <Assessment /> },
          { id: "cohorts", label: "Cohorts", content: <Cohorts /> },
        ]}
      />
    </div>
  );
}

// ---------- terms ----------

const blankTerm: TermInput = {
  name: "",
  starts_on: "",
  ends_on: "",
  registration_opens_on: "",
  registration_closes_on: "",
};

function TermForm({
  initial,
  onSave,
  busy,
  errors,
  prefix,
  submitLabel,
}: {
  initial: TermInput;
  onSave: (t: TermInput) => void;
  busy: boolean;
  errors: Record<string, string>;
  prefix: string;
  submitLabel: string;
}) {
  const [t, setT] = useState(initial);
  const field = (k: keyof TermInput, label: string, type = "date", required = true, hint?: string) => (
    <Input
      id={`${prefix}-${k}`}
      name={k}
      type={type}
      label={label}
      required={required}
      hint={hint}
      value={t[k] ?? ""}
      error={errors[`${prefix}.${k}`]}
      onChange={(e) => setT({ ...t, [k]: e.target.value })}
    />
  );
  return (
    <form
      noValidate
      className="grid grid-cols-1 gap-4 sm:grid-cols-2"
      onSubmit={(e) => {
        e.preventDefault();
        onSave(t);
      }}
    >
      <div className="sm:col-span-2">{field("name", "Name", "text", true, "For example: Term 1, 2027.")}</div>
      {field("starts_on", "Starts")}
      {field("ends_on", "Ends")}
      {field("registration_opens_on", "Students may register from", "date", false)}
      {field("registration_closes_on", "Until", "date", false)}
      <div>
        <Button type="submit" variant="secondary" loading={busy}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}

function Terms() {
  const { can } = useSession();
  const allSchools = can("schools.all");
  const { busy, errors, run } = useAction();
  const [terms, setTerms] = useState<Term[] | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const load = () =>
    api.GET("/api/v1/staff/terms").then(({ data }) => {
      setTerms(data?.items ?? []);
    });
  useEffect(() => {
    void load();
  }, []);

  return (
    <div className="flex flex-col gap-4">
      {!allSchools ? (
        <Alert tone="info" title="Terms apply to every school">
          Only the registrar&apos;s office can add or change them.
        </Alert>
      ) : null}
      {terms === null ? (
        <Loading label="Loading terms">
          <Skeleton className="h-32 w-full" />
        </Loading>
      ) : (
        <ul className="flex flex-col gap-2">
          {terms.length === 0 ? <li>No terms yet.</li> : null}
          {terms.map((t) => (
            <li key={t.id} className="rounded-card border border-stone bg-white p-4">
              {editing === t.id ? (
                <TermForm
                  prefix={`term-${t.id.slice(0, 8)}`}
                  initial={{
                    name: t.name,
                    starts_on: t.starts_on,
                    ends_on: t.ends_on,
                    registration_opens_on: t.registration_opens_on ?? "",
                    registration_closes_on: t.registration_closes_on ?? "",
                  }}
                  submitLabel="Save term"
                  busy={busy}
                  errors={errors}
                  onSave={async (body) => {
                    if (
                      await run(
                        () => api.PUT("/api/v1/staff/terms/{id}", { params: { path: { id: t.id } }, body }),
                        "Term saved",
                        `term-${t.id.slice(0, 8)}.`,
                      )
                    ) {
                      setEditing(null);
                      await load();
                    }
                  }}
                />
              ) : (
                <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                  <span>
                    <span className="font-semibold">{t.name}</span>{" "}
                    <span className="text-ink-muted">
                      {formatDate(t.starts_on)} to {formatDate(t.ends_on)}
                      {t.registration_opens_on
                        ? ` · students register ${formatDate(t.registration_opens_on)} to ${formatDate(t.registration_closes_on)}`
                        : ""}
                    </span>
                  </span>
                  {allSchools ? (
                    <Button variant="ghost" onClick={() => setEditing(t.id)}>
                      Edit<span className="sr-only"> {t.name}</span>
                    </Button>
                  ) : null}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      {allSchools ? (
        <Card>
          <h2 className="text-xl font-semibold">Add a term</h2>
          <TermForm
            key={formKey}
            prefix="new-term"
            initial={blankTerm}
            submitLabel="Add term"
            busy={busy}
            errors={errors}
            onSave={async (body) => {
              if (await run(() => api.POST("/api/v1/staff/terms", { body }), "Term added", "new-term.")) {
                setFormKey((k) => k + 1);
                await load();
              }
            }}
          />
        </Card>
      ) : null}
    </div>
  );
}

// ---------- units ----------

function Units() {
  const schools = useOptions("schools");
  const { busy, errors, run } = useAction();
  const [units, setUnits] = useState<Unit[] | null>(null);
  const [draft, setDraft] = useState({ school_id: "", code: "", title: "", description: "", hours: "" });
  const load = () =>
    api.GET("/api/v1/staff/units").then(({ data }) => {
      setUnits(data?.items ?? []);
    });
  useEffect(() => {
    void load();
  }, []);

  return (
    <div className="flex flex-col gap-4">
      {units === null ? (
        <Loading label="Loading units">
          <Skeleton className="h-32 w-full" />
        </Loading>
      ) : (
        <Table
          caption="Units"
          captionHidden
          rows={units}
          rowKey={(u) => u.id}
          empty="No units yet."
          columns={[
            {
              key: "code",
              header: "Code",
              cell: (u) => <span className="font-mono whitespace-nowrap">{u.code}</span>,
            },
            { key: "title", header: "Title", cell: (u) => u.title },
            { key: "school", header: "School", cell: (u) => u.school_name },
            { key: "hours", header: "Hours", numeric: true, cell: (u) => u.hours ?? "" },
            {
              key: "active",
              header: "Taught",
              cell: (u) => (
                <Button
                  variant="ghost"
                  onClick={async () => {
                    if (
                      await run(
                        () =>
                          api.PUT("/api/v1/staff/units/{id}", {
                            params: { path: { id: u.id } },
                            body: {
                              code: u.code,
                              title: u.title,
                              description: u.description,
                              hours: u.hours,
                              active: !u.active,
                            },
                          }),
                        u.active ? `${u.code} no longer taught` : `${u.code} taught again`,
                      )
                    )
                      await load();
                  }}
                >
                  {u.active ? "Yes, stop" : "No, start"}
                  <span className="sr-only"> {u.code}</span>
                </Button>
              ),
            },
          ]}
        />
      )}
      <Card>
        <h2 className="text-xl font-semibold">Add a unit</h2>
        <form
          noValidate
          className="grid grid-cols-1 gap-4 sm:grid-cols-2"
          onSubmit={async (e) => {
            e.preventDefault();
            const created = await run(
              () =>
                api.POST("/api/v1/staff/units", {
                  body: {
                    school_id: draft.school_id,
                    code: draft.code,
                    title: draft.title,
                    description: draft.description,
                    hours: draft.hours ? Number(draft.hours) : null,
                  },
                }),
              "Unit added",
              "unit.",
            );
            if (created) {
              setDraft({ ...draft, code: "", title: "", description: "", hours: "" });
              await load();
            }
          }}
        >
          <Select
            id="unit-school"
            name="school_id"
            label="School"
            required
            value={draft.school_id}
            error={errors["unit.school_id"]}
            onChange={(e) => setDraft({ ...draft, school_id: e.target.value })}
          >
            <option value="">Choose a school</option>
            {schools.map((s) => (
              <option key={s.id} value={s.id}>
                {s.label}
              </option>
            ))}
          </Select>
          <Input
            id="unit-code"
            name="code"
            label="Code"
            required
            maxLength={20}
            hint="Like ICT 101."
            value={draft.code}
            error={errors["unit.code"]}
            onChange={(e) => setDraft({ ...draft, code: e.target.value })}
          />
          <Input
            id="unit-title"
            name="title"
            label="Title"
            required
            maxLength={160}
            value={draft.title}
            error={errors["unit.title"]}
            onChange={(e) => setDraft({ ...draft, title: e.target.value })}
          />
          <Input
            id="unit-hours"
            name="hours"
            type="number"
            inputMode="numeric"
            min={1}
            max={2000}
            label="Hours"
            value={draft.hours}
            error={errors["unit.hours"]}
            onChange={(e) => setDraft({ ...draft, hours: e.target.value })}
          />
          <div className="sm:col-span-2">
            <Textarea
              id="unit-description"
              name="description"
              label="Description"
              rows={2}
              maxLength={2000}
              value={draft.description}
              error={errors["unit.description"]}
              onChange={(e) => setDraft({ ...draft, description: e.target.value })}
            />
          </div>
          <div>
            <Button type="submit" variant="secondary" loading={busy}>
              Add unit
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}

// ---------- programme units ----------

type Row = { unit_id: string; stage: number; is_core: boolean };

function ProgrammeUnits() {
  const programmes = useOptions("programmes");
  const { busy, errors, run } = useAction();
  const [programme, setProgramme] = useState("");
  const [units, setUnits] = useState<Unit[]>([]);
  const [rows, setRows] = useState<Row[] | null>(null);
  const [adding, setAdding] = useState("");

  useEffect(() => {
    api.GET("/api/v1/staff/units").then(({ data }) => setUnits(data?.items ?? []));
  }, []);
  useEffect(() => {
    if (!programme) return;
    api.GET("/api/v1/staff/programmes/{id}/units", { params: { path: { id: programme } } }).then(({ data }) =>
      setRows(
        (data?.items ?? []).map((u: ProgrammeUnit) => ({
          unit_id: u.unit_id,
          stage: u.stage,
          is_core: u.is_core,
        })),
      ),
    );
  }, [programme]);

  const unit = (id: string) => units.find((u) => u.id === id);

  return (
    <div className="flex flex-col gap-4">
      <Select
        id="pu-programme"
        name="programme"
        label="Programme"
        required
        value={programme}
        onChange={(e) => {
          setRows(null);
          setProgramme(e.target.value);
        }}
      >
        <option value="">Choose a programme</option>
        {programmes.map((p) => (
          <option key={p.id} value={p.id}>
            {p.label}
          </option>
        ))}
      </Select>
      {programme && rows === null ? (
        <Loading label="Loading the programme's units">
          <Skeleton className="h-24 w-full" />
        </Loading>
      ) : null}
      {programme && rows ? (
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            await run(
              () =>
                api.PUT("/api/v1/staff/programmes/{id}/units", {
                  params: { path: { id: programme } },
                  body: { items: rows },
                }),
              "Programme units saved",
              "pu.",
            );
          }}
        >
          {errors["pu.items"] ? (
            <Alert tone="danger" title="Not saved">
              {errors["pu.items"]}
            </Alert>
          ) : null}
          {rows.length === 0 ? <p>This programme has no units yet.</p> : null}
          <ul className="flex flex-col gap-2">
            {rows.map((r, i) => (
              <li
                key={r.unit_id}
                className="grid grid-cols-1 items-end gap-3 rounded-card border border-stone bg-white p-3 sm:grid-cols-[2fr_8rem_auto_auto]"
              >
                <span className="self-center">
                  <span className="font-mono">{unit(r.unit_id)?.code}</span> {unit(r.unit_id)?.title}
                </span>
                <Input
                  id={`pu-stage-${i}`}
                  name="stage"
                  type="number"
                  min={1}
                  max={10}
                  label="Year or level"
                  required
                  value={String(r.stage)}
                  onChange={(e) =>
                    setRows(rows.map((x, j) => (j === i ? { ...x, stage: Number(e.target.value) } : x)))
                  }
                />
                <Checkbox
                  id={`pu-core-${i}`}
                  name="is_core"
                  label="Core"
                  checked={r.is_core}
                  onChange={(e) =>
                    setRows(rows.map((x, j) => (j === i ? { ...x, is_core: e.target.checked } : x)))
                  }
                />
                <Button variant="ghost" onClick={() => setRows(rows.filter((_, j) => j !== i))}>
                  Remove<span className="sr-only"> {unit(r.unit_id)?.code}</span>
                </Button>
              </li>
            ))}
          </ul>
          <div className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[1fr_auto]">
            <Select
              id="pu-add"
              name="add_unit"
              label="Add a unit"
              required
              value={adding}
              onChange={(e) => setAdding(e.target.value)}
            >
              <option value="">Choose a unit</option>
              {units
                .filter((u) => u.active && !rows.some((r) => r.unit_id === u.id))
                .map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.code} {u.title}
                  </option>
                ))}
            </Select>
            <Button
              variant="outline"
              disabled={!adding}
              onClick={() => {
                setRows([...rows, { unit_id: adding, stage: 1, is_core: true }]);
                setAdding("");
              }}
            >
              Add to the list
            </Button>
          </div>
          <Button type="submit" variant="secondary" loading={busy} className="self-start">
            Save programme units
          </Button>
        </form>
      ) : null}
    </div>
  );
}

// ---------- assessment ----------

function Assessment() {
  return <ProgrammeAssessment programmes={useOptions("programmes")} />;
}

// ---------- cohorts ----------

function Cohorts() {
  const programmes = useOptions("programmes");
  const { busy, errors, run } = useAction();
  const [cohorts, setCohorts] = useState<Cohort[] | null>(null);
  const [draft, setDraft] = useState({ programme_id: "", name: "" });
  const load = () =>
    api.GET("/api/v1/staff/cohorts").then(({ data }) => {
      setCohorts(data?.items ?? []);
    });
  useEffect(() => {
    void load();
  }, []);

  return (
    <div className="flex flex-col gap-4">
      <p className="max-w-prose text-ink-muted">
        Enrolment puts each new student in the cohort of their programme&apos;s intake. Add other cohorts
        here, for example a group repeating a year.
      </p>
      {cohorts === null ? (
        <Loading label="Loading cohorts">
          <Skeleton className="h-32 w-full" />
        </Loading>
      ) : (
        <Table
          caption="Cohorts"
          captionHidden
          rows={cohorts}
          rowKey={(c) => c.id}
          empty="No cohorts yet."
          columns={[
            { key: "name", header: "Cohort", cell: (c) => c.name },
            { key: "programme", header: "Programme", cell: (c) => c.programme_title },
            { key: "active", header: "Active students", numeric: true, cell: (c) => c.active_students },
            { key: "all", header: "All students", numeric: true, cell: (c) => c.students },
          ]}
        />
      )}
      <Card>
        <h2 className="text-xl font-semibold">Add a cohort</h2>
        <form
          noValidate
          className="grid grid-cols-1 items-end gap-4 sm:grid-cols-[2fr_2fr_auto]"
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run(() => api.POST("/api/v1/staff/cohorts", { body: draft }), "Cohort added", "cohort.")
            ) {
              setDraft({ ...draft, name: "" });
              await load();
            }
          }}
        >
          <Select
            id="cohort-programme"
            name="programme_id"
            label="Programme"
            required
            value={draft.programme_id}
            error={errors["cohort.programme_id"]}
            onChange={(e) => setDraft({ ...draft, programme_id: e.target.value })}
          >
            <option value="">Choose a programme</option>
            {programmes.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </Select>
          <Input
            id="cohort-name"
            name="name"
            label="Name"
            required
            maxLength={120}
            value={draft.name}
            error={errors["cohort.name"]}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
          />
          <Button type="submit" variant="secondary" loading={busy}>
            Add cohort
          </Button>
        </form>
      </Card>
    </div>
  );
}
