"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import { api, errorMessage } from "@/lib/api/client";
import { fields } from "@/lib/form-fields";
import { useHydrated } from "@/lib/use-hydrated";

/** Only student portal paths are followed after sign-in. */
export function safeStudentNext(next: string | null): string {
  return next && next.startsWith("/student") && !next.startsWith("//") && !next.startsWith("/student/login")
    ? next
    : "/student";
}

export function StudentLoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const hydrated = useHydrated();

  async function submit(form: HTMLFormElement) {
    const { student_number, password } = fields(form, "student_number", "password");
    setBusy(true);
    setError(null);
    const { data, error: err } = await api.POST("/api/v1/student/sign-in", {
      body: { student_number, password },
    });
    setBusy(false);
    if (data) router.replace(safeStudentNext(params.get("next")));
    else setError(errorMessage(err));
  }

  return (
    <form
      method="post"
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit(e.currentTarget);
      }}
    >
      {error ? (
        <Alert tone="danger" title="Could not sign in" live>
          {error}
        </Alert>
      ) : null}
      <Input
        name="student_number"
        label="Student number"
        autoComplete="username"
        autoCapitalize="characters"
        spellCheck={false}
        required
        hint="As in your enrolment email, for example DTC/2027/0001."
      />
      <Input name="password" label="Password" type="password" autoComplete="current-password" required />
      <Button type="submit" loading={busy} disabled={!hydrated}>
        Sign in
      </Button>
      <Link
        href="/student/forgot-password"
        className="inline-flex min-h-touch items-center self-start text-royal underline underline-offset-4"
      >
        Forgot your password, or never set one?
      </Link>
    </form>
  );
}

export function StudentForgotForm() {
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const hydrated = useHydrated();

  async function submit(form: HTMLFormElement) {
    const { student_number } = fields(form, "student_number");
    setBusy(true);
    setError(null);
    const { data, error: err } = await api.POST("/api/v1/student/password-reset", {
      body: { student_number },
    });
    setBusy(false);
    if (data) setMessage(data.message);
    else setError(errorMessage(err));
  }

  if (message) {
    return (
      <Alert tone="success" title="Check your email" live>
        {message}
      </Alert>
    );
  }
  return (
    <form
      method="post"
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit(e.currentTarget);
      }}
    >
      {error ? (
        <Alert tone="danger" title="Something went wrong" live>
          {error}
        </Alert>
      ) : null}
      <Input
        name="student_number"
        label="Student number"
        autoCapitalize="characters"
        spellCheck={false}
        required
      />
      <Button type="submit" loading={busy} disabled={!hydrated}>
        Email me a link
      </Button>
      <Link
        href="/student/login"
        className="inline-flex min-h-touch items-center self-start text-royal underline underline-offset-4"
      >
        Back to sign in
      </Link>
    </form>
  );
}
