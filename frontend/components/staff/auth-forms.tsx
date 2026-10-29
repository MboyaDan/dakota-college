"use client";

import Image from "next/image";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import { api, errorMessage } from "@/lib/api/client";
import { fields } from "@/lib/form-fields";
import { useHydrated } from "@/lib/use-hydrated";

const portals = {
  staff: {
    name: "Staff portal",
    points: [
      "Write and publish website content",
      "Changes to live pages wait for approval",
      "Every action is recorded",
    ],
  },
  student: {
    name: "Student portal",
    points: [
      "Your timetable and units",
      "Notices from the college",
      "Letters, transcripts and other requests",
    ],
  },
};

/** Split layout for sign-in pages: brand panel and form (brief 5.1). */
export function AuthLayout({
  title,
  intro,
  children,
  portal = "staff",
}: {
  title: string;
  intro?: ReactNode;
  children: ReactNode;
  portal?: keyof typeof portals;
}) {
  return (
    <div className="grid min-h-full flex-1 lg:grid-cols-2">
      <div className="on-dark flex flex-col justify-center gap-6 bg-navy-900 p-8 text-white lg:p-16">
        <Image
          src="/brand/dakota-logo-reversed.svg"
          alt="Dakota Training College"
          width={1339}
          height={442}
          unoptimized
          priority
          className="-ml-2 h-20 w-auto self-start lg:-ml-3 lg:h-28"
        />
        <p className="font-serif text-2xl">{portals[portal].name}</p>
        <ul className="hidden list-disc flex-col gap-2 pl-5 text-white/90 lg:flex">
          {portals[portal].points.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      </div>
      <main id="main" className="flex flex-col justify-center gap-6 bg-ivory p-6 sm:p-12">
        <div className="flex w-full max-w-md flex-col gap-6">
          <h1 className="text-3xl font-semibold">{title}</h1>
          {intro}
          {children}
        </div>
      </main>
    </div>
  );
}

/** Only same-site paths are followed after sign-in, never another website. */
export function safeNext(next: string | null): string {
  return next && next.startsWith("/staff") && !next.startsWith("//") ? next : "/staff";
}

export function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const hydrated = useHydrated();

  async function submit(form: HTMLFormElement) {
    const { email, password } = fields(form, "email", "password");
    setBusy(true);
    setError(null);
    const { data, error: err } = await api.POST("/api/v1/auth/login", { body: { email, password } });
    setBusy(false);
    if (data) router.replace(safeNext(params.get("next")));
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
      <Input name="email" label="Email address" type="email" autoComplete="username" required />
      <Input name="password" label="Password" type="password" autoComplete="current-password" required />
      <Button type="submit" loading={busy} disabled={!hydrated}>
        Sign in
      </Button>
      <Link
        href="/staff/forgot-password"
        className="inline-flex min-h-touch items-center self-start text-royal underline underline-offset-4"
      >
        Forgot your password?
      </Link>
    </form>
  );
}

export function ForgotPasswordForm() {
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const hydrated = useHydrated();

  async function submit(form: HTMLFormElement) {
    const { email } = fields(form, "email");
    setBusy(true);
    setError(null);
    const { data, error: err } = await api.POST("/api/v1/auth/password-reset", { body: { email } });
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
      <Input name="email" label="Email address" type="email" autoComplete="email" required />
      <Button type="submit" loading={busy} disabled={!hydrated}>
        Send reset link
      </Button>
      <Link
        href="/staff/login"
        className="inline-flex min-h-touch items-center self-start text-royal underline underline-offset-4"
      >
        Back to sign in
      </Link>
    </form>
  );
}

export function ResetPasswordForm() {
  const params = useSearchParams();
  const token = params.get("token") ?? "";
  // Students' links say so, and lead back to the student portal (D-141).
  const home = params.get("portal") === "student" ? "/student" : "/staff";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);
  const hydrated = useHydrated();

  async function submit() {
    if (password !== confirm) {
      setError("The two passwords are different. Type the same password in both boxes.");
      return;
    }
    setBusy(true);
    setError(null);
    const { response, error: err } = await api.POST("/api/v1/auth/password-reset/confirm", {
      body: { token, password },
    });
    setBusy(false);
    if (response.ok) setDone(true);
    else setError(errorMessage(err));
  }

  if (!token) {
    return (
      <Alert tone="warning" title="This link is incomplete">
        Open the link from your email again, or{" "}
        <Link href={`${home}/forgot-password`} className="text-royal underline">
          ask for a new one
        </Link>
        .
      </Alert>
    );
  }
  if (done) {
    return (
      <Alert tone="success" title="Password changed" live>
        You can now{" "}
        <Link href={`${home}/login`} className="text-royal underline">
          sign in
        </Link>{" "}
        with your new password. You were signed out everywhere else.
      </Alert>
    );
  }
  return (
    <form
      method="post"
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      {error ? (
        <Alert tone="danger" title="Could not change the password" live>
          {error}
        </Alert>
      ) : null}
      <Input
        name="password"
        label="New password"
        type="password"
        autoComplete="new-password"
        required
        minLength={12}
        hint="At least 12 characters. A short sentence is easy to remember."
        value={password}
        onChange={(e) => setPassword(e.target.value)}
      />
      <Input
        name="confirm"
        label="Type it again"
        type="password"
        autoComplete="new-password"
        required
        value={confirm}
        onChange={(e) => setConfirm(e.target.value)}
      />
      <Button type="submit" loading={busy} disabled={!hydrated}>
        Change password
      </Button>
    </form>
  );
}
