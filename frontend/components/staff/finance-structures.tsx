"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Checkbox, Select, Textarea } from "@/components/ui/field";
import { Tag } from "@/components/ui/tag";
import { api } from "@/lib/api/client";
import { describePlan } from "@/lib/finance";
import { formatKes } from "@/lib/money";
import { formatNairobi } from "@/lib/staff/datetime";
import { LoadState, useLoaded, type Structure } from "./finance-common";
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

// Fee structures (Stage 14b, D-157): what each programme charges, for one
// intake or for every intake. Invoices copy the lines when they are made,
// so editing a structure changes only invoices made afterwards.

function NewStructure() {
  const router = useRouter();
  const programmes = useLoaded(() => api.GET("/api/v1/staff/finance/programmes"), []);
  const [programme, setProgramme] = useState("");
  const [intake, setIntake] = useState("");
  const [copy, setCopy] = useState(true);
  const { busy, errors, run } = useAction();
  const intakes = programmes.data?.items.find((p) => p.id === programme)?.intakes ?? [];

  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        const s = await run(
          () =>
            api.POST("/api/v1/staff/finance/fee-structures", {
              body: { programme_id: programme, intake_id: intake || null, from_published: copy },
            }),
          "Fee structure started",
          "new.",
        );
        if (s) {
          const warn = s.warnings?.length ? `?warnings=${encodeURIComponent(s.warnings.join("\n"))}` : "";
          router.push(`/staff/finance/structures/${s.id}${warn}`);
        }
      }}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Select
          id="structure-programme"
          name="programme_id"
          label="Programme"
          required
          value={programme}
          error={errors["new.programme_id"]}
          onChange={(e) => {
            setProgramme(e.target.value);
            setIntake("");
          }}
        >
          <option value="">Choose a programme</option>
          {(programmes.data?.items ?? []).map((p) => (
            <option key={p.id} value={p.id}>
              {p.title}
            </option>
          ))}
        </Select>
        <Select
          id="structure-intake"
          name="intake_id"
          label="Intake"
          value={intake}
          error={errors["new.intake_id"]}
          hint="An intake's own structure is used before the one for every intake."
          onChange={(e) => setIntake(e.target.value)}
        >
          <option value="">Every intake</option>
          {intakes.map((i) => (
            <option key={i.id} value={i.id}>
              {i.name}
            </option>
          ))}
        </Select>
      </div>
      <Checkbox
        id="structure-copy"
        name="from_published"
        label="Start from the fees published on the programme page"
        hint="You can change every line before it is used."
        checked={copy}
        onChange={(e) => setCopy(e.target.checked)}
      />
      <Button type="submit" variant="secondary" loading={busy} className="self-start">
        Start fee structure
      </Button>
    </form>
  );
}

export function StructureList() {
  const { can } = useSession();
  const { data, error } = useLoaded(() => api.GET("/api/v1/staff/finance/fee-structures"), []);
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Fee structures</h1>
        <p className="max-w-prose text-ink-muted">
          What each programme charges, which fees are charged once at enrolment and which every term, and how
          an invoice is split into instalments. Invoices already made keep their own lines.
        </p>
      </div>
      {!data ? (
        <LoadState error={error} label="Loading fee structures" />
      ) : data.items.length === 0 ? (
        <EmptyState
          title="No fee structures yet"
          description="Until a programme has one, its invoices are filled in by hand."
        />
      ) : (
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
          {data.items.map((s) => (
            <li key={s.id}>
              <Link
                href={`/staff/finance/structures/${s.id}`}
                className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:justify-between"
              >
                <span className="flex flex-col">
                  <span className="font-semibold text-royal">{s.programme_title}</span>
                  <span className="text-sm text-ink-muted">
                    {s.intake_name ?? "Every intake"} · at enrolment{" "}
                    {formatKes(s.once_cents + s.per_term_cents)} · each later term{" "}
                    {formatKes(s.per_term_cents)}
                  </span>
                </span>
                {s.is_sample ? <Tag>Sample</Tag> : null}
              </Link>
            </li>
          ))}
        </ul>
      )}
      {can("finance.invoice") ? (
        <Card className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold">New fee structure</h2>
          <NewStructure />
        </Card>
      ) : null}
    </div>
  );
}

