"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button, LinkButton, buttonClasses } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select, Textarea } from "@/components/ui/field";
import { Pagination } from "@/components/ui/pagination";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { FileText } from "@/components/icons";
import { fileSize, kindLabels } from "@/components/apply/documents";
import { api, errorMessage, isApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import {
  ageOn,
  identityLabels,
  inboxStatuses,
  inboxViews,
  levelLabels,
  relationshipLabels,
  sponsorLabels,
  staffStatus,
  type AdmissionStatus,
} from "@/lib/staff/admissions";
import { cn } from "@/lib/cn";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { useSession } from "./session";

type Page = components["schemas"]["StaffApplicationPage"];
type Detail = components["schemas"]["StaffApplicationDetail"];
type Person = { id: string; full_name: string };

export function AdmissionStatusTag({ status }: { status: AdmissionStatus }) {
  return <StatusTag status={staffStatus[status].tone}>{staffStatus[status].label}</StatusTag>;
}

// ---------- inbox ----------

export function ApplicationInbox() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [page, setPage] = useState<Page | null>(null);
  const [error, setError] = useState<string | null>(null);

  const status = params.get("status") ?? "";
  const assigned = params.get("assigned") ?? "";
  const q = params.get("q") ?? "";
  const pageNumber = Math.max(1, Number(params.get("page")) || 1);

  const hrefWith = useCallback(
    (changes: Record<string, string>, replace = false) => {
      const next = new URLSearchParams(replace ? "" : params.toString());
      for (const [k, v] of Object.entries(changes)) {
        if (v) next.set(k, v);
        else next.delete(k);
      }
      if (!("page" in changes)) next.delete("page");
      const s = next.toString();
      return s ? `${pathname}?${s}` : pathname;
    },
    [params, pathname],
  );

  useEffect(() => {
    let active = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- show the loading state for each new query
    setPage(null);
    api
      .GET("/api/v1/staff/applications", {
        params: {
          query: {
            status: (status || undefined) as Exclude<AdmissionStatus, "draft"> | undefined,
            assigned: assigned || undefined,
            q: q || undefined,
            page: pageNumber,
          },
        },
      })
      .then(({ data, error: err }) => {
        if (!active) return;
        if (data) {
          setPage(data);
          setError(null);
        } else setError(errorMessage(err));
      });
    return () => {
      active = false;
    };
  }, [status, assigned, q, pageNumber]);

  function search(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    router.push(
      hrefWith({
        status: String(data.get("status") ?? ""),
        assigned: String(data.get("assigned") ?? ""),
        q: String(data.get("q") ?? "").trim(),
      }),
    );
  }

  const isView = (query: Record<string, string>) =>
    (query.status ?? "") === status && (query.assigned ?? "") === assigned && !q;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-semibold">Applications</h1>
          <p className="max-w-prose text-ink-muted">
            Applications sent from the website. Drafts stay with the applicant until they submit.
          </p>
        </div>
        <Link href="/staff/applications/report" className="min-h-touch content-center text-royal underline">
          Admissions report
        </Link>
      </div>

      <nav aria-label="Application queues">
        <ul className="flex flex-wrap gap-2">
          {inboxViews.map((v) => (
            <li key={v.label}>
              <Link
                href={hrefWith(v.query, true)}
                aria-current={isView(v.query) ? "page" : undefined}
                className={cn(
                  "inline-flex min-h-touch items-center rounded-control border-2 px-4 font-semibold",
                  isView(v.query)
                    ? "border-navy-900 bg-navy-900 text-white"
                    : "border-royal text-royal hover:bg-royal/5",
                )}
              >
                {v.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      {/* noValidate: "Any" is an empty choice, which a required select would refuse. */}
      <form
        noValidate
        onSubmit={search}
        className="grid items-end gap-4 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_2fr_auto]"
        key={`${status}|${assigned}|${q}`}
      >
        <Select name="status" label="Status" defaultValue={status} required>
          <option value="">Any status</option>
          {inboxStatuses.map((s) => (
            <option key={s} value={s}>
              {staffStatus[s].label}
            </option>
          ))}
        </Select>
        <Select name="assigned" label="Assigned to" defaultValue={assigned} required>
          <option value="">Anyone</option>
          <option value="me">Me</option>
          <option value="none">Nobody yet</option>
        </Select>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="application-q" className="font-semibold">
            Search
          </label>
          <input
            id="application-q"
            name="q"
            type="search"
            defaultValue={q}
            placeholder="Reference, name, email or phone"
            className="block min-h-touch w-full rounded-control border border-ink-muted bg-white px-3 py-2"
          />
        </div>
        <button type="submit" className={buttonClasses("primary")}>
          Show
        </button>
      </form>

      {error ? (
        <Alert tone="danger" title="Could not load applications">
          {error}
        </Alert>
      ) : null}
      {!page && !error ? (
        <Loading label="Loading applications">
          <div className="flex flex-col gap-2">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-20 w-full" />
            ))}
          </div>
        </Loading>
      ) : null}
      {page && page.items.length === 0 ? (
        <EmptyState
          title="No applications here"
          description={
            status || assigned || q
              ? "Nothing matches these filters. Try another queue or status."
              : "Applications appear here once applicants submit them."
          }
        />
      ) : null}
      {page && page.items.length > 0 ? (
        <>
          <p className="text-sm text-ink-muted" role="status">
            {page.total === 1 ? "1 application" : `${page.total} applications`}
          </p>
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {page.items.map((a) => (
              <li key={a.id}>
                <Link
                  href={`/staff/applications/${a.id}`}
                  className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:gap-4"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="font-semibold text-royal">{a.applicant_name}</span>
                    <span className="text-sm break-words text-ink-muted">
                      <span className="font-mono">{a.reference}</span> · {a.programme_title}
                      {a.intake_name ? ` · ${a.intake_name}` : ""}
                    </span>
                  </span>
                  <span className="flex flex-col items-start gap-1 sm:w-48">
                    <AdmissionStatusTag status={a.status} />
                    {a.respond_by ? (
                      <span className="text-sm text-ink-muted">Reply by {formatDate(a.respond_by)}</span>
                    ) : null}
                  </span>
                  <span className="text-sm sm:w-40">{a.assignee_name ?? "Not assigned"}</span>
                  <span className="text-sm text-ink-muted sm:w-44">
                    Submitted {formatNairobi(a.submitted_at)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
          <Pagination
            page={page.page}
            totalPages={page.total_pages}
            hrefFor={(p) => hrefWith({ page: p === 1 ? "" : String(p) })}
            label="Application pages"
          />
        </>
      ) : null}
    </div>
  );
}

// ---------- one application ----------

/** Opens a private file through a five-minute link issued to this account. */
async function openLink(fetchLink: () => Promise<{ data?: { url: string }; error?: unknown }>) {
  // Open the tab now, while the click still counts, then point it at the link.
  const tab = window.open("", "_blank");
  const { data, error } = await fetchLink();
  if (!data) {
    tab?.close();
    return errorMessage(error);
  }
  if (tab) tab.location.href = data.url;
  else window.location.href = data.url;
  return null;
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-0.5 py-2 sm:grid-cols-[12rem_1fr] sm:gap-4">
      <dt className="text-sm text-ink-muted">{label}</dt>
      <dd className="break-words">{children || <span className="text-ink-muted">Not given</span>}</dd>
    </div>
  );
}

function Answers({ app }: { app: Detail }) {
  const s = app.submitted;
  if (!s) return null;
  const p = s.personal;
  const age = p.date_of_birth ? ageOn(p.date_of_birth) : null;
  return (
    <Card className="flex flex-col gap-4">
      <h2 className="text-xl font-semibold">Answers as submitted</h2>
      <p className="text-sm text-ink-muted">
        Exactly what the applicant declared on {formatNairobi(s.submitted_at)}. It never changes.
      </p>
      <section aria-labelledby="ans-personal">
        <h3 id="ans-personal" className="font-semibold text-navy-900">
          About the applicant
        </h3>
        <dl className="divide-y divide-stone">
          <Row label="Name">{[p.first_name, p.middle_name, p.last_name].filter(Boolean).join(" ")}</Row>
          <Row label="Date of birth">
            {p.date_of_birth ? `${formatDate(p.date_of_birth)}${age !== null ? ` (${age} today)` : ""}` : ""}
          </Row>
          <Row label="Identity document">
            {p.identity_document_type
              ? `${identityLabels[p.identity_document_type] ?? p.identity_document_type} ${p.identity_document}`
              : ""}
          </Row>
          <Row label="Phone">{p.phone}</Row>
          <Row label="Email">{s.email}</Row>
          <Row label="Nationality">{p.nationality}</Row>
          <Row label="County">{p.county}</Row>
        </dl>
      </section>
      {s.guardian ? (
        <section aria-labelledby="ans-guardian">
          <h3 id="ans-guardian" className="font-semibold text-navy-900">
            Parent or guardian
          </h3>
          <dl className="divide-y divide-stone">
            <Row label="Name">{s.guardian.full_name}</Row>
            <Row label="Relationship">
              {relationshipLabels[s.guardian.relationship] ?? s.guardian.relationship}
            </Row>
            <Row label="Phone">{s.guardian.phone}</Row>
            <Row label="Email">{s.guardian.email}</Row>
            <Row label="Agreement">
              {s.guardian.consent_given ? "The applicant confirmed the guardian agrees" : ""}
            </Row>
          </dl>
        </section>
      ) : null}
      <section aria-labelledby="ans-education">
        <h3 id="ans-education" className="font-semibold text-navy-900">
          Education
        </h3>
        {s.education.length === 0 ? <p className="text-ink-muted">None given.</p> : null}
        <ul className="flex flex-col gap-3">
          {s.education.map((e, i) => (
            <li key={i} className="rounded-control border border-stone p-3">
              <p className="font-semibold">
                {levelLabels[e.level] ?? e.level}: {e.institution}
              </p>
              <p className="text-sm">
                {[e.qualification, e.year_completed, e.mean_grade && `Mean grade ${e.mean_grade}`]
                  .filter(Boolean)
                  .join(" · ")}
              </p>
              {e.subjects.length > 0 ? (
                <p className="text-sm text-ink-muted">
                  {e.subjects.map((g) => `${g.subject} ${g.grade}`).join(", ")}
                </p>
              ) : null}
            </li>
          ))}
        </ul>
      </section>
      <section aria-labelledby="ans-sponsor">
        <h3 id="ans-sponsor" className="font-semibold text-navy-900">
          Who will pay
        </h3>
        <dl className="divide-y divide-stone">
          <Row label="Payer">{sponsorLabels[s.sponsor.type] ?? ""}</Row>
          {s.sponsor.type && s.sponsor.type !== "self" ? (
            <>
              <Row label="Name">{s.sponsor.name}</Row>
              <Row label="Phone">{s.sponsor.phone}</Row>
            </>
          ) : null}
        </dl>
      </section>
    </Card>
  );
}

export function ApplicationReview({ id }: { id: string }) {
  const { user, can } = useSession();
  const { show } = useToast();
  const [app, setApp] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [people, setPeople] = useState<Person[]>([]);
  const [busy, setBusy] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [linkError, setLinkError] = useState<string | null>(null);
  const [note, setNote] = useState("");
  const [message, setMessage] = useState("");
  const [resumeReason, setResumeReason] = useState("");
  const [conditions, setConditions] = useState("");
  const [respondBy, setRespondBy] = useState("");
  const [reason, setReason] = useState("");
  const review = can("admissions.review");

  useEffect(() => {
    let active = true;
    api.GET("/api/v1/staff/applications/{id}", { params: { path: { id } } }).then(({ data, error: err }) => {
      if (!active) return;
      if (data) setApp(data);
      else setError(errorMessage(err));
    });
    if (review) {
      api.GET("/api/v1/staff/applications/assignees").then(({ data }) => {
        if (active && data) setPeople(data.items);
      });
    }
    return () => {
      active = false;
    };
  }, [id, review]);

  /** Runs one workflow call; the answer is the application as it is now. */
  async function run(
    call: () => Promise<{ data?: Detail; error?: unknown; response: Response }>,
    success: string,
    /** Keeps field errors apart when two forms share a field name. */
    form = "",
  ): Promise<boolean> {
    setBusy(true);
    setFieldErrors({});
    const { data, error: err, response } = await call();
    setBusy(false);
    if (data) {
      setApp(data);
      show({ tone: "success", title: success });
      return true;
    }
    if (response.status === 422 && isApiError(err) && err.error.fields?.length) {
      const fields: Record<string, string> = {};
      for (const f of err.error.fields) fields[form + f.field] ??= f.message;
      setFieldErrors(fields);
      return false;
    }
    show({ tone: "danger", title: errorMessage(err) });
    // Someone else may have acted first: show the application as it is now.
    if (response.status === 409) {
      const fresh = await api.GET("/api/v1/staff/applications/{id}", { params: { path: { id } } });
      if (fresh.data) setApp(fresh.data);
    }
    return false;
  }
  const path = { params: { path: { id } } };

  const assign = (assignee: string | null) =>
    run(
      () => api.POST("/api/v1/staff/applications/{id}/assign", { ...path, body: { assignee_id: assignee } }),
      assignee ? "Assigned" : "Owner removed",
    );

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the application">
        {error}
      </Alert>
    );
  }
  if (!app) {
    return (
      <Loading label="Loading the application">
        <SkeletonText lines={10} />
      </Loading>
    );
  }

  const has = (a: Detail["actions"][number]) => app.actions.includes(a);
  const openRequest = app.info_requests.find((r) => !r.closed_at);

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb
        items={[{ label: "Applications", href: "/staff/applications" }, { label: app.applicant.name }]}
      />
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-3xl font-semibold break-words">{app.applicant.name}</h1>
          <AdmissionStatusTag status={app.status} />
        </div>
        <p className="text-ink-muted">
          <span className="font-mono break-all">{app.reference}</span> · {app.programme_title}
          {app.intake_name
            ? ` · ${app.intake_name}${app.intake_starts_on ? `, starting ${formatDate(app.intake_starts_on)}` : ""}`
            : ""}{" "}
          · submitted {formatNairobi(app.submitted_at)}
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="flex min-w-0 flex-col gap-6">
          {app.status === "submitted" && has("assign") ? (
            <Alert tone="info" title="Not reviewed yet">
              Assign it to someone (or to yourself) to start the review. Offers and rejections are possible
              once it is in review.
            </Alert>
          ) : null}

          {has("enrol") ? (
            <Card className="flex flex-col gap-3">
              <h2 className="text-xl font-semibold">Enrol</h2>
              <p>
                The applicant accepted the offer
                {app.offer?.accepted_at ? ` on ${formatNairobi(app.offer.accepted_at)}` : ""}. Enrolling them:
              </p>
              <ul className="list-disc pl-5">
                <li>gives them a student number and a student record;</li>
                <li>turns their applicant account into a student account and signs them out;</li>
                <li>opens their first invoice as a draft for Finance to complete.</li>
              </ul>
              <div>
                <Dialog
                  title="Enrol this applicant?"
                  description={`${app.applicant.name} becomes a student on ${app.programme_title}. This cannot be undone here.`}
                  triggerLabel="Enrol as a student"
                  triggerVariant="primary"
                  actions={
                    <>
                      <Button type="submit" variant="outline">
                        Cancel
                      </Button>
                      <Button
                        type="submit"
                        onClick={() =>
                          void run(
                            () => api.POST("/api/v1/staff/applications/{id}/enrol", path),
                            "Enrolled as a student",
                          )
                        }
                      >
                        Enrol now
                      </Button>
                    </>
                  }
                >
                  <p>They will be emailed their student number.</p>
                </Dialog>
              </div>
            </Card>
          ) : null}

          {has("offer") || has("request_info") || has("reject") || has("resume_review") ? (
            <section aria-labelledby="decide-title" className="flex flex-col gap-4">
              <h2 id="decide-title" className="text-2xl font-semibold">
                Decide
              </h2>
              <p className="text-sm text-ink-muted">
                The applicant is emailed after each decision and sees what you write here. Use notes for
                anything only staff should read.
              </p>

              {has("resume_review") && openRequest ? (
                <Card className="flex flex-col gap-3">
                  <h3 className="text-lg font-semibold">Waiting on the applicant</h3>
                  <p>
                    Asked {formatNairobi(openRequest.requested_at)} by {openRequest.requested_by_name}:
                  </p>
                  <blockquote className="border-l-4 border-stone pl-3 whitespace-pre-line">
                    {openRequest.message}
                  </blockquote>
                  <form
                    noValidate
                    className="flex flex-col gap-3"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      if (
                        await run(
                          () =>
                            api.POST("/api/v1/staff/applications/{id}/resume-review", {
                              ...path,
                              body: { reason: resumeReason },
                            }),
                          "Back in review",
                          "resume.",
                        )
                      )
                        setResumeReason("");
                    }}
                  >
                    <Textarea
                      id="resume-reason"
                      name="resume_reason"
                      label="Why are you continuing without a reply?"
                      rows={2}
                      maxLength={2000}
                      value={resumeReason}
                      error={fieldErrors["resume.reason"]}
                      hint="For example: they sent it by email. The applicant sees this."
                      onChange={(e) => setResumeReason(e.target.value)}
                    />
                    <Button type="submit" variant="outline" loading={busy} className="self-start">
                      Resume review
                    </Button>
                  </form>
                </Card>
              ) : null}

              {has("offer") ? (
                <Card className="flex flex-col gap-3">
                  <h3 className="text-lg font-semibold">Offer a place</h3>
                  <form
                    noValidate
                    className="flex flex-col gap-3"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      if (
                        await run(
                          () =>
                            api.POST("/api/v1/staff/applications/{id}/offer", {
                              ...path,
                              body: { conditions, respond_by: respondBy || undefined },
                            }),
                          "Place offered; the offer letter is ready",
                        )
                      ) {
                        setConditions("");
                        setRespondBy("");
                      }
                    }}
                  >
                    <Textarea
                      id="offer-conditions"
                      name="conditions"
                      label="Conditions of the offer"
                      rows={3}
                      maxLength={2000}
                      value={conditions}
                      error={fieldErrors.conditions}
                      hint="Printed on the offer letter. For example: bring your original KCSE certificate."
                      onChange={(e) => setConditions(e.target.value)}
                    />
                    <Input
                      id="offer-respond-by"
                      name="respond_by"
                      type="date"
                      label="Reply by"
                      value={respondBy}
                      error={fieldErrors.respond_by}
                      hint="Leave empty to use the usual period (Site settings). The offer lapses after this day."
                      onChange={(e) => setRespondBy(e.target.value)}
                    />
                    <Button type="submit" loading={busy} className="self-start">
                      Offer a place
                    </Button>
                  </form>
                </Card>
              ) : null}

              {has("request_info") ? (
                <Card className="flex flex-col gap-3">
                  <h3 className="text-lg font-semibold">Ask for more information</h3>
                  <form
                    noValidate
                    className="flex flex-col gap-3"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      if (!message.trim()) {
                        setFieldErrors({ message: "Write what the applicant needs to send." });
                        return;
                      }
                      if (
                        await run(
                          () =>
                            api.POST("/api/v1/staff/applications/{id}/request-info", {
                              ...path,
                              body: { message },
                            }),
                          "Request sent to the applicant",
                        )
                      )
                        setMessage("");
                    }}
                  >
                    <Textarea
                      id="info-message"
                      name="message"
                      label="What do you need from the applicant?"
                      required
                      rows={3}
                      maxLength={2000}
                      value={message}
                      error={fieldErrors.message}
                      hint="Emailed to them as you write it. They can reply and upload documents."
                      onChange={(e) => setMessage(e.target.value)}
                    />
                    <Button type="submit" variant="secondary" loading={busy} className="self-start">
                      Send request
                    </Button>
                  </form>
                </Card>
              ) : null}

              {has("reject") ? (
                <Card className="flex flex-col gap-3">
                  <h3 className="text-lg font-semibold">Reject</h3>
                  <form
                    noValidate
                    className="flex flex-col gap-3"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      if (!reason.trim()) {
                        setFieldErrors({ "reject.reason": "Write the reason. The applicant will read it." });
                        return;
                      }
                      if (
                        await run(
                          () =>
                            api.POST("/api/v1/staff/applications/{id}/reject", { ...path, body: { reason } }),
                          "Application rejected",
                          "reject.",
                        )
                      )
                        setReason("");
                    }}
                  >
                    <Textarea
                      id="reject-reason"
                      name="reason"
                      label="Reason"
                      required
                      rows={3}
                      maxLength={2000}
                      value={reason}
                      error={fieldErrors["reject.reason"]}
                      hint="Emailed to the applicant and shown in their application. Be clear and kind."
                      onChange={(e) => setReason(e.target.value)}
                    />
                    <Button type="submit" variant="danger" loading={busy} className="self-start">
                      Reject application
                    </Button>
                  </form>
                </Card>
              ) : null}
            </section>
          ) : null}

          {app.student ? (
            <Card className="flex flex-col gap-2">
              <h2 className="text-xl font-semibold">Student</h2>
              <p>
                Student number <span className="font-mono font-semibold">{app.student.student_number}</span>,
                enrolled {formatNairobi(app.student.enrolled_at)}.
              </p>
              <p className="text-sm text-ink-muted">
                First invoice:{" "}
                {app.student.invoice_status === "draft"
                  ? "a draft, waiting for Finance to add the fees and issue it."
                  : app.student.invoice_status}
              </p>
            </Card>
          ) : null}

          {app.offer ? (
            <Card className="flex flex-col gap-2">
              <h2 className="text-xl font-semibold">Offer</h2>
              <p>
                Offered by {app.offer.issued_by_name}, {formatNairobi(app.offer.issued_at)}. Reply by{" "}
                {formatDate(app.offer.respond_by)}.
              </p>
              {app.offer.accepted_at ? <p>Accepted {formatNairobi(app.offer.accepted_at)}.</p> : null}
              {app.offer.declined_at ? <p>Declined {formatNairobi(app.offer.declined_at)}.</p> : null}
              {app.offer.conditions ? (
                <p className="whitespace-pre-line">
                  <strong>Conditions:</strong> {app.offer.conditions}
                </p>
              ) : null}
              {app.offer.has_letter ? (
                <Button
                  variant="outline"
                  className="self-start"
                  icon={<FileText aria-hidden className="size-5" />}
                  onClick={async () =>
                    setLinkError(
                      await openLink(() =>
                        api.POST("/api/v1/staff/applications/{id}/offer-letter/link", path),
                      ),
                    )
                  }
                >
                  Open the offer letter
                </Button>
              ) : null}
            </Card>
          ) : null}

          {linkError ? (
            <Alert tone="danger" title="Could not open the file" live>
              {linkError}
            </Alert>
          ) : null}

          <Answers app={app} />

          <Card className="flex flex-col gap-3">
            <h2 className="text-xl font-semibold">Documents</h2>
            {app.documents.length === 0 ? <p>No documents.</p> : null}
            <ul className="flex flex-col gap-2">
              {app.documents.map((d) => (
                <li
                  key={d.id}
                  className="flex flex-col gap-1 rounded-control border border-stone px-3 py-2 sm:flex-row sm:items-center sm:gap-3"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="font-semibold">{kindLabels[d.kind]}</span>
                    <span className="text-sm break-words text-ink-muted">
                      {d.name} ({fileSize(d.size_bytes)})
                      {d.scan_status === "not_scanned" ? " · not virus-scanned" : ""}
                      {d.after_submission ? ` · sent later, ${formatNairobi(d.created_at)}` : ""}
                    </span>
                  </span>
                  <Button
                    variant="ghost"
                    onClick={async () =>
                      setLinkError(
                        await openLink(() =>
                          api.POST("/api/v1/staff/applications/{id}/documents/{doc}/link", {
                            params: { path: { id, doc: d.id } },
                          }),
                        ),
                      )
                    }
                  >
                    View<span className="sr-only"> {kindLabels[d.kind]}</span>
                  </Button>
                </li>
              ))}
            </ul>
          </Card>

          {app.info_requests.length > 0 ? (
            <section aria-labelledby="requests-title" className="flex flex-col gap-3">
              <h2 id="requests-title" className="text-xl font-semibold">
                Requests for information
              </h2>
              <ul className="flex flex-col gap-2">
                {app.info_requests.map((r) => (
                  <li
                    key={r.id}
                    className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4"
                  >
                    <p className="text-sm text-ink-muted">
                      {r.requested_by_name}, {formatNairobi(r.requested_at)}
                    </p>
                    <p className="whitespace-pre-line">{r.message}</p>
                    {r.closed_at ? (
                      r.response ? (
                        <p className="border-l-4 border-gold pl-3 whitespace-pre-line">
                          <strong>Reply ({formatNairobi(r.closed_at)}):</strong> {r.response}
                        </p>
                      ) : (
                        <p className="text-sm text-ink-muted">
                          Closed without a reply by {r.closed_by_name ?? "staff"},{" "}
                          {formatNairobi(r.closed_at)}.
                        </p>
                      )
                    ) : (
                      <p className="text-sm font-semibold">Waiting for a reply.</p>
                    )}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <section aria-labelledby="notes-title" className="flex flex-col gap-3">
            <h2 id="notes-title" className="text-xl font-semibold">
              Internal notes
            </h2>
            <p className="text-sm text-ink-muted">Only staff can see notes. They cannot be edited later.</p>
            {app.notes.length ? (
              <ul className="flex flex-col gap-2">
                {app.notes.map((n) => (
                  <li key={n.id} className="rounded-card border border-stone bg-white p-4">
                    <p className="whitespace-pre-line">{n.body}</p>
                    <p className="mt-2 text-sm text-ink-muted">
                      {n.author_name}, {formatNairobi(n.created_at)}
                    </p>
                  </li>
                ))}
              </ul>
            ) : (
              <p>No notes yet.</p>
            )}
            {has("note") ? (
              <form
                noValidate
                className="flex flex-col gap-3"
                onSubmit={async (e) => {
                  e.preventDefault();
                  if (!note.trim()) {
                    setFieldErrors({ body: "Write the note before saving it." });
                    return;
                  }
                  if (
                    await run(
                      () =>
                        api.POST("/api/v1/staff/applications/{id}/notes", { ...path, body: { body: note } }),
                      "Note added",
                    )
                  )
                    setNote("");
                }}
              >
                <Textarea
                  id="application-note"
                  name="note"
                  label="Add a note"
                  required
                  rows={3}
                  maxLength={5000}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  error={fieldErrors.body}
                  hint="For example what you checked, or who you called."
                />
                <Button type="submit" variant="secondary" loading={busy} className="self-start">
                  Save note
                </Button>
              </form>
            ) : null}
          </section>
        </div>

        <div className="flex min-w-0 flex-col gap-6">
          <Card>
            <h2 className="text-xl font-semibold">Applicant</h2>
            <dl className="flex flex-col gap-2">
              <div>
                <dt className="text-sm text-ink-muted">Email</dt>
                <dd className="break-all">
                  <a href={`mailto:${app.applicant.email}`} className="text-royal underline">
                    {app.applicant.email}
                  </a>
                </dd>
              </div>
              {app.applicant.phone ? (
                <div>
                  <dt className="text-sm text-ink-muted">Phone</dt>
                  <dd>
                    <a
                      href={`tel:${app.applicant.phone.replace(/[^\d+]/g, "")}`}
                      className="text-royal underline"
                    >
                      {app.applicant.phone}
                    </a>
                  </dd>
                </div>
              ) : null}
            </dl>
          </Card>

          <Card>
            <h2 className="text-xl font-semibold">Reviewer</h2>
            <p>{app.assignee_name ?? "Not assigned yet."}</p>
            {has("assign") ? (
              <form
                noValidate
                className="flex flex-col gap-3"
                onSubmit={(e) => {
                  e.preventDefault();
                  const v = String(new FormData(e.currentTarget).get("assignee") ?? "");
                  void assign(v || null);
                }}
              >
                <Select
                  id="application-assignee"
                  name="assignee"
                  label="Assign to"
                  defaultValue={app.assigned_to ?? ""}
                  key={app.assigned_to ?? "none"}
                  required
                >
                  <option value="">Nobody</option>
                  {people.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.full_name}
                    </option>
                  ))}
                </Select>
                <div className="flex flex-wrap gap-2">
                  <Button type="submit" variant="secondary" loading={busy}>
                    Assign
                  </Button>
                  {app.assigned_to !== user.id && people.some((p) => p.id === user.id) ? (
                    <Button variant="outline" disabled={busy} onClick={() => void assign(user.id)}>
                      Assign to me
                    </Button>
                  ) : null}
                </div>
              </form>
            ) : null}
          </Card>

          <section aria-labelledby="history-title" className="flex flex-col gap-2">
            <h2 id="history-title" className="text-xl font-semibold">
              History
            </h2>
            <ol className="flex flex-col gap-3 border-l-2 border-stone pl-4 text-sm">
              {app.history.map((h, i) => (
                <li key={i} className="flex flex-col">
                  <span className="font-semibold">{staffStatus[h.to].label}</span>
                  <span className="text-ink-muted">
                    {formatNairobi(h.at)} · {h.changed_by_name ?? "the applicant"}
                  </span>
                  {h.reason ? <span className="break-words">{h.reason}</span> : null}
                </li>
              ))}
            </ol>
          </section>

          <LinkButton href="/staff/applications" variant="outline">
            Back to applications
          </LinkButton>
        </div>
      </div>
    </div>
  );
}
