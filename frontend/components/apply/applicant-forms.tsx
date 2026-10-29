"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button, LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { FileText } from "@/components/icons";
import { api, errorMessage, isApiError, setCsrfToken } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { applicationStatus } from "@/lib/apply/status";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { fields } from "@/lib/form-fields";
import { useHydrated } from "@/lib/use-hydrated";

// Applicant sign-in without a password (D-96): ask for a link, open it,
// confirm with one click. The link page posts the token instead of signing in
// on page load, because mail scanners open links too.

type FieldErrors = Record<string, string>;

function fieldErrors(err: unknown): FieldErrors {
  const out: FieldErrors = {};
  if (isApiError(err)) for (const f of err.error.fields ?? []) out[f.field] ??= f.message;
  return out;
}

const linkLink = "inline-flex min-h-touch items-center self-start text-royal underline underline-offset-4";

function CheckEmail({ message, again }: { message: string; again: () => void }) {
  return (
    <div className="flex flex-col gap-4">
      <Alert tone="success" title="Check your email" live>
        {message}
      </Alert>
      <button type="button" onClick={again} className={linkLink}>
        Use a different email address
      </button>
    </div>
  );
}

function useSubmit() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [message, setMessage] = useState<string | null>(null);

  async function run(call: () => Promise<{ data?: { message: string }; error?: unknown }>) {
    setBusy(true);
    setError(null);
    setErrors({});
    const { data, error: err } = await call();
    setBusy(false);
    if (data) {
      setMessage(data.message);
      return;
    }
    const fields = fieldErrors(err);
    setErrors(fields);
    if (Object.keys(fields).length === 0) setError(errorMessage(err));
  }
  return { busy, error, errors, message, run, reset: () => setMessage(null) };
}

function Form({
  onSubmit,
  error,
  children,
}: {
  onSubmit: (form: HTMLFormElement) => void;
  error: string | null;
  children: ReactNode;
}) {
  return (
    <form
      method="post"
      noValidate
      className="flex flex-col gap-5"
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        onSubmit(e.currentTarget);
      }}
    >
      {error ? (
        <Alert tone="danger" title="Something went wrong" live>
          {error}
        </Alert>
      ) : null}
      {children}
    </form>
  );
}

/** Start or continue: the applicant's email, and their name for a new account. */
/** Where to go after signing in: an explicit return path, or starting the chosen programme. */
function nextFrom(params: URLSearchParams): string | undefined {
  const next = params.get("next");
  if (next?.startsWith("/apply/") && !next.startsWith("//")) return next;
  const programme = params.get("programme");
  if (!programme) return undefined;
  const q = new URLSearchParams({ programme });
  const intake = params.get("intake");
  if (intake) q.set("intake", intake);
  return `/apply/new?${q}`;
}

export function StartForm() {
  const params = useSearchParams();
  const s = useSubmit();
  const hydrated = useHydrated();

  if (s.message) return <CheckEmail message={s.message} again={s.reset} />;
  return (
    <Form
      error={s.error}
      onSubmit={(form) => {
        const { email, full_name } = fields(form, "email", "full_name");
        void s.run(() =>
          api.POST("/api/v1/applicant/sign-in-link", {
            body: { email, full_name: full_name || undefined, next: nextFrom(params) },
          }),
        );
      }}
    >
      <Input
        name="full_name"
        label="Your name"
        autoComplete="name"
        hint="As it appears on your ID or birth certificate."
        maxLength={120}
        error={s.errors.full_name}
      />
      <Input
        name="email"
        label="Email address"
        type="email"
        inputMode="email"
        autoComplete="email"
        required
        hint="We send a sign-in link here. Use an address you can open on this phone or computer."
        error={s.errors.email}
      />
      <Button type="submit" loading={s.busy} disabled={!hydrated}>
        Email me a link
      </Button>
      <Link href="/apply/resume" className={linkLink}>
        I have a reference number
      </Link>
    </Form>
  );
}

/** Resume by reference number plus the email on the application (AC-04). */
export function ResumeForm() {
  const s = useSubmit();
  const hydrated = useHydrated();

  if (s.message) return <CheckEmail message={s.message} again={s.reset} />;
  return (
    <Form
      error={s.error}
      onSubmit={(form) => {
        const body = fields(form, "reference", "email");
        void s.run(() => api.POST("/api/v1/applicant/resume", { body }));
      }}
    >
      <Input
        name="reference"
        label="Reference number"
        autoComplete="off"
        autoCapitalize="characters"
        spellCheck={false}
        required
        hint="It looks like DTC-APP-2026-000123. It is in the email we sent when you started."
        maxLength={40}
        className="font-mono"
        error={s.errors.reference}
      />
      <Input
        name="email"
        label="Email address on the application"
        type="email"
        inputMode="email"
        autoComplete="email"
        required
        error={s.errors.email}
      />
      <Button type="submit" loading={s.busy} disabled={!hydrated}>
        Email me a link
      </Button>
      <Link href="/apply/start" className={linkLink}>
        I do not have a reference number
      </Link>
    </Form>
  );
}

