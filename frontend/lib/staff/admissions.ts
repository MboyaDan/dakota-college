import type { Status } from "@/components/ui/tag";
import type { components } from "@/lib/api/schema";

// Staff wording for the admissions workflow (Stage 10b). Applicants see
// their own wording (lib/apply/status.ts).

export type AdmissionStatus = components["schemas"]["MyApplication"]["status"];
export type Action = components["schemas"]["StaffApplicationDetail"]["actions"][number];

export const staffStatus: Record<AdmissionStatus, { label: string; tone: Status }> = {
  draft: { label: "Draft", tone: "warning" },
  submitted: { label: "New", tone: "info" },
  under_review: { label: "In review", tone: "info" },
  info_requested: { label: "Waiting on applicant", tone: "warning" },
  offered: { label: "Offered", tone: "success" },
  offer_accepted: { label: "Accepted, to enrol", tone: "success" },
  enrolled: { label: "Enrolled", tone: "success" },
  rejected: { label: "Rejected", tone: "danger" },
  withdrawn: { label: "Withdrawn", tone: "info" },
  expired: { label: "Expired", tone: "danger" },
};

/** Statuses staff can filter the inbox by, in workflow order. */
export const inboxStatuses: Exclude<AdmissionStatus, "draft">[] = [
  "submitted",
  "under_review",
  "info_requested",
  "offered",
  "offer_accepted",
  "enrolled",
  "rejected",
  "withdrawn",
  "expired",
];

/** Shortcuts above the inbox: the queues people work from. */
export const inboxViews: { label: string; query: Record<string, string> }[] = [
  { label: "New", query: { status: "submitted" } },
  { label: "Assigned to me", query: { assigned: "me" } },
  { label: "Waiting on applicant", query: { status: "info_requested" } },
  { label: "Ready to enrol", query: { status: "offer_accepted" } },
  { label: "All", query: {} },
];

export const identityLabels: Record<string, string> = {
  national_id: "National ID card",
  birth_certificate: "Birth certificate",
  passport: "Passport",
};

export const relationshipLabels: Record<string, string> = {
  mother: "Mother",
  father: "Father",
  guardian: "Guardian",
  other_relative: "Other relative",
  other: "Other",
};

export const levelLabels: Record<string, string> = {
  kcse: "KCSE",
  kcpe: "KCPE",
  certificate: "Certificate",
  diploma: "Diploma",
  degree: "Degree",
  other: "Other",
};

export const sponsorLabels: Record<string, string> = {
  self: "The applicant",
  parent_guardian: "Parent or guardian",
  sponsor: "A sponsor, employer or organisation",
  other: "Someone else",
};

/** Whole years between a YYYY-MM-DD birth date and today (Nairobi). */
export function ageOn(dateOfBirth: string, today: Date = new Date()): number | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateOfBirth);
  if (!m) return null;
  const nairobi = new Date(today.getTime() + 3 * 60 * 60 * 1000);
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  let age = nairobi.getUTCFullYear() - y;
  if (nairobi.getUTCMonth() + 1 < mo || (nairobi.getUTCMonth() + 1 === mo && nairobi.getUTCDate() < d)) age--;
  return age;
}
