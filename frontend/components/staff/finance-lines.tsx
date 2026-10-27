"use client";

import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/field";
import {
  amountHint,
  categoryLabel,
  chargeLabel,
  parseAmount,
  percentTotal,
  type Category,
  type Charge,
} from "@/lib/finance";
import { centsToInput, formatKes } from "@/lib/money";

// Editors for fee lines and instalment plans (Stage 14b), used by draft
// invoices and fee structures. Amounts are kept as typed and turned into
// cents only when saved, so a half-typed amount is never rounded.

export type LineDraft = { label: string; category: Category; charge: Charge; amount: string };
export type PlanDraft = { percent: string; due_days: string };

export function toLineDrafts(
  lines: { label: string; category: string; charge?: string; amount_cents: number }[],
): LineDraft[] {
  return lines.map((l) => ({
    label: l.label,
    category: l.category as Category,
    charge: (l.charge ?? "per_term") as Charge,
    amount: centsToInput(l.amount_cents),
  }));
}

export function toPlanDrafts(plan: { percent: number; due_days: number }[]): PlanDraft[] {
  return plan.map((p) => ({ percent: String(p.percent), due_days: String(p.due_days) }));
}

/** Lines ready to send, or the field messages for amounts that are not amounts. */
export function linesToSend(lines: LineDraft[], form: string) {
  const problems: Record<string, string> = {};
  const out = lines.map((l, i) => {
    const cents = parseAmount(l.amount);
    if (cents === null)
      problems[`${form}lines.${i}.amount_cents`] = `Enter an amount above zero. ${amountHint}`;
    return { label: l.label, category: l.category, charge: l.charge, amount_cents: cents ?? 0 };
  });
  return { lines: out, problems };
}

const whole = (v: string) => (/^\d{1,4}$/.test(v.trim()) ? Number(v.trim()) : Number.NaN);

/** The plan as numbers; a field that is not a whole number is NaN. */
export function planToSend(plan: PlanDraft[]) {
  return plan.map((p) => ({ percent: whole(p.percent), due_days: whole(p.due_days) }));
}

/** Messages for plan fields that are not whole numbers (NaN would be sent as null). */
export function planProblems(plan: PlanDraft[], form: string): Record<string, string> {
  const problems: Record<string, string> = {};
  planToSend(plan).forEach((p, i) => {
    if (Number.isNaN(p.percent))
      problems[`${form}instalments.${i}.percent`] = "Enter a whole number from 1 to 100.";
    if (Number.isNaN(p.due_days))
      problems[`${form}instalments.${i}.due_days`] = "Enter a whole number of days from 0 to 730.";
  });
  return problems;
}

/** The total of the lines that are valid amounts so far. */
export function linesTotal(lines: LineDraft[], only?: Charge): number {
  return lines.reduce((sum, l) => (only && l.charge !== only ? sum : sum + (parseAmount(l.amount) ?? 0)), 0);
}

export function LinesEditor({
  lines,
  setLines,
  errors,
  form,
  withCharge,
}: {
  lines: LineDraft[];
  setLines: (l: LineDraft[]) => void;
  errors: Record<string, string>;
  form: string;
  withCharge: boolean;
}) {
  const set = (i: number, patch: Partial<LineDraft>) =>
    setLines(lines.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 font-semibold">Fees</legend>
      {lines.length === 0 ? <p>No lines yet.</p> : null}
      {lines.map((l, i) => (
        <div
          key={i}
          className={
            withCharge
              ? "grid grid-cols-1 items-end gap-3 rounded-card border border-stone bg-white p-3 sm:grid-cols-2 xl:grid-cols-[2fr_1fr_1fr_1fr_auto]"
              : "grid grid-cols-1 items-end gap-3 rounded-card border border-stone bg-white p-3 sm:grid-cols-2 xl:grid-cols-[2fr_1fr_1fr_auto]"
          }
        >
          <Input
            id={`${form}line-${i}-label`}
            name={`${form}line-${i}-label`}
            label="Fee"
            required
            maxLength={120}
            value={l.label}
            error={errors[`${form}lines.${i}.label`]}
            onChange={(e) => set(i, { label: e.target.value })}
          />
          <Select
            id={`${form}line-${i}-category`}
            name={`${form}line-${i}-category`}
            label="Type"
            required
            value={l.category}
            error={errors[`${form}lines.${i}.category`]}
            onChange={(e) => set(i, { category: e.target.value as Category })}
          >
            {(Object.keys(categoryLabel) as Category[]).map((c) => (
              <option key={c} value={c}>
                {categoryLabel[c]}
              </option>
            ))}
          </Select>
          {withCharge ? (
            <Select
              id={`${form}line-${i}-charge`}
              name={`${form}line-${i}-charge`}
              label="Charged"
              required
              value={l.charge}
              error={errors[`${form}lines.${i}.charge`]}
              onChange={(e) => set(i, { charge: e.target.value as Charge })}
            >
              {(Object.keys(chargeLabel) as Charge[]).map((c) => (
                <option key={c} value={c}>
                  {chargeLabel[c]}
                </option>
              ))}
            </Select>
          ) : null}
          <Input
            id={`${form}line-${i}-amount`}
            name={`${form}line-${i}-amount`}
            label="Amount (KES)"
            required
            inputMode="decimal"
            autoComplete="off"
            value={l.amount}
            error={errors[`${form}lines.${i}.amount_cents`]}
            onChange={(e) => set(i, { amount: e.target.value })}
          />
          <Button variant="ghost" onClick={() => setLines(lines.filter((_, j) => j !== i))}>
            Remove<span className="sr-only"> {l.label || `fee ${i + 1}`}</span>
          </Button>
        </div>
      ))}
      {errors[`${form}lines`] ? <p className="font-medium text-danger">{errors[`${form}lines`]}</p> : null}
      <p className="text-sm text-ink-muted">{amountHint}</p>
      <Button
        variant="outline"
        className="self-start"
        disabled={lines.length >= 30}
        onClick={() =>
          setLines([...lines, { label: "", category: "tuition", charge: "per_term", amount: "" }])
        }
      >
        Add a fee
      </Button>
    </fieldset>
  );
}

