"use client";

import Link from "next/link";
import { useState, type FormEvent, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox, Input, Select } from "@/components/ui/field";
import { Plus, Trash2 } from "@/components/icons";
import type { components } from "@/lib/api/schema";
import { useAutosave } from "@/lib/apply/autosave";
import { counties, kcseGrades } from "@/lib/apply/kenya";
import { formatDate } from "@/lib/staff/datetime";
import { kindLabels } from "./documents";
import { SaveStatus, StepButtons, stepTitles, type StepKey, type StepProps } from "./step-kit";
import { SubmitPanel } from "./submit-panel";

type Personal = components["schemas"]["ApplicationPersonal"];
type Guardian = components["schemas"]["ApplicationGuardian"];
type Education = components["schemas"]["ApplicationEducation"];
type Sponsor = components["schemas"]["ApplicationSponsor"];

const fieldLabels: Record<string, string> = {
  intake_id: "Intake",
  first_name: "First name",
  middle_name: "Middle name",
  last_name: "Last name",
  date_of_birth: "Date of birth",
  identity_document_type: "Identity document",
  identity_document: "Document number",
  nationality: "Nationality",
  county: "County",
  phone: "Phone number",
  full_name: "Name",
  relationship: "Relationship",
  email: "Email address",
  consent_given: "Agreement",
  type: "Who will pay",
  name: "Name",
};

function labelFor(field: string): string {
  const last = field.split(".").at(-1) ?? field;
  const entry = /^education\.(\d+)/.exec(field);
  const base = fieldLabels[last] ?? last.replace(/_/g, " ");
  return entry ? `School ${Number(entry[1]) + 1}: ${base.toLowerCase()}` : base;
}

/** Lists the refused answers at the top, each linking to its field. */
function ErrorSummary({ fields, id }: { fields: Record<string, string>; id: string }) {
  const entries = Object.entries(fields);
  if (entries.length === 0) return null;
  return (
    <div id={id} tabIndex={-1} className="focus:outline-none">
      <Alert tone="danger" title="Check these answers" live>
        <ul className="list-disc pl-5">
          {entries.map(([f, msg]) => (
            <li key={f}>
              <a href={`#field-${f}`} className="underline">
                {labelFor(f)}: {msg}
              </a>
            </li>
          ))}
        </ul>
      </Alert>
    </div>
  );
}

/**
 * The form around each step: autosave status, refused answers, and "Save
 * and continue", which saves at once and moves on only if it worked.
 */
function StepForm({
  editable,
  state,
  fields,
  flush,
  next,
  back,
  children,
}: Pick<StepProps, "editable" | "next" | "back"> & {
  state: Parameters<typeof SaveStatus>[0]["state"];
  fields: Record<string, string>;
  flush: () => Promise<boolean>;
  children: ReactNode;
}) {
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!editable) {
      next();
      return;
    }
    setBusy(true);
    const ok = await flush();
    setBusy(false);
    if (ok) next();
    else document.getElementById("step-errors")?.focus();
  }
  return (
    <form method="post" noValidate onSubmit={submit} className="flex flex-col gap-5">
      <ErrorSummary fields={fields} id="step-errors" />
      <fieldset disabled={!editable} className="flex min-w-0 flex-col gap-5">
        {children}
      </fieldset>
      {editable ? <SaveStatus state={state} retry={() => void flush()} /> : null}
      <StepButtons back={back} busy={busy} label={editable ? "Save and continue" : "Continue"} />
    </form>
  );
}

// ---------- 1. programme and intake ----------

