"use client";

import Link from "next/link";
import { useState, type ChangeEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/field";
import { StatusTag } from "@/components/ui/tag";
import { FileText } from "@/components/icons";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { StepButtons, type Detail, type StepProps } from "./step-kit";

type Doc = components["schemas"]["ApplicationDocument"];
type Kind = Doc["kind"];

export const MAX_DOCUMENT_BYTES = 10 * 1024 * 1024;
const ACCEPT = "application/pdf,image/jpeg,image/png,.pdf,.jpg,.jpeg,.png";

export const kindLabels: Record<Kind, string> = {
  national_id: "National ID card",
  birth_certificate: "Birth certificate",
  passport: "Passport",
  kcse_certificate: "KCSE certificate or result slip",
  other_certificate: "Other certificate",
  passport_photo: "Passport-size photo",
  other: "Other document",
};

const kindHints: Partial<Record<Kind, string>> = {
  national_id: "Both sides, in one PDF or two photos.",
  birth_certificate: "The whole certificate, flat and in good light.",
  passport: "The page with your photo.",
  kcse_certificate: "If you do not have the certificate yet, upload the result slip.",
  passport_photo: "A recent photo of your face against a plain background.",
};

export function fileSize(bytes: number): string {
  return bytes < 1024 * 1024
    ? `${Math.max(1, Math.round(bytes / 1024))} KB`
    : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** Opens a document through a five-minute link issued to this account. */
async function openDocument(appId: string, docId: string): Promise<string | null> {
  // Open the tab now, while the click still counts, then point it at the link.
  const tab = window.open("", "_blank");
  const { data, error } = await api.POST("/api/v1/applicant/applications/{id}/documents/{doc}/link", {
    params: { path: { id: appId, doc: docId } },
  });
  if (!data) {
    tab?.close();
    return errorMessage(error);
  }
  if (tab) tab.location.href = data.url;
  else window.location.href = data.url;
  return null;
}

/** Uploaded documents with View (and Remove while the draft can change). */
export function DocumentList({
  appId,
  docs,
  onRemove,
}: {
  appId: string;
  docs: Doc[];
  onRemove?: (doc: Doc) => void;
}) {
  const [error, setError] = useState<string | null>(null);
  if (docs.length === 0) return null;
  return (
    <>
      {error ? (
        <Alert tone="danger" title="Could not open the document" live>
          {error}
        </Alert>
      ) : null}
      <ul className="flex flex-col gap-2">
        {docs.map((d) => (
          <li
            key={d.id}
            className="flex flex-col gap-1 rounded-control border border-stone bg-white px-3 py-2 sm:flex-row sm:items-center sm:gap-3"
          >
            <span className="flex min-w-0 flex-1 items-start gap-2">
              <FileText aria-hidden className="mt-0.5 size-5 shrink-0 text-ink-muted" />
              <span className="min-w-0 break-words">
                {d.name}{" "}
                <span className="text-sm whitespace-nowrap text-ink-muted">({fileSize(d.size_bytes)})</span>
              </span>
            </span>
            <span className="flex gap-2">
              <Button
                type="button"
                variant="ghost"
                onClick={async () => setError(await openDocument(appId, d.id))}
              >
                View<span className="sr-only"> {d.name}</span>
              </Button>
              {onRemove ? (
                <Button type="button" variant="ghost" onClick={() => onRemove(d)}>
                  Remove<span className="sr-only"> {d.name}</span>
                </Button>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
    </>
  );
}

/** Uploads one document; on failure, says what went wrong in plain words. */
export async function uploadDocument(appId: string, kind: Kind, file: File): Promise<string | null> {
  if (file.size > MAX_DOCUMENT_BYTES) {
    return `${file.name} is larger than 10 MB. Take a smaller photo or save the PDF at a lower quality.`;
  }
  const form = new FormData();
  form.set("kind", kind);
  form.set("file", file);
  const { data, error } = await api.POST("/api/v1/applicant/applications/{id}/documents", {
    params: { path: { id: appId } },
    body: form as never,
    bodySerializer: (b) => b as unknown as FormData,
  });
  if (data) return null;
  const field = isApiError(error) ? error.error.fields?.[0]?.message : undefined;
  return `${file.name}: ${field ?? errorMessage(error)}`;
}

/** A file picker that uploads as soon as a file is chosen. */
export function UploadButton({
  id,
  label,
  busy,
  onFile,
}: {
  id: string;
  label: string;
  busy: boolean;
  onFile: (f: File) => void;
}) {
  return (
    <div>
      <label htmlFor={id} className="sr-only">
        {label}
      </label>
      <input
        id={id}
        type="file"
        accept={ACCEPT}
        disabled={busy}
        onChange={(e: ChangeEvent<HTMLInputElement>) => {
          const f = e.target.files?.[0];
          e.target.value = "";
          if (f) onFile(f);
        }}
        className="block w-full text-sm file:mr-3 file:min-h-touch file:cursor-pointer file:rounded-control file:border-2 file:border-royal file:bg-white file:px-4 file:font-semibold file:text-royal hover:file:bg-royal/5 disabled:opacity-60"
      />
    </div>
  );
}

export function DocumentsStep({ id, detail, editable, setDetail, reload, next, back, goTo }: StepProps) {
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<{ tone: "success" | "danger"; text: string } | null>(null);
  const [extraKind, setExtraKind] = useState<Kind>("other_certificate");
  const required = detail.required_documents;
  const optional = detail.documents.filter((d) => !required.includes(d.kind));

  async function upload(kind: Kind, file: File) {
    setMessage(null);
    setBusy(kind);
    const problem = await uploadDocument(id, kind, file);
    setBusy(null);
    if (problem) {
      setMessage({ tone: "danger", text: problem });
      return;
    }
    setMessage({ tone: "success", text: `${kindLabels[kind]} uploaded.` });
    await reload();
  }

  async function remove(doc: Doc) {
    setMessage(null);
    const { data, error } = await api.DELETE("/api/v1/applicant/applications/{id}/documents/{doc}", {
      params: { path: { id, doc: doc.id } },
    });
    if (data) {
      setDetail(data as Detail);
      setMessage({ tone: "success", text: `${doc.name} removed.` });
    } else setMessage({ tone: "danger", text: errorMessage(error) });
  }

  return (
    <form
      method="post"
      noValidate
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        next();
      }}
    >
      <p>
        Upload a clear photo or scan of each document: PDF, JPG or PNG, up to 10 MB each. A photo taken with
        your phone is fine if every word can be read.
      </p>
      <div role="status" className="min-h-6">
        {message ? (
          <Alert tone={message.tone} title={message.tone === "success" ? "Done" : "Not uploaded"}>
            {message.text}
          </Alert>
        ) : null}
      </div>
      {!detail.personal.identity_document_type ? (
        <Alert tone="warning" title="Choose your identity document first">
          In{" "}
          <button type="button" className="text-royal underline" onClick={() => goTo("personal")}>
            About you
          </button>
          , choose whether you have a national ID card, a birth certificate or a passport. Then upload it
          here.
        </Alert>
      ) : null}

      {required.map((kind) => {
        const docs = detail.documents.filter((d) => d.kind === kind);
        return (
          <Card key={kind} className="flex flex-col gap-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-lg font-semibold text-navy-900">{kindLabels[kind]}</h3>
              {docs.length > 0 ? (
                <StatusTag status="success">Uploaded</StatusTag>
              ) : (
                <StatusTag status="warning">Needed</StatusTag>
              )}
            </div>
            {kindHints[kind] ? <p className="text-sm text-ink-muted">{kindHints[kind]}</p> : null}
            <DocumentList appId={id} docs={docs} onRemove={editable ? remove : undefined} />
            {editable ? (
              <UploadButton
                id={`upload-${kind}`}
                label={`Upload your ${kindLabels[kind].toLowerCase()}`}
                busy={busy !== null}
                onFile={(f) => void upload(kind, f)}
              />
            ) : null}
            {busy === kind ? <p className="text-sm text-ink-muted">Uploading…</p> : null}
          </Card>
        );
      })}

      <Card className="flex flex-col gap-3">
        <h3 className="text-lg font-semibold text-navy-900">Other documents</h3>
        <p className="text-sm text-ink-muted">
          Optional: other certificates or anything else that supports your application.
        </p>
        <DocumentList appId={id} docs={optional} onRemove={editable ? remove : undefined} />
        {editable ? (
          <div className="flex flex-col gap-3">
            <Select
              name="extra_kind"
              label="What is it?"
              value={extraKind}
              onChange={(e) => setExtraKind(e.target.value as Kind)}
            >
              <option value="other_certificate">{kindLabels.other_certificate}</option>
              <option value="other">{kindLabels.other}</option>
            </Select>
            <UploadButton
              id="upload-extra"
              label="Upload another document"
              busy={busy !== null}
              onFile={(f) => void upload(extraKind, f)}
            />
            {busy === extraKind ? <p className="text-sm text-ink-muted">Uploading…</p> : null}
          </div>
        ) : null}
      </Card>

      <p className="text-sm text-ink-muted">
        Only you and the admissions office can open your documents.{" "}
        <Link href="/privacy" className="text-royal underline">
          How we use your information
        </Link>
      </p>
      <StepButtons back={back} label="Continue" />
    </form>
  );
}