/** The page the emailed link opens. One click signs in. */
export function ContinueForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [token] = useState(() => params.get("token") ?? "");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const hydrated = useHydrated();

  // Take the token out of the address bar and history once it is read.
  useEffect(() => {
    if (token) window.history.replaceState(null, "", "/apply/continue");
  }, [token]);

  async function submit() {
    setBusy(true);
    setError(null);
    const { data, error: err } = await api.POST("/api/v1/applicant/sign-in", { body: { token } });
    if (data) {
      setCsrfToken(data.csrf_token);
      router.replace(data.next || "/apply/account");
      return;
    }
    setBusy(false);
    setError(errorMessage(err));
  }

  if (!token) {
    return (
      <Alert tone="warning" title="This link is incomplete">
        Open the link from your email again, or{" "}
        <Link href="/apply/start" className="text-royal underline">
          ask for a new one
        </Link>
        .
      </Alert>
    );
  }
  return (
    <Form error={error} onSubmit={() => void submit()}>
      {error ? (
        <LinkButton href="/apply/start" variant="outline">
          Ask for a new link
        </LinkButton>
      ) : (
        <Button type="submit" loading={busy} disabled={!hydrated}>
          Continue to my application
        </Button>
      )}
    </Form>
  );
}

type Session = components["schemas"]["Session"];
type MyApplication = components["schemas"]["MyApplication"];

/** The applicant's home: who is signed in and their applications. */
export function ApplicantAccount() {
  const router = useRouter();
  const [state, setState] = useState<
    | { kind: "loading" }
    | { kind: "signed-out" }
    | { kind: "not-applicant"; session: Session }
    | { kind: "ready"; session: Session; items: MyApplication[] }
    | { kind: "error"; message: string }
  >({ kind: "loading" });

  useEffect(() => {
    let active = true;
    (async () => {
      const { data: session, response, error } = await api.GET("/api/v1/auth/session");
      if (!active) return;
      if (!session) {
        setState(
          response.status === 401 ? { kind: "signed-out" } : { kind: "error", message: errorMessage(error) },
        );
        return;
      }
      setCsrfToken(session.csrf_token);
      if (session.user.kind !== "applicant") {
        setState({ kind: "not-applicant", session });
        return;
      }
      const res = await api.GET("/api/v1/applicant/applications");
      if (!active) return;
      setState(
        res.data
          ? { kind: "ready", session, items: res.data.items }
          : { kind: "error", message: errorMessage(res.error) },
      );
    })();
    return () => {
      active = false;
    };
  }, []);

  async function signOut() {
    await api.POST("/api/v1/auth/logout");
    setCsrfToken("");
    router.replace("/apply");
  }

  switch (state.kind) {
    case "loading":
      return (
        <Loading label="Loading your applications">
          <div className="flex flex-col gap-4">
            <Skeleton className="h-8 w-1/2" />
            <Skeleton className="h-32 w-full" />
          </div>
        </Loading>
      );
    case "error":
      return (
        <Alert tone="danger" title="Could not load your applications" live>
          {state.message}
        </Alert>
      );
    case "signed-out":
      return (
        <Alert tone="info" title="Please sign in">
          <p>Your session has ended, or you have not signed in on this device.</p>
          <p className="mt-3 flex flex-wrap gap-3">
            <LinkButton href="/apply/start">Email me a sign-in link</LinkButton>
          </p>
        </Alert>
      );
    case "not-applicant":
      return (
        <Alert tone="warning" title="You are signed in with a staff account">
          <p>Signed in as {state.session.user.email}. Sign out first to use the applicant pages.</p>
          <p className="mt-3">
            <Button variant="outline" onClick={() => void signOut()}>
              Sign out
            </Button>
          </p>
        </Alert>
      );
  }

  const { session, items } = state;
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p>
          Signed in as <strong>{session.user.email}</strong>
        </p>
        <Button variant="outline" onClick={() => void signOut()}>
          Sign out
        </Button>
      </div>
      <h2 className="text-2xl font-semibold text-navy-900">Your applications</h2>
      {items.length === 0 ? (
        <EmptyState
          icon={FileText}
          title="No applications yet"
          description={
            <p>
              Find the programme you want and choose Apply on its page. Your application will appear here with
              its reference number and status.
            </p>
          }
          action={<LinkButton href="/programmes">Find a course</LinkButton>}
        />
      ) : (
        <ul className="flex flex-col gap-4">
          {items.map((a) => {
            const st = applicationStatus[a.status];
            return (
              <li key={a.id}>
                <Card className="flex flex-col gap-2">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <h3 className="text-lg font-semibold text-navy-900">{a.programme_title}</h3>
                    <StatusTag status={st.tone}>{st.label}</StatusTag>
                  </div>
                  <p>
                    Reference <span className="font-mono">{a.reference}</span>
                  </p>
                  {a.intake_name && a.intake_starts_on ? (
                    <p>
                      {a.intake_name} intake, starting {formatDate(a.intake_starts_on)}
                    </p>
                  ) : null}
                  <p className="text-sm text-ink-muted">
                    {a.submitted_at
                      ? `Submitted ${formatNairobi(a.submitted_at)}`
                      : `Started ${formatNairobi(a.created_at)}`}
                  </p>
                  <p>{st.next}</p>
                  <Link
                    href={`/apply/application?id=${a.id}`}
                    className="inline-flex min-h-touch items-center self-start font-semibold text-royal underline underline-offset-4"
                  >
                    {a.status === "draft" ? "Continue your application" : "View your application"}
                    <span className="sr-only"> {a.reference}</span>
                  </Link>
                </Card>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