export function ChoiceStep({ detail, editable, save, next, back }: StepProps) {
  const s = useAutosave(detail.intake_id ?? "", (intake_id: string) =>
    save("/api/v1/applicant/applications/{id}/intake", { intake_id }),
  );
  return (
    <StepForm editable={editable} state={s.state} fields={s.fields} flush={s.flush} next={next} back={back}>
      <p>
        You are applying for <strong>{detail.programme_title}</strong>.{" "}
        <Link href={`/programmes/${detail.programme_slug}`} className="text-royal underline">
          Read about the programme
        </Link>
        . To apply for a different programme, start a new application from its page.
      </p>
      {detail.open_intakes.length === 0 ? (
        <Alert tone="warning" title="No intake is taking applications">
          This programme is not taking applications now. Contact the admissions office to ask about the next
          intake.
        </Alert>
      ) : (
        <fieldset
          className="flex flex-col gap-2"
          aria-describedby={s.fields.intake_id ? "field-intake_id-error" : undefined}
        >
          <legend className="mb-1 font-semibold">When do you want to start?</legend>
          {detail.open_intakes.map((i) => (
            <label
              key={i.id}
              className="flex min-h-touch cursor-pointer items-start gap-3 rounded-control border border-stone bg-white p-3 has-checked:border-royal has-checked:bg-royal/5"
            >
              <input
                type="radio"
                name="intake_id"
                id={i.id === detail.open_intakes[0].id ? "field-intake_id" : undefined}
                value={i.id}
                checked={s.value === i.id}
                onChange={() => s.update(i.id)}
                className="mt-1 size-5 accent-royal"
              />
              <span className="flex flex-col">
                <span className="font-semibold">{i.name}</span>
                <span className="text-sm text-ink-muted">
                  Starts {formatDate(i.starts_on)}
                  {i.application_deadline ? `. Apply by ${formatDate(i.application_deadline)}.` : "."}
                </span>
              </span>
            </label>
          ))}
          {s.fields.intake_id ? (
            <p id="field-intake_id-error" className="text-sm font-medium text-danger">
              {s.fields.intake_id}
            </p>
          ) : null}
        </fieldset>
      )}
    </StepForm>
  );
}

// ---------- 2. about you ----------

