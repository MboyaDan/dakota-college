"use client";

import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { Select, Textarea } from "@/components/ui/field";
import { FileText } from "@/components/icons";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { DocumentList, UploadButton, kindLabels, uploadDocument } from "./documents";
import type { Detail } from "./step-kit";

// What the applicant can do once the admissions office has acted (Stage
// 10b): send more information, reply to an offer, or withdraw.

type Kind = components["schemas"]["ApplicationDocument"]["kind"];
type Result = { data?: Detail; error?: unknown; response: Response };
type PanelProps = { id: string; detail: Detail; onChange: (d: Detail) => void; reload: () => Promise<void> };

/** Runs an applicant action; on a conflict (the status moved on) shows the latest. */
function useAction({ onChange, reload }: Pick<PanelProps, "onChange" | "reload">) {
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<{ message: string; fields: Record<string, string> } | null>(null);
  async function run(call: () => Promise<Result>): Promise<boolean> {
    setBusy(true);
    setProblem(null);
    const { data, error, response } = await call();
    setBusy(false);
    if (data) {
      onChange(data);
      return true;
    }
    const fields: Record<string, string> = {};
    if (isApiError(error)) for (const f of error.error.fields ?? []) fields[f.field] ??= f.message;
    setProblem({ message: errorMessage(error), fields });
    if (response.status === 409) await reload();
    return false;
  }
  return { busy, problem, run };
}

const extraKinds: Kind[] = [
  "kcse_certificate",
  "other_certificate",
  "national_id",
  "birth_certificate",
  "passport",
  "passport_photo",
  "other",
];

export function InfoRequestPanel({ id, detail, onChange, reload }: PanelProps) {
  const request = detail.info_request;
  const [reply, setReply] = useState("");
  const [kind, setKind] = useState<Kind>("other_certificate");
  const [uploading, setUploading] = useState(false);
  const [uploadMessage, setUploadMessage] = useState<{ tone: "success" | "danger"; text: string } | null>(
    null,
  );
  const [replyError, setReplyError] = useState<string | undefined>();
  const { busy, problem, run } = useAction({ onChange, reload });
  if (!request) return null;
  // Documents sent before the request are part of the record and stay.
  const added = detail.documents.filter((d) => Date.parse(d.created_at) >= Date.parse(request.requested_at));

  async function upload(file: File) {
    setUploadMessage(null);
    setUploading(true);
    const failed = await uploadDocument(id, kind, file);
    setUploading(false);
    if (failed) {
      setUploadMessage({ tone: "danger", text: failed });
      return;
    }
    setUploadMessage({ tone: "success", text: `${kindLabels[kind]} uploaded.` });
    await reload();
  }

  async function remove(docId: string, name: string) {
    setUploadMessage(null);
    const { data, error } = await api.DELETE("/api/v1/applicant/applications/{id}/documents/{doc}", {
      params: { path: { id, doc: docId } },
    });
    if (data) {
      onChange(data);
      setUploadMessage({ tone: "success", text: `${name} removed.` });
    } else setUploadMessage({ tone: "danger", text: errorMessage(error) });
  }

  return (
    <Card className="flex flex-col gap-4 border-2 border-gold">
      <h2 className="text-2xl font-semibold text-navy-900">The admissions office needs more from you</h2>
      <p className="text-sm text-ink-muted">Asked {formatNairobi(request.requested_at)}</p>
      <blockquote className="border-l-4 border-gold pl-3 whitespace-pre-line">{request.message}</blockquote>

      <section aria-labelledby="more-docs-title" className="flex flex-col gap-3">
        <h3 id="more-docs-title" className="text-lg font-semibold text-navy-900">
          Upload documents (if asked)
        </h3>
        <div role="status" className="min-h-6">
          {uploadMessage ? (
            <Alert
              tone={uploadMessage.tone}
              title={uploadMessage.tone === "success" ? "Done" : "Not uploaded"}
            >
              {uploadMessage.text}
            </Alert>
          ) : null}
        </div>
        <DocumentList appId={id} docs={added} onRemove={(d) => void remove(d.id, d.name)} />
        <Select
          id="more-kind"
          name="more_kind"
          label="What is it?"
          required
          value={kind}
          onChange={(e) => setKind(e.target.value as Kind)}
        >
          {extraKinds.map((k) => (
            <option key={k} value={k}>
              {kindLabels[k]}
            </option>
          ))}
        </Select>
        <UploadButton
          id="upload-more"
          label="Upload a document"
          busy={uploading}
          onFile={(f) => void upload(f)}
        />
        {uploading ? <p className="text-sm text-ink-muted">Uploading…</p> : null}
        <p className="text-sm text-ink-muted">PDF, JPG or PNG, up to 10 MB each.</p>
      </section>

      <form
        method="post"
        noValidate
        className="flex flex-col gap-3"
        onSubmit={async (e) => {
          e.preventDefault();
          if (!reply.trim()) {
            setReplyError("Write your reply before sending it.");
            return;
          }
          setReplyError(undefined);
          if (
            await run(() =>
              api.POST("/api/v1/applicant/applications/{id}/reply", {
                params: { path: { id } },
                body: { message: reply },
              }),
            )
          )
            setReply("");
        }}
      >
        <Textarea
          id="info-reply"
          name="reply"
          label="Your reply"
          required
          rows={4}
          maxLength={2000}
          value={reply}
          error={replyError ?? problem?.fields.message}
          hint="Say what you have sent or answer the question. When you send it, your application goes back for review."
          onChange={(e) => setReply(e.target.value)}
        />
        {problem && !problem.fields.message ? (
          <Alert tone="danger" title="Not sent" live>
            {problem.message}
          </Alert>
        ) : null}
        <Button type="submit" loading={busy} className="self-start">
          Send reply
        </Button>
      </form>
    </Card>
  );
}

