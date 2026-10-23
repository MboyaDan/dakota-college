import type { components } from "@/lib/api/schema";
import type { Status } from "@/components/ui/tag";

export type ApplicationStatus = components["schemas"]["MyApplication"]["status"];

/** What each application state means to the applicant, in plain words (brief 8). */
export const applicationStatus: Record<ApplicationStatus, { label: string; tone: Status; next: string }> = {
  draft: { label: "Not submitted", tone: "warning", next: "Finish the form and submit it." },
  submitted: { label: "Received", tone: "info", next: "The admissions office will review it." },
  under_review: { label: "Being reviewed", tone: "info", next: "An admissions officer is reviewing it." },
  info_requested: {
    label: "We need more from you",
    tone: "warning",
    next: "Check your email for what to send.",
  },
  offered: { label: "Place offered", tone: "success", next: "Read your offer and reply to it." },
  offer_accepted: { label: "Offer accepted", tone: "success", next: "The registrar will enrol you." },
  enrolled: { label: "Enrolled", tone: "success", next: "Welcome to Dakota Training College." },
  rejected: { label: "Not successful", tone: "danger", next: "Check your email for the reason." },
  withdrawn: { label: "Withdrawn", tone: "info", next: "You withdrew this application." },
  expired: { label: "Expired", tone: "danger", next: "Start a new application if you still want to apply." },
};
