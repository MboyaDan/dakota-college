"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api, errorMessage, isApiError, setCsrfToken } from "@/lib/api/client";
import { formatDate } from "@/lib/staff/datetime";
import { useHydrated } from "@/lib/use-hydrated";

type Intake = { id: string; name: string; starts_on: string; application_deadline: string | null };

/**
 * Confirms the programme and intake and starts the application. Visitors
 * who are not signed in as an applicant are sent to get a sign-in link,
 * which brings them back here.
 */
export function NewApplication({
  programme,
  intakes,
  chosen,
}: {
  programme: { slug: string; title: string };
  intakes: Intake[];
  chosen?: string;
}) {
  const router = useRouter();
  const hydrated = useHydrated();
  const [ready, setReady] = useState(false);
  const [intake, setIntake] = useState(
    intakes.find((i) => i.id === chosen)?.id ?? (intakes.length === 1 ? intakes[0].id : ""),
  );
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.GET("/api/v1/auth/session").then(({ data }) => {
      if (data?.user.kind === "applicant") {
        setCsrfToken(data.csrf_token);
        setReady(true);
        return;
      }
      const q = new URLSearchParams({ programme: programme.slug });
      if (chosen) q.set("intake", chosen);
      router.replace(`/apply/start?${q}`);
    });
  }, [programme.slug, chosen, router]);

  async function start() {
    if (!intake) {
      setError("Choose when you want to start.");
      return;
    }
    setBusy(true);
    setError(null);
    const { data, error: err } = await api.POST("/api/v1/applicant/applications", {
      body: { programme: programme.slug, intake_id: intake },
    });
    if (data) {
      router.push(`/apply/application?id=${data.id}&step=personal`);
      return;
    }
    setBusy(false);
    setError(isApiError(err) && err.error.fields?.length ? err.error.fields[0].message : errorMessage(err));
  }

  if (!ready) {
    return (
      <Loading label="Checking your sign-in">
        <Skeleton className="h-40 w-full" />
      </Loading>
    );
  }
  return (
    <form
      method="post"
      className="flex flex-col gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        void start();
      }}
    >
      {error ? (
        <Alert tone="danger" title="Could not start the application" live>
          {error}
        </Alert>
      ) : null}
      <p>
        You are applying for <strong>{programme.title}</strong>.
      </p>
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 font-semibold">When do you want to start?</legend>
        {intakes.map((i) => (
          <label
            key={i.id}
            className="flex min-h-touch cursor-pointer items-start gap-3 rounded-control border border-stone bg-white p-3 has-checked:border-royal has-checked:bg-royal/5"
          >
            <input
              type="radio"
              name="intake"
              value={i.id}
              checked={intake === i.id}
              onChange={() => setIntake(i.id)}
              className="mt-1 size-5 accent-royal"
            />
            <span className="flex flex-col">
              <span className="font-semibold">{i.name}</span>
              <span className="text-sm text-ink-muted">
                Starts {formatDate(i.starts_on)}
                {i.application_deadline ? `. Apply by ${formatDate(i.application_deadline)}.` : "."}
              </span>
            </span>
          </label>
        ))}
      </fieldset>
      <p className="text-sm text-ink-muted">
        We will give you a reference number and email it to you. The form saves as you go, so you can stop and
        come back.
      </p>
      <Button type="submit" loading={busy} disabled={!hydrated} className="self-start">
        Start my application
      </Button>
    </form>
  );
}
