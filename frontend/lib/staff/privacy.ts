import type { Status } from "@/components/ui/tag";
import type { components } from "@/lib/api/schema";

// Wording for data protection requests and the retention log (Stage 17e,
// docs/privacy-requests.md, docs/retention.md).

type Request = components["schemas"]["DataRequest"];
type RuleResult = components["schemas"]["RuleResult"];
export type RequestKind = Request["kind"];
export type RequestEvent = components["schemas"]["DataRequestEvent"];

export const requestKinds: { value: RequestKind; label: string; hint: string }[] = [
  { value: "access", label: "See their data", hint: "Export the data, check it, then send it." },
  {
    value: "portability",
    label: "Take their data elsewhere",
    hint: "Export the data, check it, then send it.",
  },
  {
    value: "correction",
    label: "Correct their data",
    hint: "Correct it on the record, then note what changed.",
  },
  {
    value: "erasure",
    label: "Delete their data",
    hint: "Erase the records that can be erased; refuse the rest with the reason.",
  },
  {
    value: "objection",
    label: "Stop a use of their data",
    hint: "Decide with the Principal and note the decision.",
  },
  {
    value: "withdraw_consent",
    label: "Withdraw consent",
    hint: "Stop the use (for example remove a photo) and note what was done.",
  },
  { value: "other", label: "Something else", hint: "Note what was asked and what was done." },
];

export function kindLabel(kind: RequestKind): string {
  return requestKinds.find((k) => k.value === kind)?.label ?? kind;
}

export const requestStatus: Record<Request["status"], { label: string; tone: Status }> = {
  open: { label: "Open", tone: "info" },
  completed: { label: "Completed", tone: "success" },
  refused: { label: "Refused", tone: "danger" },
};

const eventLabels: Record<RequestEvent["action"], string> = {
  recorded: "Recorded",
  identity_checked: "Identity checked",
  note: "Note",
  exported: "Data exported",
  erased: "Record erased",
  completed: "Completed",
  refused: "Refused",
};

export function eventLabel(e: RequestEvent): string {
  return eventLabels[e.action] ?? e.action;
}

/** Whether an export fits this kind of request (D-213). */
export function canExport(kind: RequestKind): boolean {
  return kind === "access" || kind === "portability";
}

/** Days until the due date (negative when overdue), in Nairobi dates. */
export function daysLeft(dueOn: string, today: string): number {
  return Math.round((Date.parse(dueOn) - Date.parse(today)) / 86_400_000);
}

export function dueText(dueOn: string, today: string): string {
  const d = daysLeft(dueOn, today);
  if (d < 0) return d === -1 ? "1 day overdue" : `${-d} days overdue`;
  if (d === 0) return "Due today";
  return d === 1 ? "Due tomorrow" : `Due in ${d} days`;
}

export const ruleLabels: Record<string, string> = {
  removed_files: "Files of erased records, removed from storage",
};

/** "12 removed", "3 due, not removed (report)" or "Nothing due". */
export function resultText(r: RuleResult, mode: "report" | "apply"): string {
  if (r.due === 0) return "Nothing due";
  if (r.done === r.due) return `${r.done} removed`;
  if (r.done === 0 && mode === "report") return `${r.due} due, not removed (report)`;
  return `${r.done} of ${r.due} removed`;
}

/** Today's date in Nairobi as YYYY-MM-DD. */
export function nairobiToday(now = new Date()): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Africa/Nairobi" }).format(now);
}