function StructureEditor({ s, onSaved }: { s: Structure; onSaved: (s: Structure) => void }) {
  const [lines, setLines] = useState<LineDraft[]>(toLineDrafts(s.lines));
  const [plan, setPlan] = useState<PlanDraft[]>(toPlanDrafts(s.instalments));
  const [note, setNote] = useState(s.note);
  const { busy, errors, setErrors, run } = useAction();
  const once = linesTotal(lines, "once");
  const perTerm = linesTotal(lines, "per_term");

  return (
    <form
      noValidate
      className="flex flex-col gap-6"
      onSubmit={async (e) => {
        e.preventDefault();
        const sent = linesToSend(lines, "");
        const problems = { ...sent.problems, ...planProblems(plan, "") };
        if (Object.keys(problems).length > 0) {
          setErrors(problems);
          return;
        }
        const saved = await run(
          () =>
            api.PUT("/api/v1/staff/finance/fee-structures/{id}", {
              params: { path: { id: s.id } },
              body: { note, lines: sent.lines, instalments: planToSend(plan) },
            }),
          "Fee structure saved",
        );
        if (saved) onSaved(saved);
      }}
    >
      <LinesEditor lines={lines} setLines={setLines} errors={errors} form="" withCharge />
      <p>
        First invoice (at enrolment): <span className="font-semibold">{formatKes(once + perTerm)}</span>. Each
        later term: <span className="font-semibold">{formatKes(perTerm)}</span>.
      </p>
      <PlanEditor plan={plan} setPlan={setPlan} errors={errors} form="" total={null} />
      <Textarea
        id="structure-note"
        name="note"
        label="Note for finance staff"
        rows={2}
        maxLength={1000}
        value={note}
        error={errors.note}
        onChange={(e) => setNote(e.target.value)}
      />
      <Button type="submit" variant="secondary" loading={busy} className="self-start">
        Save fee structure
      </Button>
    </form>
  );
}

export function StructureView({ id, warnings }: { id: string; warnings: string[] }) {
  const { can } = useSession();
  const router = useRouter();
  const {
    data: s,
    setData,
    error,
  } = useLoaded(
    () => api.GET("/api/v1/staff/finance/fee-structures/{id}", { params: { path: { id } } }),
    [id],
  );
  const [version, setVersion] = useState(0);
  const { run } = useAction();
  if (!s) return <LoadState error={error} label="Loading the fee structure" />;

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb
        items={[{ label: "Fee structures", href: "/staff/finance/structures" }, { label: s.programme_title }]}
      />
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-3xl font-semibold break-words">{s.programme_title}</h1>
          {s.is_sample ? <Tag>Sample</Tag> : null}
        </div>
        <p className="text-ink-muted">
          {s.intake_name ?? "Every intake"} · updated {formatNairobi(s.updated_at)}
          {s.updated_by_name ? ` by ${s.updated_by_name}` : ""}
        </p>
        <p>{describePlan(s.instalments)}</p>
      </div>
      {warnings.length > 0 ? (
        <Alert tone="warning" title="Check these lines">
          <ul className="list-disc pl-5">
            {warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        </Alert>
      ) : null}
      {can("finance.invoice") ? (
        <>
          <StructureEditor
            key={version}
            s={s}
            onSaved={(next) => {
              setData(next);
              setVersion((v) => v + 1);
            }}
          />
          <Card className="flex flex-col gap-3">
            <h2 className="text-xl font-semibold">Remove</h2>
            <p>
              Invoices already made from it keep their lines. New invoices for this programme are then filled
              by hand.
            </p>
            <div>
              <Dialog
                title="Remove this fee structure?"
                triggerLabel="Remove fee structure"
                triggerVariant="outline"
                actions={
                  <>
                    <Button type="submit" variant="outline">
                      Cancel
                    </Button>
                    <Button
                      type="submit"
                      variant="danger"
                      onClick={async () => {
                        const ok = await run(
                          () =>
                            api.DELETE("/api/v1/staff/finance/fee-structures/{id}", {
                              params: { path: { id } },
                            }),
                          "Fee structure removed",
                        );
                        if (ok) router.push("/staff/finance/structures");
                      }}
                    >
                      Remove
                    </Button>
                  </>
                }
              >
                <p>
                  {s.programme_title}, {s.intake_name ?? "every intake"}.
                </p>
              </Dialog>
            </div>
          </Card>
        </>
      ) : (
        <ul className="flex flex-col gap-1">
          {s.lines.map((l, i) => (
            <li key={i}>
              {l.label}: {formatKes(l.amount_cents)} (
              {l.charge === "once" ? "once, at enrolment" : "every term"})
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
