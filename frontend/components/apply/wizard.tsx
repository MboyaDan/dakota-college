"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { Check } from "@/components/icons";
import { api, errorMessage, isApiError, setCsrfToken } from "@/lib/api/client";
import type { SaveOutcome } from "@/lib/apply/autosave";
import { applicationStatus } from "@/lib/apply/status";
import { cn } from "@/lib/cn";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { stepTitles, type Detail, type SavePath, type StepKey, type StepProps } from "./step-kit";
import { ChoiceStep, EducationStep, GuardianStep, PersonalStep, ReviewStep, SponsorStep } from "./steps";

/**
 * The online application form (brief 8, Stage 9a). One step per screen;
 * each step saves itself a moment after a change and on "Save and
 * continue", so a dropped connection or a closed tab loses little.
 */
export function ApplicationWizard() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const id = params.get("id") ?? "";
  const [detail, setDetail] = useState<Detail | null>(null);
  const [problem, setProblem] = useState<"signed-out" | "not-found" | { message: string } | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);

  const load = useCallback(async () => {
    const { data: session, response } = await api.GET("/api/v1/auth/session");
    if (!session || session.user.kind !== "applicant") {
      setProblem(
        response.status === 401 || session ? "signed-out" : { message: "We could not reach the server." },
      );
      return;
    }
    setCsrfToken(session.csrf_token);
    const res = await api.GET("/api/v1/applicant/applications/{id}", { params: { path: { id } } });
    if (res.data) setDetail(res.data);
    else if (res.response.status === 404) setProblem("not-found");
    else setProblem({ message: errorMessage(res.error) });
  }, [id]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loads once per application
    void load();
  }, [load]);

  const steps = useMemo<StepKey[]>(
    () => [
      "choice",
      "personal",
      ...(detail?.needs_guardian ? (["guardian"] as const) : []),
      "education",
      "sponsor",
      "review",
    ],
    [detail?.needs_guardian],
  );
  const requested = params.get("step") as StepKey | null;
  const current: StepKey = requested && steps.includes(requested) ? requested : "choice";
  const index = steps.indexOf(current);

  const goTo = useCallback(
    (step: StepKey) => {
      router.push(`${pathname}?id=${id}&step=${step}`);
    },
    [router, pathname, id],
  );

  // Move focus to the new step's heading, so screen readers and keyboards
  // start at the top of it.
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    heading.current?.focus();
  }, [current]);

  const save = useCallback(
    async (path: SavePath, body: unknown): Promise<SaveOutcome> => {
      // The body type depends on the path; each step builds the right one.
      const { data, error, response } = await api.PUT(path, {
        params: { path: { id } },
        body: body as never,
      });
      if (data) {
        setDetail(data);
        return { ok: true };
      }
      if (response?.status === 422 && isApiError(error)) {
        const fields: Record<string, string> = {};
        for (const f of error.error.fields ?? []) fields[f.field] ??= f.message;
        return { ok: false, fields };
      }
      if (response?.status === 409 || response?.status === 401) void load();
      return { ok: false, message: errorMessage(error) };
    },
    [id, load],
  );

  if (problem === "signed-out") {
    return (
      <Alert tone="info" title="Please sign in to continue">
        <p>
          Your session has ended, or you have not signed in on this device. Your answers so far are saved.
        </p>
        <p className="mt-3">
          <LinkButton href={`/apply/start?next=${encodeURIComponent(`/apply/application?id=${id}`)}`}>
            Email me a sign-in link
          </LinkButton>
        </p>
      </Alert>
    );
  }
  if (problem === "not-found") {
    return (
      <Alert tone="warning" title="We could not find that application">
        Open it from{" "}
        <Link href="/apply/account" className="text-royal underline">
          your account page
        </Link>
        .
      </Alert>
    );
  }
  if (problem) {
    return (
      <Alert tone="danger" title="Could not load your application" live>
        {problem.message}
      </Alert>
    );
  }
  if (!detail) {
    return (
      <Loading label="Loading your application">
        <div className="flex flex-col gap-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-64 w-full" />
        </div>
      </Loading>
    );
  }

  const props: StepProps = {
    detail,
    editable: detail.editable,
    save,
    next: () => goTo(steps[Math.min(index + 1, steps.length - 1)]),
    back: index > 0 ? () => goTo(steps[index - 1]) : undefined,
    goTo,
  };
  const st = applicationStatus[detail.status];

  return (
    <div className="flex flex-col gap-6">
      <Card className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="font-serif text-xl font-semibold text-navy-900">{detail.programme_title}</p>
          <StatusTag status={st.tone}>{st.label}</StatusTag>
        </div>
        <p>
          Reference <span className="font-mono font-semibold">{detail.reference}</span>
        </p>
        {detail.intake_name && detail.intake_starts_on ? (
          <p>
            {detail.intake_name} intake, starting {formatDate(detail.intake_starts_on)}
          </p>
        ) : null}
        {detail.draft_expires_at ? (
          <p className="text-sm text-ink-muted">
            Saved as you go. If you stop, we keep it until {formatNairobi(detail.draft_expires_at)}; each save
            moves that date on.
          </p>
        ) : null}
      </Card>

      {!detail.editable ? (
        <Alert tone="warning" title="This application can no longer be changed">
          {st.next}
        </Alert>
      ) : null}

      <StepNav steps={steps} current={current} detail={detail} id={id} />

      <section aria-labelledby="step-title" className="flex flex-col gap-5">
        <h2
          id="step-title"
          ref={heading}
          tabIndex={-1}
          className="text-2xl font-semibold text-navy-900 focus:outline-none"
        >
          <span className="block text-base font-normal text-ink-muted">
            Step {index + 1} of {steps.length}
          </span>
          {stepTitles[current]}
        </h2>
        {
          {
            choice: <ChoiceStep key="choice" {...props} />,
            personal: <PersonalStep key="personal" {...props} />,
            guardian: <GuardianStep key="guardian" {...props} />,
            education: <EducationStep key="education" {...props} />,
            sponsor: <SponsorStep key="sponsor" {...props} />,
            review: <ReviewStep key="review" {...props} />,
          }[current]
        }
      </section>
    </div>
  );
}

function StepNav({
  steps,
  current,
  detail,
  id,
}: {
  steps: StepKey[];
  current: StepKey;
  detail: Detail;
  id: string;
}) {
  const pathname = usePathname();
  const needs = (s: StepKey) => detail.missing.some((m) => m.step === s);
  return (
    <nav aria-label="Application steps">
      <ol className="flex flex-wrap gap-2">
        {steps.map((s, i) => {
          const done = s !== "review" && !needs(s);
          return (
            <li key={s}>
              <Link
                href={`${pathname}?id=${id}&step=${s}`}
                aria-current={s === current ? "step" : undefined}
                className={cn(
                  "inline-flex min-h-touch items-center gap-2 rounded-control border px-3 text-sm",
                  s === current
                    ? "border-navy-900 bg-navy-900 font-semibold text-white"
                    : "border-stone bg-white text-royal hover:bg-ivory",
                )}
              >
                {done ? <Check aria-hidden className="size-4" /> : <span aria-hidden>{i + 1}.</span>}
                {stepTitles[s]}
                {s === "review" ? null : (
                  <span className="sr-only">{done ? " (complete)" : " (not finished)"}</span>
                )}
              </Link>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