async function openLetter(id: string): Promise<string | null> {
  // Open the tab now, while the click still counts, then point it at the link.
  const tab = window.open("", "_blank");
  const { data, error } = await api.POST("/api/v1/applicant/applications/{id}/offer-letter/link", {
    params: { path: { id } },
  });
  if (!data) {
    tab?.close();
    return errorMessage(error);
  }
  if (tab) tab.location.href = data.url;
  else window.location.href = data.url;
  return null;
}

export function OfferPanel({ id, detail, onChange, reload }: PanelProps) {
  const offer = detail.offer;
  const [reason, setReason] = useState("");
  const [letterError, setLetterError] = useState<string | null>(null);
  const { busy, problem, run } = useAction({ onChange, reload });
  if (!offer) return null;
  const open = detail.status === "offered";

  return (
    <Card className={open ? "flex flex-col gap-4 border-2 border-gold" : "flex flex-col gap-3"}>
      <h2 className="text-2xl font-semibold text-navy-900">
        {open ? "You have been offered a place" : "Your offer"}
      </h2>
      <p>
        {detail.programme_title}
        {detail.intake_name ? `, ${detail.intake_name} intake` : ""}
        {detail.intake_starts_on ? `, starting ${formatDate(detail.intake_starts_on)}` : ""}.
      </p>
      {open ? (
        <p>
          <strong>Please reply by {formatDate(offer.respond_by)}.</strong> After that day the offer lapses.
        </p>
      ) : null}
      {offer.accepted_at ? <p>You accepted on {formatNairobi(offer.accepted_at)}.</p> : null}
      {offer.declined_at ? <p>You declined on {formatNairobi(offer.declined_at)}.</p> : null}
      {offer.conditions ? (
        <div>
          <h3 className="font-semibold text-navy-900">Conditions of the offer</h3>
          <p className="whitespace-pre-line">{offer.conditions}</p>
        </div>
      ) : null}
      {offer.has_letter ? (
        <Button
          variant="outline"
          className="self-start"
          icon={<FileText aria-hidden className="size-5" />}
          onClick={async () => setLetterError(await openLetter(id))}
        >
          Open your offer letter (PDF)
        </Button>
      ) : null}
      {letterError ? (
        <Alert tone="danger" title="Could not open the letter" live>
          {letterError}
        </Alert>
      ) : null}
      {problem ? (
        <Alert tone="danger" title="Not done" live>
          {problem.message}
        </Alert>
      ) : null}
      {open ? (
        <div className="flex flex-wrap gap-3">
          <Dialog
            title="Accept this offer?"
            description={`You accept a place on ${detail.programme_title}. The registrar will then enrol you and email your student number.`}
            triggerLabel="Accept offer"
            triggerVariant="primary"
            actions={
              <>
                <Button type="submit" variant="outline">
                  Not yet
                </Button>
                <Button
                  type="submit"
                  loading={busy}
                  onClick={() =>
                    void run(() =>
                      api.POST("/api/v1/applicant/applications/{id}/accept-offer", {
                        params: { path: { id } },
                      }),
                    )
                  }
                >
                  Yes, accept
                </Button>
              </>
            }
          >
            <p>Read your offer letter and its conditions first.</p>
          </Dialog>
          <Dialog
            title="Decline this offer?"
            description="Your application ends and the place goes to someone else. You cannot undo this."
            triggerLabel="Decline offer"
            triggerVariant="outline"
            actions={
              <>
                <Button type="submit" variant="outline">
                  Keep the offer
                </Button>
                <Button
                  type="submit"
                  variant="danger"
                  onClick={() =>
                    void run(() =>
                      api.POST("/api/v1/applicant/applications/{id}/withdraw", {
                        params: { path: { id } },
                        body: { reason },
                      }),
                    )
                  }
                >
                  Decline offer
                </Button>
              </>
            }
          >
            <Textarea
              id="decline-reason"
              name="decline_reason"
              label="Why are you declining?"
              rows={3}
              maxLength={2000}
              value={reason}
              hint="This helps the college. The admissions office will see it."
              onChange={(e) => setReason(e.target.value)}
            />
          </Dialog>
        </div>
      ) : null}
    </Card>
  );
}

