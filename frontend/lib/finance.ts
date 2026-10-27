import type { Status } from "@/components/ui/tag";
import { formatKes, parseKesToCents } from "./money";

// Finance wording and rules on screen (Stage 14b), shared by the staff
// finance screens and the student Fees page. Amounts are integer cents
// (lib/money); the backend decides everything, these only explain.

/** A positive amount typed in shillings, in cents; null if not one. */
export function parseAmount(text: string): number | null {
  const cents = parseKesToCents(text);
  return cents !== null && cents > 0 ? cents : null;
}

export const amountHint = "In shillings, for example 12,500 or 12,500.50.";

export const methodLabel = { cash: "Cash", bank: "Bank", mpesa: "M-Pesa", sponsor: "Sponsor" } as const;
export type Method = keyof typeof methodLabel;

export const categoryLabel = {
  tuition: "Tuition",
  registration: "Registration",
  assessment: "Assessment",
  caution: "Caution",
  other: "Other",
} as const;
export type Category = keyof typeof categoryLabel;

export const chargeLabel = { once: "Once, at enrolment", per_term: "Every term" } as const;
export type Charge = keyof typeof chargeLabel;

export const paymentStatus: Record<"pending" | "confirmed" | "rejected", { label: string; tone: Status }> = {
  pending: { label: "Waiting for confirmation", tone: "warning" },
  confirmed: { label: "Confirmed", tone: "success" },
  rejected: { label: "Rejected", tone: "danger" },
};

export const invoiceStatus: Record<"draft" | "issued" | "void", { label: string; tone: Status }> = {
  draft: { label: "Draft", tone: "info" },
  issued: { label: "Issued", tone: "success" },
  void: { label: "Voided", tone: "danger" },
};

/** What an invoice is for, in words. */
export function invoiceFor(kind: string, term: string | null | undefined): string {
  if (kind === "enrolment") return "First fees, at enrolment";
  return term ?? "Other fees";
}

/** "Owes KES 1.00", "In credit KES 1.00" or "Nothing owed". */
export function describeBalance(cents: number): string {
  if (cents > 0) return `Owes ${formatKes(cents)}`;
  if (cents < 0) return `In credit ${formatKes(-cents)}`;
  return "Nothing owed";
}

/** The plan "50% on issue, 50% after 30 days" in words. */
export function describePlan(plan: { percent: number; due_days: number }[]): string {
  if (plan.length === 0) return "All due when the invoice is issued.";
  return (
    plan
      .map((p) => `${p.percent}% ${p.due_days === 0 ? "on issue" : `after ${p.due_days} days`}`)
      .join(", ") + "."
  );
}

/** Sum of the shares of a plan, for a running total while it is edited. */
export function percentTotal(plan: { percent: number }[]): number {
  return plan.reduce((sum, p) => sum + (Number.isFinite(p.percent) ? p.percent : 0), 0);
}

/** A fresh key for one action (NN-04); the same key is kept for retries. */
export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

/** An M-Pesa code as the API wants it: ten capital letters or digits. */
export function normaliseMpesaCode(text: string): string {
  return text.replace(/\s+/g, "").toUpperCase();
}