export function PersonalStep({ detail, editable, save, next, back }: StepProps) {
  const s = useAutosave<Personal>(detail.personal, (body) =>
    save("/api/v1/applicant/applications/{id}/personal", body),
  );
  const v = s.value;
  const set = (k: keyof Personal) => (e: { target: { value: string } }) =>
    s.update((prev) => ({ ...prev, [k]: e.target.value }));
  return (
    <StepForm editable={editable} state={s.state} fields={s.fields} flush={s.flush} next={next} back={back}>
      <p>Use your names as they appear on your ID or birth certificate.</p>
      <Input
        name="first_name"
        label="First name"
        required
        autoComplete="given-name"
        maxLength={80}
        value={v.first_name}
        error={s.fields.first_name}
        onChange={set("first_name")}
      />
      <Input
        name="middle_name"
        label="Middle name"
        autoComplete="additional-name"
        maxLength={80}
        value={v.middle_name}
        error={s.fields.middle_name}
        onChange={set("middle_name")}
      />
      <Input
        name="last_name"
        label="Last name"
        required
        autoComplete="family-name"
        maxLength={80}
        value={v.last_name}
        error={s.fields.last_name}
        onChange={set("last_name")}
      />
      <Input
        name="date_of_birth"
        label="Date of birth"
        type="date"
        required
        autoComplete="bday"
        hint="If you are under 18, we will also ask about your parent or guardian."
        value={v.date_of_birth}
        error={s.fields.date_of_birth}
        onChange={set("date_of_birth")}
      />
      <Select
        name="identity_document_type"
        label="Identity document"
        required
        value={v.identity_document_type}
        error={s.fields.identity_document_type}
        onChange={set("identity_document_type")}
      >
        <option value="">Choose one</option>
        <option value="national_id">National ID card</option>
        <option value="birth_certificate">Birth certificate</option>
        <option value="passport">Passport</option>
      </Select>
      <Input
        name="identity_document"
        label="Document number"
        required
        autoComplete="off"
        spellCheck={false}
        maxLength={30}
        className="font-mono"
        value={v.identity_document}
        error={s.fields.identity_document}
        onChange={set("identity_document")}
      />
      <Input
        name="phone"
        label="Phone number"
        type="tel"
        inputMode="tel"
        required
        autoComplete="tel"
        hint="For example 0712 345 678 or +254 712 345 678."
        value={v.phone}
        error={s.fields.phone}
        onChange={set("phone")}
      />
      <p className="text-sm text-ink-muted">
        We will email you at <strong>{detail.email}</strong>.
      </p>
      <Input
        name="nationality"
        label="Nationality"
        required
        autoComplete="off"
        hint="For example Kenyan."
        maxLength={60}
        value={v.nationality}
        error={s.fields.nationality}
        onChange={set("nationality")}
      />
      <Select
        name="county"
        label="County where you live"
        required
        value={v.county}
        error={s.fields.county}
        onChange={set("county")}
      >
        <option value="">Choose a county</option>
        {counties.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </Select>
    </StepForm>
  );
}

// ---------- 3. parent or guardian (under 18) ----------

export function GuardianStep({ detail, editable, save, next, back }: StepProps) {
  const s = useAutosave<Guardian>(detail.guardian, (body) =>
    save("/api/v1/applicant/applications/{id}/guardian", body),
  );
  const v = s.value;
  const set = (k: keyof Guardian) => (e: { target: { value: string } }) =>
    s.update((prev) => ({ ...prev, [k]: e.target.value }));
  return (
    <StepForm editable={editable} state={s.state} fields={s.fields} flush={s.flush} next={next} back={back}>
      <p>
        You are under 18, so we need the details of a parent or guardian, and their agreement before you
        apply.
      </p>
      <Input
        name="full_name"
        label="Their full name"
        required
        autoComplete="off"
        maxLength={120}
        value={v.full_name}
        error={s.fields.full_name}
        onChange={set("full_name")}
      />
      <Select
        name="relationship"
        label="How are they related to you?"
        required
        value={v.relationship}
        error={s.fields.relationship}
        onChange={set("relationship")}
      >
        <option value="">Choose one</option>
        <option value="mother">Mother</option>
        <option value="father">Father</option>
        <option value="guardian">Guardian</option>
        <option value="other_relative">Other relative</option>
        <option value="other">Someone else</option>
      </Select>
      <Input
        name="phone"
        label="Their phone number"
        type="tel"
        inputMode="tel"
        required
        autoComplete="off"
        value={v.phone}
        error={s.fields.phone}
        onChange={set("phone")}
      />
      <Input
        name="email"
        label="Their email address"
        type="email"
        inputMode="email"
        autoComplete="off"
        value={v.email}
        error={s.fields.email}
        onChange={set("email")}
      />
      <Checkbox
        name="consent_given"
        label="My parent or guardian named above has read the privacy notice and agrees that Dakota Training College may use my details to consider this application."
        hint={
          <>
            Show them the{" "}
            <Link href="/privacy" target="_blank" className="text-royal underline">
              privacy notice (opens in a new tab)
            </Link>{" "}
            first. If you change their name, ask them again.
          </>
        }
        checked={v.consent_given}
        error={s.fields.consent_given}
        onChange={(e) => s.update((prev) => ({ ...prev, consent_given: e.target.checked }))}
      />
    </StepForm>
  );
}

// ---------- 4. education ----------

const levels: { value: Education["level"]; label: string }[] = [
  { value: "kcse", label: "KCSE" },
  { value: "kcpe", label: "KCPE" },
  { value: "certificate", label: "Certificate" },
  { value: "diploma", label: "Diploma" },
  { value: "degree", label: "Degree" },
  { value: "other", label: "Other" },
];

const blankEducation = (): Education => ({
  level: "kcse",
  institution: "",
  year_completed: null,
  qualification: "",
  mean_grade: "",
  subjects: [],
});

export function EducationStep({ detail, editable, save, next, back }: StepProps) {
  const s = useAutosave<Education[]>(
    detail.education.length ? detail.education : [blankEducation()],
    (items) => save("/api/v1/applicant/applications/{id}/education", { items }),
  );
  const items = s.value;
  const change = (i: number, patch: Partial<Education>) =>
    s.update((prev) => prev.map((e, j) => (j === i ? { ...e, ...patch } : e)));
  const f = (i: number, k: string) => `education.${i}.${k}`;

  return (
    <StepForm editable={editable} state={s.state} fields={s.fields} flush={s.flush} next={next} back={back}>
      <p>Start with your KCSE or equivalent. Add any certificate, diploma or degree you have.</p>
      {items.map((e, i) => (
        <Card key={i} className="flex flex-col gap-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-lg font-semibold text-navy-900">School or college {i + 1}</h3>
            {items.length > 1 ? (
              <Button
                type="button"
                variant="ghost"
                icon={<Trash2 aria-hidden className="size-4" />}
                onClick={() => s.update((prev) => prev.filter((_, j) => j !== i))}
              >
                Remove<span className="sr-only"> school or college {i + 1}</span>
              </Button>
            ) : null}
          </div>
          <Select
            name={f(i, "level")}
            label="Level"
            required
            value={e.level}
            error={s.fields[f(i, "level")]}
            onChange={(ev) => change(i, { level: ev.target.value as Education["level"], mean_grade: "" })}
          >
            {levels.map((l) => (
              <option key={l.value} value={l.value}>
                {l.label}
              </option>
            ))}
          </Select>
          <Input
            name={f(i, "institution")}
            label="Name of the school or college"
            required
            maxLength={120}
            value={e.institution}
            error={s.fields[f(i, "institution")]}
            onChange={(ev) => change(i, { institution: ev.target.value })}
          />
          <Input
            name={f(i, "year_completed")}
            label="Year you finished"
            inputMode="numeric"
            autoComplete="off"
            maxLength={4}
            className="max-w-32"
            value={e.year_completed ?? ""}
            error={s.fields[f(i, "year_completed")]}
            onChange={(ev) => {
              const digits = ev.target.value.replace(/\D/g, "").slice(0, 4);
              change(i, { year_completed: digits ? Number(digits) : null });
            }}
          />
          {e.level === "kcse" || e.level === "kcpe" ? null : (
            <Input
              name={f(i, "qualification")}
              label="Qualification"
              hint="For example Certificate in Food Production."
              maxLength={120}
              value={e.qualification}
              error={s.fields[f(i, "qualification")]}
              onChange={(ev) => change(i, { qualification: ev.target.value })}
            />
          )}
          {e.level === "kcse" ? (
            <div className="max-w-40">
              <Select
                name={f(i, "mean_grade")}
                label="KCSE mean grade"
                required

                value={e.mean_grade}
                error={s.fields[f(i, "mean_grade")]}
                onChange={(ev) => change(i, { mean_grade: ev.target.value })}
              >
                <option value="">Choose</option>
                {kcseGrades.map((g) => (
                  <option key={g} value={g}>
                    {g}
                  </option>
                ))}
              </Select>
            </div>
          ) : (
            <Input
              name={f(i, "mean_grade")}
              label={e.level === "kcpe" ? "Total marks" : "Grade or class"}
              maxLength={20}
              className="max-w-48"
              value={e.mean_grade}
              error={s.fields[f(i, "mean_grade")]}
              onChange={(ev) => change(i, { mean_grade: ev.target.value })}
            />
          )}
          {e.level === "kcse" ? (
            <SubjectGrades
              index={i}
              subjects={e.subjects}
              fields={s.fields}
              onChange={(subjects) => change(i, { subjects })}
            />
          ) : null}
        </Card>
      ))}
      {items.length < 6 ? (
        <Button
          type="button"
          variant="outline"
          icon={<Plus aria-hidden className="size-5" />}
          onClick={() => s.update((prev) => [...prev, { ...blankEducation(), level: "certificate" }])}
          className="self-start"
        >
          Add another school or college
        </Button>
      ) : null}
    </StepForm>
  );
}

function SubjectGrades({
  index,
  subjects,
  fields,
  onChange,
}: {
  index: number;
  subjects: Education["subjects"];
  fields: Record<string, string>;
  onChange: (s: Education["subjects"]) => void;
}) {
  const f = (j: number, k: string) => `education.${index}.subjects.${j}.${k}`;
  const set = (j: number, patch: Partial<Education["subjects"][number]>) =>
    onChange(subjects.map((sg, k) => (k === j ? { ...sg, ...patch } : sg)));
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 font-semibold">
        Subject grades <span className="font-normal text-ink-muted">(optional)</span>
      </legend>
      <p className="text-sm text-ink-muted">
        Some programmes ask for a grade in a subject, such as English or Mathematics.
      </p>
      {subjects.map((sg, j) => (
        <div key={j} className="flex flex-wrap items-end gap-3">
          <div className="min-w-40 flex-1">
            <Input
              name={f(j, "subject")}
              label={`Subject ${j + 1}`}
              maxLength={60}
              value={sg.subject}
              error={fields[f(j, "subject")]}
              onChange={(e) => set(j, { subject: e.target.value })}
            />
          </div>
          <div className="w-28">
            <Select
              name={f(j, "grade")}
              label="Grade"
              value={sg.grade}
              error={fields[f(j, "grade")]}
              onChange={(e) => set(j, { grade: e.target.value })}
            >
              <option value="">Choose</option>
              {kcseGrades.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </Select>
          </div>
          <Button type="button" variant="ghost" onClick={() => onChange(subjects.filter((_, k) => k !== j))}>
            Remove<span className="sr-only"> subject {j + 1}</span>
          </Button>
        </div>
      ))}
      {subjects.length < 20 ? (
        <Button
          type="button"
          variant="ghost"
          icon={<Plus aria-hidden className="size-5" />}
          className="self-start"
          onClick={() => onChange([...subjects, { subject: "", grade: "" }])}
        >
          Add a subject
        </Button>
      ) : null}
    </fieldset>
  );
}

// ---------- 5. fees and sponsor ----------

const payers: { value: Sponsor["type"]; label: string }[] = [
  { value: "self", label: "I will pay my own fees" },
  { value: "parent_guardian", label: "My parent or guardian" },
  { value: "sponsor", label: "A sponsor, employer or organisation" },
  { value: "other", label: "Someone else" },
];

export function SponsorStep({ detail, editable, save, next, back }: StepProps) {
  const s = useAutosave<Sponsor>(detail.sponsor, (body) =>
    save("/api/v1/applicant/applications/{id}/sponsor", body),
  );
  const v = s.value;
  return (
    <StepForm editable={editable} state={s.state} fields={s.fields} flush={s.flush} next={next} back={back}>
      <p>
        Tell us who will pay your fees, so the college knows who to contact about them. You can see the fees
        on the{" "}
        <Link href={`/programmes/${detail.programme_slug}`} className="text-royal underline">
          programme page
        </Link>
        .
      </p>
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 font-semibold">Who will pay your fees?</legend>
        {payers.map((p, i) => (
          <label
            key={p.value}
            className="flex min-h-touch cursor-pointer items-center gap-3 rounded-control border border-stone bg-white p-3 has-checked:border-royal has-checked:bg-royal/5"
          >
            <input
              type="radio"
              name="type"
              id={i === 0 ? "field-type" : undefined}
              value={p.value}
              checked={v.type === p.value}
              onChange={() => s.update((prev) => ({ ...prev, type: p.value }))}
              className="size-5 accent-royal"
            />
            {p.label}
          </label>
        ))}
        {s.fields.type ? <p className="text-sm font-medium text-danger">{s.fields.type}</p> : null}
      </fieldset>
      {v.type && v.type !== "self" ? (
        <>
          <Input
            name="name"
            label="Their name"
            required
            maxLength={120}
            value={v.name}
            error={s.fields.name}
            onChange={(e) => s.update((prev) => ({ ...prev, name: e.target.value }))}
          />
          <Input
            name="phone"
            label="Their phone number"
            type="tel"
            inputMode="tel"
            required
            value={v.phone}
            error={s.fields.phone}
            onChange={(e) => s.update((prev) => ({ ...prev, phone: e.target.value }))}
          />
        </>
      ) : null}
    </StepForm>
  );
}

// ---------- 6. check your answers ----------

const docLabels: Record<string, string> = {
  national_id: "National ID card",
  birth_certificate: "Birth certificate",
  passport: "Passport",
};

function Summary({
  title,
  step,
  goTo,
  editable,
  children,
}: {
  title: string;
  step: StepKey;
  goTo: (s: StepKey) => void;
  editable: boolean;
  children: ReactNode;
}) {
  return (
    <Card className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-lg font-semibold text-navy-900">{title}</h3>
        <Button type="button" variant="ghost" onClick={() => goTo(step)}>
          {editable ? "Change" : "View"}
          <span className="sr-only"> {title.toLowerCase()}</span>
        </Button>
      </div>
      <dl className="grid gap-x-6 gap-y-2 sm:grid-cols-[12rem_1fr]">{children}</dl>
    </Card>
  );
}

function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <>
      <dt className="font-semibold">{label}</dt>
      <dd className="break-words">{value || <span className="text-ink-muted">Not given</span>}</dd>
    </>
  );
}

