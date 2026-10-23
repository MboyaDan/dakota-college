"use client";

import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import type { components } from "@/lib/api/schema";
import type { SaveOutcome, SaveState } from "@/lib/apply/autosave";
import { cn } from "@/lib/cn";

// Shared by the application form shell (wizard.tsx) and its steps (steps.tsx).

export type Detail = components["schemas"]["ApplicationDetail"];
export type StepKey = "choice" | "personal" | "guardian" | "education" | "sponsor" | "review";
export type SavePath =
  | "/api/v1/applicant/applications/{id}/intake"
  | "/api/v1/applicant/applications/{id}/personal"
  | "/api/v1/applicant/applications/{id}/guardian"
  | "/api/v1/applicant/applications/{id}/education"
  | "/api/v1/applicant/applications/{id}/sponsor";

export const stepTitles: Record<StepKey, string> = {
  choice: "Programme and intake",
  personal: "About you",
  guardian: "Parent or guardian",
  education: "Education",
  sponsor: "Fees and sponsor",
  review: "Check your answers",
};

/** What every step gets from the wizard. */
export type StepProps = {
  detail: Detail;
  editable: boolean;
  /** Saves this step's values; the wizard updates the application from the answer. */
  save: (path: SavePath, body: unknown) => Promise<SaveOutcome>;
  next: () => void;
  back?: () => void;
  goTo: (step: StepKey) => void;
};

/** Announces saving politely, and says plainly when something did not save. */
export function SaveStatus({ state, retry }: { state: SaveState; retry: () => void }) {
  let text: ReactNode = "";
  switch (state.kind) {
    case "saving":
      text = "Saving…";
      break;
    case "saved":
      text = `All changes saved at ${state.at.toLocaleTimeString("en-KE", { hour: "2-digit", minute: "2-digit" })}.`;
      break;
    case "invalid":
      text = "Not saved: check the answers marked below.";
      break;
    case "failed":
      text = (
        <>
          Not saved: {state.message}{" "}
          <button type="button" onClick={retry} className="min-h-touch font-semibold text-royal underline">
            Try again
          </button>
        </>
      );
      break;
  }
  return (
    <p
      role="status"
      className={cn(
        "min-h-6 text-sm",
        state.kind === "failed" || state.kind === "invalid" ? "font-medium text-danger" : "text-ink-muted",
      )}
    >
      {text}
    </p>
  );
}

/** The buttons at the foot of each step: one primary action. */
export function StepButtons({
  back,
  busy,
  label = "Save and continue",
}: {
  back?: () => void;
  busy?: boolean;
  label?: string;
}) {
  return (
    <div className="flex flex-col-reverse gap-3 border-t border-stone pt-5 sm:flex-row sm:justify-between">
      {back ? (
        <Button type="button" variant="outline" onClick={back}>
          Back
        </Button>
      ) : (
        <span />
      )}
      <Button type="submit" loading={busy}>
        {label}
      </Button>
    </div>
  );
}
