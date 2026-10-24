"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/field";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import { submissionKey } from "@/lib/apply/submission-key";

/**
 * The declaration and the one submit button. Each attempt carries an
 * Idempotency-Key; a retry after a lost connection reuses it, so the
 * application is submitted once however often the button is pressed (AC-04).
 */
export function SubmitPanel({
  id,
  ready,
  back,
  reload,
}: {
  id: string;
  /** Nothing is missing. */
  ready: boolean;
  back?: () => void;
  reload: () => Promise<void>;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const [declared, setDeclared] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (!declared) {
      setError("Tick the declaration to confirm your answers are true.");
      return;
    }
    setBusy(true);
    setError(null);
    const key = submissionKey.get(id);
    const {
      data,
      error: err,
      response,
    } = await api.POST("/api/v1/applicant/applications/{id}/submit", {
      params: { path: { id }, header: { "Idempotency-Key": key } },
      body: { declaration: true },
    });
    const code = isApiError(err) ? err.error.code : "";
    if (data || code === "already_submitted") {
      submissionKey.clear(id);
      await reload();
      router.replace(`${pathname}?id=${id}`);
      return;
    }
    setBusy(false);
    // A refusal is final for this attempt; only a lost connection is retried
    // with the same key.
    if (response) submissionKey.clear(id);
    setError(
      isApiError(err) && err.error.fields?.length
        ? err.error.fields.map((f) => f.message).join(" ")
        : response
          ? errorMessage(err)
          : "We could not reach the server, so we do not know whether it was sent. Check your connection and press Submit again: it will not be sent twice.",
    );
  }

  return (
    <form
      method="post"
      noValidate
      className="flex flex-col gap-4 border-t border-stone pt-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <h3 className="text-lg font-semibold text-navy-900">Declaration</h3>
      {error ? (
        <Alert tone="danger" title="Not submitted" live>
          {error}
        </Alert>
      ) : null}
      {ready ? (
        <>
          <Checkbox
            name="declaration"
            label="The information I have given is true and complete. I have read the privacy notice, and I agree that Dakota Training College may use my details to consider this application."
            hint={
              <Link href="/privacy" target="_blank" className="text-royal underline">
                Read the privacy notice (opens in a new tab)
              </Link>
            }
            checked={declared}
            onChange={(e) => setDeclared(e.target.checked)}
          />
          <p className="text-sm text-ink-muted">After you submit, you cannot change your answers.</p>
        </>
      ) : (
        <p>Finish the answers listed above, then come back here to submit.</p>
      )}
      <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-between">
        {back ? (
          <Button type="button" variant="outline" onClick={back}>
            Back
          </Button>
        ) : (
          <span />
        )}
        {ready ? (
          <Button type="submit" loading={busy}>
            Submit application
          </Button>
        ) : null}
      </div>
    </form>
  );
}
