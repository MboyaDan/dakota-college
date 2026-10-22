"use client";

import { useEffect, useRef, useState } from "react";
import { CircleAlert } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

type ImageFile = components["schemas"]["ImageFile"];

type Props = {
  id: string;
  label: string;
  hint?: string;
  error?: string;
  value: string | null;
  onChange: (fileId: string | null) => void;
};

/** Uploads an image and shows a preview. The stored value is the file id. */
export function ImageField({ id, label, hint, error, value, onChange }: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [image, setImage] = useState<ImageFile | null>(null);
  const [busy, setBusy] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  // Load details of an image chosen earlier; state is set only when the
  // request answers, and a stale answer for another id is ignored.
  useEffect(() => {
    if (!value || image?.id === value) return;
    let active = true;
    api.GET("/api/v1/staff/images/{id}", { params: { path: { id: value } } }).then(({ data }) => {
      if (active && data) setImage(data);
    });
    return () => {
      active = false;
    };
  }, [value, image?.id]);

  async function upload(file: File) {
    setBusy(true);
    setUploadError(null);
    const body = new FormData();
    body.append("file", file);
    const { data, error: err } = await api.POST("/api/v1/staff/images", {
      // openapi-fetch sends FormData as is; the browser sets the boundary.
      body: body as unknown as { file: string },
      bodySerializer: (b) => b as unknown as FormData,
    });
    setBusy(false);
    if (data) {
      setImage(data);
      onChange(data.id);
    } else {
      setUploadError(errorMessage(err));
    }
    if (input.current) input.current.value = "";
  }

  const shown = value && image?.id === value ? image : null;
  const preview = shown?.variants[0]?.url ?? shown?.url;
  const describedBy =
    [hint ? `${id}-hint` : null, error || uploadError ? `${id}-error` : null].filter(Boolean).join(" ") ||
    undefined;

  return (
    <div className="flex flex-col gap-2">
      <label htmlFor={id} className="font-semibold">
        {label}
      </label>
      {hint ? (
        <p id={`${id}-hint`} className="text-sm text-ink-muted">
          {hint}
        </p>
      ) : null}
      {preview ? (
        // eslint-disable-next-line @next/next/no-img-element -- staff preview of an uploaded file
        <img
          src={preview}
          alt=""
          className="max-h-48 w-auto self-start rounded-control border border-stone"
        />
      ) : null}
      <input
        ref={input}
        id={id}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        aria-describedby={describedBy}
        aria-invalid={error || uploadError ? true : undefined}
        disabled={busy}
        className="min-h-touch text-sm file:mr-3 file:min-h-touch file:rounded-control file:border-2 file:border-royal file:bg-white file:px-4 file:font-semibold file:text-royal"
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) void upload(f);
        }}
      />
      {busy ? <p role="status">Uploading…</p> : null}
      {value ? (
        <Button variant="ghost" className="self-start px-0" onClick={() => onChange(null)}>
          Remove photo
        </Button>
      ) : null}
      {error || uploadError ? (
        <p id={`${id}-error`} className="flex items-start gap-1.5 text-sm font-medium text-danger">
          <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
          {uploadError ?? error}
        </p>
      ) : null}
    </div>
  );
}