export function ReviewStep({ id, detail, editable, back, goTo, reload }: StepProps) {
  const p = detail.personal;
  const steps = [...new Set(detail.missing.map((m) => m.step))] as StepKey[];
  return (
    <div className="flex flex-col gap-5">
      {detail.missing.length > 0 ? (
        <Alert tone="warning" title="Some answers are still needed">
          <ul className="flex flex-col gap-3">
            {steps.map((st) => (
              <li key={st}>
                <button
                  type="button"
                  onClick={() => goTo(st)}
                  className="min-h-touch font-semibold text-royal underline"
                >
                  {stepTitles[st]}
                </button>
                <ul className="list-disc pl-5">
                  {detail.missing
                    .filter((m) => m.step === st)
                    .map((m) => (
                      <li key={m.field}>{m.message}</li>
                    ))}
                </ul>
              </li>
            ))}
          </ul>
        </Alert>
      ) : (
        <Alert tone="success" title="All the answers we need are here">
          Check them below. You can change any of them until you submit, at the bottom of this page.
        </Alert>
      )}

      <Summary title={stepTitles.choice} step="choice" goTo={goTo} editable={editable}>
        <Row label="Programme" value={detail.programme_title} />
        <Row
          label="Intake"
          value={
            detail.intake_name && detail.intake_starts_on
              ? `${detail.intake_name}, starting ${formatDate(detail.intake_starts_on)}`
              : ""
          }
        />
      </Summary>
      <Summary title={stepTitles.personal} step="personal" goTo={goTo} editable={editable}>
        <Row label="Name" value={[p.first_name, p.middle_name, p.last_name].filter(Boolean).join(" ")} />
        <Row label="Date of birth" value={p.date_of_birth ? formatDate(p.date_of_birth) : ""} />
        <Row
          label="Identity document"
          value={
            p.identity_document_type ? `${docLabels[p.identity_document_type]} ${p.identity_document}` : ""
          }
        />
        <Row label="Phone" value={p.phone} />
        <Row label="Email" value={detail.email} />
        <Row label="Nationality" value={p.nationality} />
        <Row label="County" value={p.county} />
      </Summary>
      {detail.needs_guardian ? (
        <Summary title={stepTitles.guardian} step="guardian" goTo={goTo} editable={editable}>
          <Row label="Name" value={detail.guardian.full_name} />
          <Row label="Phone" value={detail.guardian.phone} />
          <Row label="Email" value={detail.guardian.email} />
          <Row label="Agreement" value={detail.guardian.consent_given ? "Given" : ""} />
        </Summary>
      ) : null}
      <Summary title={stepTitles.education} step="education" goTo={goTo} editable={editable}>
        {detail.education.length === 0 ? <Row label="Schools" value="" /> : null}
        {detail.education.map((e, i) => (
          <Row
            key={i}
            label={levels.find((l) => l.value === e.level)?.label ?? e.level}
            value={[e.institution, e.qualification, e.year_completed, e.mean_grade && `grade ${e.mean_grade}`]
              .filter(Boolean)
              .join(", ")}
          />
        ))}
      </Summary>
      <Summary title={stepTitles.sponsor} step="sponsor" goTo={goTo} editable={editable}>
        <Row label="Who will pay" value={payers.find((x) => x.value === detail.sponsor.type)?.label ?? ""} />
        {detail.sponsor.type && detail.sponsor.type !== "self" ? (
          <>
            <Row label="Name" value={detail.sponsor.name} />
            <Row label="Phone" value={detail.sponsor.phone} />
          </>
        ) : null}
      </Summary>

      <Summary title={stepTitles.documents} step="documents" goTo={goTo} editable={editable}>
        {detail.documents.length === 0 ? <Row label="Documents" value="" /> : null}
        {detail.documents.map((doc) => (
          <Row key={doc.id} label={kindLabels[doc.kind]} value={doc.name} />
        ))}
      </Summary>

      {editable ? (
        <SubmitPanel id={id} ready={detail.missing.length === 0} back={back} reload={reload} />
      ) : (
        <div className="flex flex-col-reverse gap-3 border-t border-stone pt-5 sm:flex-row sm:justify-between">
          {back ? (
            <Button type="button" variant="outline" onClick={back}>
              Back
            </Button>
          ) : (
            <span />
          )}
          <Link
            href="/apply/account"
            className="inline-flex min-h-touch items-center font-semibold text-royal underline"
          >
            Back to your applications
          </Link>
        </div>
      )}
    </div>
  );
}