/** Withdrawing, for any status before enrolment except an open offer (declined above). */
export function WithdrawPanel({ id, detail, onChange, reload }: PanelProps) {
  const [reason, setReason] = useState("");
  const { problem, run } = useAction({ onChange, reload });
  if (!detail.can_withdraw || detail.status === "offered") return null;
  return (
    <section aria-labelledby="withdraw-title" className="flex flex-col gap-2 border-t border-stone pt-5">
      <h2 id="withdraw-title" className="text-lg font-semibold text-navy-900">
        Changed your mind?
      </h2>
      <p className="text-sm">You can withdraw this application. You can apply again later.</p>
      {problem ? (
        <Alert tone="danger" title="Not withdrawn" live>
          {problem.message}
        </Alert>
      ) : null}
      <div>
        <Dialog
          title="Withdraw your application?"
          description="The admissions office stops working on it. You cannot undo this."
          triggerLabel="Withdraw application"
          triggerVariant="outline"
          actions={
            <>
              <Button type="submit" variant="outline">
                Keep my application
              </Button>
              <Button
                type="submit"
                variant="danger"
                onClick={() =>
                  void run(() =>
                    api.POST("/api/v1/applicant/applications/{id}/withdraw", {
                      params: { path: { id } },
                      body: { reason },
                    }),
                  )
                }
              >
                Withdraw
              </Button>
            </>
          }
        >
          <Textarea
            id="withdraw-reason"
            name="withdraw_reason"
            label="Why are you withdrawing?"
            rows={3}
            maxLength={2000}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </Dialog>
      </div>
    </section>
  );
}
