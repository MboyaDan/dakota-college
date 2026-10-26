import type { Status } from "@/components/ui/tag";
import type { components } from "@/lib/api/schema";

// Service request wording, shared by the student portal and the staff screen.

type Request = components["schemas"]["ServiceRequest"];

export const requestKinds: { value: Request["kind"]; label: string }[] = [
  { value: "letter", label: "A letter (for example, proof of studies)" },
  { value: "transcript", label: "A transcript" },
  { value: "deferment", label: "To defer my studies" },
  { value: "change_of_course", label: "To change my course" },
  { value: "other", label: "Something else" },
];

export const requestStatus: Record<Request["status"], { label: string; tone: Status }> = {
  open: { label: "Received", tone: "info" },
  in_progress: { label: "In progress", tone: "warning" },
  done: { label: "Done", tone: "success" },
  declined: { label: "Declined", tone: "danger" },
};
