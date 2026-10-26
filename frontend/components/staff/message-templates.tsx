"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input, Textarea } from "@/components/ui/field";
import { Loading, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag, Tag } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { renderTemplate, unknownPlaceholders } from "@/lib/staff/messages";
import { useSession } from "./session";

type Template = components["schemas"]["NotificationTemplate"];

/** Editable emails, text messages and the offer letter for applicants (notifications.manage). */
export function MessageTemplates() {
  const [items, setItems] = useState<Template[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.GET("/api/v1/staff/notification-templates").then(({ data, error: err }) => {
      if (data) setItems(data.items);
      else setError(errorMessage(err));
    });
  }, []);

  return (
    <div className="flex max-w-4xl flex-col gap-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-3xl font-semibold">Messages to applicants</h1>
        <p>
          The wording of emails and text messages the website sends, and of the offer letter. Words in braces,
          like <code className="font-mono">{"{name}"}</code>, are filled in for each applicant. Changes apply
          to the next message sent.
        </p>
      </div>
      {error ? (
        <Alert tone="danger" title="Could not load the messages" live>
          {error}
        </Alert>
      ) : null}
      {!items && !error ? (
        <Loading label="Loading messages">
          <SkeletonText lines={8} />
        </Loading>
      ) : null}
      {items?.map((t) => (
        <TemplateCard
          key={`${t.key}/${t.channel}`}
          template={t}
          onSaved={(next) =>
            setItems(
              (all) => all?.map((x) => (x.key === next.key && x.channel === next.channel ? next : x)) ?? null,
            )
          }
        />
      ))}
    </div>
  );
}

function TemplateCard({ template: t, onSaved }: { template: Template; onSaved: (t: Template) => void }) {
  const { show } = useToast();
  const { user } = useSession();
  const [subject, setSubject] = useState(t.subject);
  const [body, setBody] = useState(t.body);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const id = `${t.key}-${t.channel}`.replace(/[^a-z0-9-]/g, "-");
  const email = t.channel === "email";
  const letter = t.channel === "letter";
  // Emails and letters have a subject line (a letter's heading); texts do not.
  const titled = email || letter;
  const kind = email ? "email" : letter ? "letter" : "text message";
  const changed = subject !== t.subject || body !== t.body;
  const unknown = unknownPlaceholders(`${subject} ${body}`, t.placeholders);

  async function save() {
    setBusy(true);
    setErrors({});
    const { response, error } = await api.PUT("/api/v1/staff/notification-templates/{key}/{channel}", {
      params: { path: { key: t.key, channel: t.channel } },
      body: { subject, body },
    });
    setBusy(false);
    if (response.ok) {
      onSaved({
        ...t,
        subject: titled ? subject.trim() : "",
        body: body.trim(),
        updated_at: new Date().toISOString(),
        updated_by_name: user.full_name,
      });
      show({
        tone: "success",
        title: "Message saved",
        description: `${t.name} uses the new wording from now on.`,
      });
      return;
    }
    const fields: Record<string, string> = {};
    if (isApiError(error)) for (const f of error.error.fields ?? []) fields[f.field] ??= f.message;
    if (Object.keys(fields).length === 0) fields.form = errorMessage(error);
    setErrors(fields);
  }

  return (
    <Card className="flex flex-col gap-4">
      <section aria-labelledby={`${id}-title`} className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <h2 id={`${id}-title`} className="text-xl font-semibold text-navy-900">
            {t.name}
          </h2>
          <Tag tone="navy">{email ? "Email" : letter ? "Printed letter (PDF)" : "Text message"}</Tag>
          {t.sending || letter ? null : <StatusTag status="warning">Not sent yet: no SMS provider</StatusTag>}
        </div>
        <p className="text-ink-muted">{t.description}</p>
        <form
          method="post"
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          {errors.form ? (
            <Alert tone="danger" title="Not saved" live>
              {errors.form}
            </Alert>
          ) : null}
          {titled ? (
            <Input
              id={`${id}-subject`}
              name="subject"
              label={letter ? "Heading" : "Subject"}
              required
              maxLength={150}
              value={subject}
              error={errors.subject}
              onChange={(e) => setSubject(e.target.value)}
            />
          ) : null}
          <Textarea
            id={`${id}-body`}
            name="body"
            label={email ? "Message" : letter ? "Letter text" : "Text message"}
            required
            rows={titled ? 9 : 3}
            maxLength={t.max_length}
            hint={
              <>
                You can use{" "}
                {t.placeholders.map((p, i) => (
                  <span key={p}>
                    {i > 0 ? ", " : null}
                    <code className="font-mono">{`{${p}}`}</code>
                  </span>
                ))}
                .{titled ? null : ` ${body.length} of ${t.max_length} characters.`}
              </>
            }
            value={body}
            error={
              errors.body ??
              (unknown.length > 0
                ? `Remove {${unknown.join("}, {")}}: this message cannot fill it in.`
                : undefined)
            }
            onChange={(e) => setBody(e.target.value)}
          />
          <details className="rounded-control border border-stone bg-ivory p-3">
            <summary className="min-h-touch cursor-pointer content-center font-semibold text-royal">
              Preview with sample details
            </summary>
            <div className="mt-2 flex flex-col gap-2 text-sm">
              {titled ? (
                <p>
                  <strong>{letter ? "Heading:" : "Subject:"}</strong> {renderTemplate(subject)}
                </p>
              ) : null}
              <p className="whitespace-pre-wrap break-words">{renderTemplate(body)}</p>
            </div>
          </details>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm text-ink-muted">
              {t.updated_by_name
                ? `Last changed by ${t.updated_by_name}, ${formatNairobi(t.updated_at)}`
                : "Starter wording, not changed yet"}
            </p>
            <Button type="submit" variant="secondary" loading={busy} disabled={!changed}>
              Save {kind}
            </Button>
          </div>
        </form>
      </section>
    </Card>
  );
}
