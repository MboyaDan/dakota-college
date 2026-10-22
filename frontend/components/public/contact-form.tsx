"use client";

import { useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input, Select, Textarea } from "@/components/ui/field";
import { api, errorMessage, isApiError } from "@/lib/api/client";

// Contact form (brief 6.2). Sends to POST /api/v1/public/enquiries, which
// saves the question and emails the right office. Spam protection is in the
// API (D-70); this form adds the hidden honeypot field and the time the form
// was open.

const queryTypes = [
  { value: "admissions", label: "Applying and admissions" },
  { value: "programmes", label: "A programme" },
  { value: "fees", label: "Fees and payment" },
  { value: "student_life", label: "Student life" },
  { value: "alumni", label: "Alumni: transcripts and certificates" },
  { value: "other", label: "Something else" },
] as const;

type Option = { slug: string; title: string };
type Errors = Partial<
  Record<"full_name" | "email" | "phone" | "query_type" | "programme" | "message", string>
>;

export function ContactForm({
  programmes,
  fallbackContact,
}: {
  programmes: Option[];
  fallbackContact: string;
}) {
  const startedAt = useRef(0);
  const [programme, setProgramme] = useState("");
  const [queryType, setQueryType] = useState("admissions");
  const [errors, setErrors] = useState<Errors>({});
  const [failure, setFailure] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    startedAt.current = Date.now();
    // "Ask a question" on a programme page links here with ?programme=slug.
    // Read on the client so the page itself stays prerendered.
    const slug = new URLSearchParams(window.location.search).get("programme");
    /* eslint-disable react-hooks/set-state-in-effect -- runs once after hydration */
    if (slug && programmes.some((p) => p.slug === slug)) {
      setProgramme(slug);
      setQueryType("programmes");
    }
    setHydrated(true);
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [programmes]);

  async function submit(form: HTMLFormElement) {
    const data = new FormData(form);
    const value = (name: string) => String(data.get(name) ?? "").trim();
    const body = {
      full_name: value("full_name"),
      email: value("email"),
      phone: value("phone"),
      query_type: value("query_type") as (typeof queryTypes)[number]["value"],
      programme: value("programme"),
      message: value("message"),
      website: value("website"),
      elapsed_ms: Date.now() - startedAt.current,
    };

    const found: Errors = {};
    if (!body.full_name) found.full_name = "Enter your name.";
    if (!body.email && !body.phone) found.email = "Give an email address or a phone number, so we can reply.";
    if (!body.message) found.message = "Write your question.";
    // Render the messages now, so focus can move to the first field to fix.
    flushSync(() => {
      setErrors(found);
      setFailure(null);
    });
    if (Object.keys(found).length) {
      form.querySelector<HTMLElement>("[aria-invalid=true]")?.focus();
      return;
    }

    setBusy(true);
    const { error, response } = await api.POST("/api/v1/public/enquiries", { body });
    setBusy(false);
    if (response?.status === 202) {
      setSent(true);
      return;
    }
    if (isApiError(error) && error.error.fields?.length) {
      const fromApi: Errors = {};
      for (const f of error.error.fields) fromApi[f.field as keyof Errors] ??= f.message;
      setErrors(fromApi);
    }
    setFailure(
      isApiError(error) && error.error.code !== "internal_error"
        ? error.error.message
        : `${errorMessage(error)}${fallbackContact ? ` You can also reach us on ${fallbackContact}.` : ""}`,
    );
  }

  if (sent) {
    return (
      <Alert tone="success" title="Thank you, your message is on its way" live>
        <p>The college office has received your question and will reply by email or phone.</p>
      </Alert>
    );
  }

  return (
    <form
      method="post"
      noValidate
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit(e.currentTarget);
      }}
    >
      {failure ? (
        <Alert tone="danger" title="Your message was not sent" live>
          {failure}
        </Alert>
      ) : null}
      <Input
        name="full_name"
        label="Your name"
        autoComplete="name"
        required
        maxLength={120}
        error={errors.full_name}
      />
      <p className="-mb-2 text-sm text-ink-muted">Give an email address or a phone number, or both.</p>
      <Input
        name="email"
        label="Email address"
        type="email"
        autoComplete="email"
        maxLength={254}
        error={errors.email}
      />
      <Input
        name="phone"
        label="Phone number"
        type="tel"
        autoComplete="tel"
        hint="For example 0712 345 678 or +254 712 345 678."
        maxLength={20}
        error={errors.phone}
      />
      <Select
        name="query_type"
        label="What is your question about?"
        required
        value={queryType}
        onChange={(e) => setQueryType(e.target.value)}
        error={errors.query_type}
      >
        {queryTypes.map((q) => (
          <option key={q.value} value={q.value}>
            {q.label}
          </option>
        ))}
      </Select>
      {programmes.length ? (
        <Select
          name="programme"
          label="Programme"
          value={programme}
          onChange={(e) => setProgramme(e.target.value)}
          error={errors.programme}
        >
          <option value="">Not about one programme</option>
          {programmes.map((p) => (
            <option key={p.slug} value={p.slug}>
              {p.title}
            </option>
          ))}
        </Select>
      ) : null}
      <Textarea
        name="message"
        label="Your question"
        required
        maxLength={5000}
        rows={6}
        error={errors.message}
      />
      {/* Honeypot: hidden from people and screen readers; bots fill it in. */}
      <div aria-hidden className="absolute -left-[10000px] h-px w-px overflow-hidden">
        <label htmlFor="contact-website">Leave this field empty</label>
        <input id="contact-website" name="website" type="text" tabIndex={-1} autoComplete="off" />
      </div>
      <p className="text-sm text-ink-muted">
        We use your details only to answer your question. See our{" "}
        <a href="/privacy" className="text-royal underline">
          privacy notice
        </a>
        .
      </p>
      <Button type="submit" loading={busy} disabled={!hydrated} className="self-start">
        Send message
      </Button>
    </form>
  );
}