export function PlanEditor({
  plan,
  setPlan,
  errors,
  form,
  total,
}: {
  plan: PlanDraft[];
  setPlan: (p: PlanDraft[]) => void;
  errors: Record<string, string>;
  form: string;
  /** The invoice total, to show each share in shillings; null when it varies. */
  total: number | null;
}) {
  const set = (i: number, patch: Partial<PlanDraft>) =>
    setPlan(plan.map((p, j) => (j === i ? { ...p, ...patch } : p)));
  const sum = percentTotal(planToSend(plan));
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 font-semibold">Instalments</legend>
      <p className="text-sm text-ink-muted">
        {plan.length === 0
          ? "None: everything is due when the invoice is issued."
          : "Shares of the total, each due a number of days after the invoice is issued. Up to 12; the shares add up to 100."}
      </p>
      {plan.map((p, i) => {
        const share = Number(p.percent);
        return (
          <div
            key={i}
            className="grid grid-cols-2 items-end gap-3 rounded-card border border-stone bg-white p-3 sm:grid-cols-[8rem_10rem_1fr_auto]"
          >
            <Input
              id={`${form}plan-${i}-percent`}
              name={`${form}plan-${i}-percent`}
              label="Share (%)"
              required
              inputMode="numeric"
              value={p.percent}
              error={errors[`${form}instalments.${i}.percent`]}
              onChange={(e) => set(i, { percent: e.target.value })}
            />
            <Input
              id={`${form}plan-${i}-days`}
              name={`${form}plan-${i}-days`}
              label="Due after (days)"
              required
              inputMode="numeric"
              hint={i === 0 ? "0 means on issue." : undefined}
              value={p.due_days}
              error={errors[`${form}instalments.${i}.due_days`]}
              onChange={(e) => set(i, { due_days: e.target.value })}
            />
            <p className="col-span-2 self-center text-sm sm:col-span-1">
              {total !== null && Number.isInteger(share) && share > 0 && share <= 100
                ? `About ${formatKes(Math.floor((total * share) / 100))}`
                : ""}
            </p>
            <Button
              variant="ghost"
              className="col-span-2 justify-self-start sm:col-span-1"
              onClick={() => setPlan(plan.filter((_, j) => j !== i))}
            >
              Remove<span className="sr-only"> instalment {i + 1}</span>
            </Button>
          </div>
        );
      })}
      {plan.length > 0 ? (
        <p aria-live="polite" className={sum === 100 ? "text-success" : "font-semibold text-danger"}>
          The shares add up to {sum}
          {sum === 100 ? "." : ". They must add up to 100."}
        </p>
      ) : null}
      {errors[`${form}instalments`] ? (
        <p className="font-medium text-danger">{errors[`${form}instalments`]}</p>
      ) : null}
      <Button
        variant="outline"
        className="self-start"
        disabled={plan.length >= 12}
        onClick={() =>
          setPlan([
            ...plan,
            { percent: plan.length === 0 ? "100" : "", due_days: plan.length === 0 ? "0" : "" },
          ])
        }
      >
        Add an instalment
      </Button>
    </fieldset>
  );
}
