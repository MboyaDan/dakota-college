"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select, Textarea } from "@/components/ui/field";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { Table } from "@/components/ui/table";
import { StatusTag } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import {
  canExport,
  dueText,
  eventLabel,
  kindLabel,
  nairobiToday,
  requestKinds,
  requestStatus,
  resultText,
  ruleLabels,
  type RequestKind,
} from "@/lib/staff/privacy";
import { attachmentName } from "@/lib/staff/reports";
import { useAction } from "./use-action";

// Data protection requests and the retention log (Stage 17e, privacy.manage).
// The procedure staff follow is docs/privacy-requests.md.

type Summary = components["schemas"]["DataRequestSummary"];
type Request = components["schemas"]["DataRequest"];
type Matches = components["schemas"]["PrivacyMatches"];
type Overview = components["schemas"]["RetentionOverview"];
type RunResult = components["schemas"]["RetentionRunResult"];
type RuleResult = components["schemas"]["RuleResult"];

function Due({ r }: { r: Summary }) {
  if (r.status !== "open") return <>{r.closed_at ? `Closed ${formatNairobi(r.closed_at)}` : "Closed"}</>;
  const text = dueText(r.due_on, nairobiToday());
  return r.overdue ? <strong className="text-danger">{text}</strong> : <>{text}</>;
}

// ---------- register ----------

export function DataRequests() {
  const [status, setStatus] = useState("open");
  const [list, setList] = useState<{ items: Summary[]; total: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [recording, setRecording] = useState(false);

  useEffect(() => {
    let active = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- show the loading state for each new filter
    setList(null);
    api
      .GET("/api/v1/staff/privacy/requests", {
        params: { query: { status: (status || undefined) as Summary["status"] | undefined } },
      })
      .then(({ data, error: err }) => {
        if (!active) return;
        if (data) {
          setList(data);
          setError(null);
        } else setError(errorMessage(err));
      });
    return () => {
      active = false;
    };
  }, [status]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Data requests</h1>
        <p className="max-w-prose text-ink-muted">
          Requests from people to see, correct or delete the personal data the college holds about them.
          Record each one the day it arrives and answer it by its due date.
        </p>
      </div>
      {recording ? (
        <RecordRequest onCancel={() => setRecording(false)} />
      ) : (
        <Button className="self-start" onClick={() => setRecording(true)}>
          Record a request
        </Button>
      )}
      <Select
        id="request-status"
        name="status"
        label="Show"
        required
        value={status}
        onChange={(e) => setStatus(e.target.value)}
        className="sm:max-w-xs"
      >
        <option value="open">Open</option>
        <option value="completed">Completed</option>
        <option value="refused">Refused</option>
        <option value="">All</option>
      </Select>
      {error ? (
        <Alert tone="danger" title="Could not load requests">
          {error}
        </Alert>
      ) : list === null ? (
        <Loading label="Loading requests">
          <Skeleton className="h-32 w-full" />
        </Loading>
      ) : list.items.length === 0 ? (
        <EmptyState
          title={status === "open" ? "No open requests" : "No requests here"}
          description="When someone asks about their personal data, record it with the button above."
        />
      ) : (
        <>
          <Table
            caption="Data requests"
            captionHidden
            rows={list.items}
            rowKey={(r) => r.id}
            columns={[
              {
                key: "ref",
                header: "Reference",
                cell: (r) => (
                  <Link
                    href={`/staff/privacy/${r.id}`}
                    className="font-mono font-semibold text-royal underline"
                  >
                    {r.reference}
                  </Link>
                ),
              },
              { key: "kind", header: "Asks to", cell: (r) => kindLabel(r.kind) },
              { key: "from", header: "From", cell: (r) => r.requester_name },
              { key: "received", header: "Received", cell: (r) => formatDate(r.received_on) },
              { key: "due", header: "Due", cell: (r) => <Due r={r} /> },
              {
                key: "status",
                header: "Status",
                cell: (r) => (
                  <StatusTag status={requestStatus[r.status].tone}>{requestStatus[r.status].label}</StatusTag>
                ),
              },
            ]}
          />
          {list.total > list.items.length ? (
            <p className="text-sm text-ink-muted">
              Showing {list.items.length} of {list.total}.
            </p>
          ) : null}
        </>
      )}
    </div>
  );
}

function RecordRequest({ onCancel }: { onCancel: () => void }) {
  const router = useRouter();
  const { busy, errors, run } = useAction();
  const [kind, setKind] = useState<RequestKind>("access");
  const today = nairobiToday();

  return (
    <Card>
      <h2 className="text-xl font-semibold">Record a request</h2>
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={async (e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const text = (k: string) => String(f.get(k) ?? "");
          const res = await run(
            () =>
              api.POST("/api/v1/staff/privacy/requests", {
                body: {
                  kind,
                  requester_name: text("requester_name"),
                  requester_contact: text("requester_contact"),
                  acting_for: text("acting_for"),
                  subject_email: text("subject_email"),
                  subject_phone: text("subject_phone"),
                  details: text("details"),
                  received_on: text("received_on"),
                  due_on: text("due_on"),
                },
              }),
            "Request recorded",
          );
          if (res) router.push(`/staff/privacy/${res.id}`);
        }}
      >
        <Select
          id="request-kind"
          name="kind"
          label="What do they ask for?"
          required
          value={kind}
          error={errors.kind}
          hint={requestKinds.find((k) => k.value === kind)?.hint}
          onChange={(e) => setKind(e.target.value as RequestKind)}
        >
          {requestKinds.map((k) => (
            <option key={k.value} value={k.value}>
              {k.label}
            </option>
          ))}
        </Select>
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            name="requester_name"
            label="Who is asking"
            required
            maxLength={200}
            error={errors.requester_name}
          />
          <Input
            name="requester_contact"
            label="How to reach them"
            hint="Email address, phone number or postal address."
            required
            maxLength={200}
            error={errors.requester_contact}
          />
        </div>
        <Input
          name="acting_for"
          label="Asking for someone else?"
          hint='Leave empty if they ask about themselves. Otherwise say who they are, for example "Parent".'
          maxLength={200}
          error={errors.acting_for}
        />
        <fieldset className="flex flex-col gap-4">
          <legend className="mb-2 font-semibold">How to find the person&apos;s records</legend>
          <p className="text-sm text-ink-muted">
            The email address or phone number the college would hold for the person. Give at least one.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              name="subject_email"
              type="email"
              label="Email address"
              autoComplete="off"
              error={errors.subject_email}
            />
            <Input
              name="subject_phone"
              type="tel"
              label="Phone number"
              autoComplete="off"
              error={errors.subject_phone}
            />
          </div>
        </fieldset>
        <Textarea name="details" label="What they asked" rows={3} maxLength={5000} error={errors.details} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            name="received_on"
            type="date"
            label="Received on"
            defaultValue={today}
            max={today}
            required
            error={errors.received_on}
          />
          <Input
            name="due_on"
            type="date"
            label="Answer by"
            hint="Leave empty for 7 days after it was received."
            error={errors.due_on}
          />
        </div>
        <div className="flex flex-wrap gap-3">
          <Button type="submit" loading={busy}>
            Record request
          </Button>
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}

// ---------- one request ----------

export function DataRequestView({ id }: { id: string }) {
  const [r, setR] = useState<Request | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    api
      .GET("/api/v1/staff/privacy/requests/{id}", { params: { path: { id } } })
      .then(({ data, error: err }) => {
        if (!active) return;
        if (data) setR(data);
        else setError(errorMessage(err));
      });
    return () => {
      active = false;
    };
  }, [id]);

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the request">
        {error}
      </Alert>
    );
  }
  if (!r) {
    return (
      <Loading label="Loading the request">
        <SkeletonText lines={8} />
      </Loading>
    );
  }

  const open = r.status === "open";
  const checked = r.identity_checked_at !== null;
  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "Data requests", href: "/staff/privacy" }, { label: r.reference }]} />
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-mono text-2xl font-semibold sm:text-3xl">{r.reference}</h1>
        <StatusTag status={requestStatus[r.status].tone}>{requestStatus[r.status].label}</StatusTag>
      </div>
      <p className="text-ink-muted">
        {kindLabel(r.kind)} · <Due r={r} />
      </p>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="flex min-w-0 flex-col gap-6">
          {!open ? (
            <Alert
              tone={r.status === "completed" ? "success" : "warning"}
              title={`${requestStatus[r.status].label}: what they were told`}
            >
              <p className="whitespace-pre-line">{r.outcome}</p>
            </Alert>
          ) : null}
          {open && !checked ? <IdentityForm id={r.id} onDone={setR} /> : null}
          {open && checked ? <NextStep r={r} onDone={setR} /> : null}
          <RecordsFound r={r} onDone={setR} />
          {open ? <NoteForm id={r.id} onDone={setR} /> : null}
          {open ? <CloseForm id={r.id} onDone={setR} /> : null}
        </div>
        <div className="flex min-w-0 flex-col gap-6">
          <Card>
            <h2 className="text-xl font-semibold">Request</h2>
            <dl className="flex flex-col gap-2">
              <Fact term="From">{r.requester_name}</Fact>
              <Fact term="Contact">{r.requester_contact}</Fact>
              {r.acting_for ? <Fact term="Asking for">{r.acting_for}</Fact> : null}
              <Fact term="Records found by">
                {[r.subject_email, r.subject_phone].filter(Boolean).join(", ")}
              </Fact>
              {r.details ? (
                <Fact term="What they asked">
                  <span className="whitespace-pre-line">{r.details}</span>
                </Fact>
              ) : null}
              <Fact term="Received">{formatDate(r.received_on)}</Fact>
              <Fact term="Answer by">{formatDate(r.due_on)}</Fact>
              <Fact term="Identity">
                {checked
                  ? `${r.identity_method} (${r.identity_checked_by_name}, ${formatNairobi(r.identity_checked_at)})`
                  : "Not checked yet"}
              </Fact>
            </dl>
          </Card>
          <section aria-labelledby="history-title" className="flex flex-col gap-3">
            <h2 id="history-title" className="text-xl font-semibold">
              History
            </h2>
            <ol className="flex flex-col gap-2">
              {r.events.map((e, i) => (
                <li key={i} className="rounded-card border border-stone bg-white p-3">
                  <p className="font-semibold">{eventLabel(e)}</p>
                  {e.note ? <p className="whitespace-pre-line break-words">{e.note}</p> : null}
                  <p className="text-sm text-ink-muted">
                    {e.actor_name}, {formatNairobi(e.at)}
                  </p>
                </li>
              ))}
            </ol>
          </section>
        </div>
      </div>
    </div>
  );
}

function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-sm text-ink-muted">{term}</dt>
      <dd className="break-words">{children}</dd>
    </div>
  );
}

type Done = (r: Request) => void;

function IdentityForm({ id, onDone }: { id: string; onDone: Done }) {
  const { busy, errors, run } = useAction();
  return (
    <Card>
      <h2 className="text-xl font-semibold">Step 1: check who is asking</h2>
      <p>
        Before you send or delete anything, make sure the person asking is the person, or their parent or
        guardian. In person, see their ID. By phone or email, contact them back on details the college already
        holds.
      </p>
      <form
        noValidate
        className="flex flex-col gap-3"
        onSubmit={async (e) => {
          e.preventDefault();
          const method = String(new FormData(e.currentTarget).get("method") ?? "");
          const res = await run(
            () =>
              api.POST("/api/v1/staff/privacy/requests/{id}/identity", {
                params: { path: { id } },
                body: { method },
              }),
            "Identity check recorded",
          );
          if (res) onDone(res);
        }}
      >
        <Input
          name="method"
          label="How did you check?"
          hint='For example "Saw national ID at the front office" or "Called back on the number on the application".'
          required
          maxLength={500}
          error={errors.method}
        />
        <Button type="submit" variant="secondary" className="self-start" loading={busy}>
          Record identity check
        </Button>
      </form>
    </Card>
  );
}

function NextStep({ r, onDone }: { r: Request; onDone: Done }) {
  if (canExport(r.kind)) return <ExportStep r={r} onDone={onDone} />;
  if (r.kind === "erasure") {
    return (
      <Card>
        <h2 className="text-xl font-semibold">Step 2: erase what can be erased</h2>
        <p>
          Check each record below belongs to the person, then erase it. Applicants who became students, and
          applications still being decided, cannot be erased: refuse that part of the request and give the
          reason. Erasing cannot be undone.
        </p>
      </Card>
    );
  }
  return (
    <Card>
      <h2 className="text-xl font-semibold">Step 2: deal with the request</h2>
      <p>{requestKinds.find((k) => k.value === r.kind)?.hint}</p>
    </Card>
  );
}

function ExportStep({ r, onDone }: { r: Request; onDone: Done }) {
  const { show } = useToast();
  const [busy, setBusy] = useState(false);

  async function download() {
    setBusy(true);
    const { data, error, response } = await api.POST("/api/v1/staff/privacy/requests/{id}/export", {
      params: { path: { id: r.id } },
      parseAs: "blob",
    });
    if (!data) {
      setBusy(false);
      show({ tone: "danger", title: "Not downloaded", description: errorMessage(error) });
      return;
    }
    const name = attachmentName(
      response.headers.get("Content-Disposition"),
      `dakota-${r.reference.toLowerCase()}-data.json`,
    );
    const url = URL.createObjectURL(data as Blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    show({ tone: "success", title: "Downloaded", description: name });
    // The export is now in the request's history.
    const fresh = await api.GET("/api/v1/staff/privacy/requests/{id}", { params: { path: { id: r.id } } });
    setBusy(false);
    if (fresh.data) onDone(fresh.data);
  }

  return (
    <Card>
      <h2 className="text-xl font-semibold">Step 2: export their data</h2>
      <p>
        One file with every record below. Before you send it, check each record belongs to the person and
        remove anything about other people, such as another person&apos;s name in a staff note. Send it
        securely.
      </p>
      <Button variant="secondary" className="self-start" loading={busy} onClick={() => void download()}>
        Download their data (JSON)
      </Button>
    </Card>
  );
}

function RecordsFound({ r, onDone }: { r: Request; onDone: Done }) {
  const m: Matches = r.matches;
  const canErase = r.status === "open" && r.kind === "erasure" && r.identity_checked_at !== null;
  const applicants = m.applicants ?? [];
  const students = m.students ?? [];
  const enquiries = m.enquiries ?? [];
  const guardians = m.guardians ?? [];
  const staff = m.staff ?? [];
  const none =
    !applicants.length && !students.length && !enquiries.length && !guardians.length && !staff.length;

  return (
    <section aria-labelledby="records-title" className="flex flex-col gap-3">
      <h2 id="records-title" className="text-xl font-semibold">
        Records found
      </h2>
      <p className="text-sm text-ink-muted">
        Records that carry {[r.subject_email, r.subject_phone].filter(Boolean).join(" or ")}. Two people can
        share a phone number: check each one is the person&apos;s. Paper files and email are not searched.
      </p>
      {none ? (
        <p>
          No records carry these details. Tell the person the college holds no data about them in this system.
        </p>
      ) : null}
      {applicants.length ? (
        <MatchGroup title="Applications">
          {applicants.map((a) => (
            <MatchCard
              key={a.id}
              title={a.name}
              lines={[
                `${a.email}${a.phone ? ` · ${a.phone}` : ""}`,
                `${a.applications === 1 ? "1 application" : `${a.applications} applications`} · since ${formatNairobi(a.created_at)}`,
                a.is_student
                  ? "Became a student: kept with the student record"
                  : a.in_progress > 0
                    ? "An application is still being decided"
                    : "",
              ]}
              action={
                canErase && a.erasable ? (
                  <EraseButton
                    requestId={r.id}
                    type="applicant"
                    recordId={a.id}
                    name={a.name}
                    onDone={onDone}
                  />
                ) : null
              }
            />
          ))}
        </MatchGroup>
      ) : null}
      {students.length ? (
        <MatchGroup title="Student records (kept)">
          {students.map((s) => (
            <MatchCard
              key={s.id}
              title={s.name}
              mono={s.student_number}
              href={`/staff/students/${s.id}`}
              lines={[
                `${s.email}${s.phone ? ` · ${s.phone}` : ""}`,
                `Enrolled ${formatNairobi(s.enrolled_at)} · ${s.status}`,
              ]}
            />
          ))}
        </MatchGroup>
      ) : null}
      {enquiries.length ? (
        <MatchGroup title="Enquiries">
          {enquiries.map((e) => (
            <MatchCard
              key={e.id}
              title={e.name}
              href={`/staff/enquiries/${e.id}`}
              lines={[
                [e.email, e.phone].filter(Boolean).join(" · "),
                `Sent ${formatNairobi(e.created_at)} · ${e.status}`,
              ]}
              action={
                canErase ? (
                  <EraseButton
                    requestId={r.id}
                    type="enquiry"
                    recordId={e.id}
                    name={`the enquiry from ${e.name}`}
                    onDone={onDone}
                  />
                ) : null
              }
            />
          ))}
        </MatchGroup>
      ) : null}
      {guardians.length ? (
        <MatchGroup title="Named as a parent or guardian">
          {guardians.map((g, i) => (
            <MatchCard
              key={i}
              title={g.name}
              mono={g.record}
              lines={[
                [g.email, g.phone].filter(Boolean).join(" · "),
                g.source === "student" ? "On a student's record" : "On an application",
              ]}
            />
          ))}
        </MatchGroup>
      ) : null}
      {staff.length ? (
        <MatchGroup title="Staff accounts">
          {staff.map((u) => (
            <MatchCard key={u.id} title={u.name} lines={[u.email, `Account ${u.status}`]} />
          ))}
        </MatchGroup>
      ) : null}
    </section>
  );
}

function MatchGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <h3 className="font-semibold">{title}</h3>
      <ul className="flex flex-col gap-2">{children}</ul>
    </div>
  );
}

function MatchCard({
  title,
  mono,
  href,
  lines,
  action,
}: {
  title: string;
  mono?: string;
  href?: string;
  lines: string[];
  action?: ReactNode;
}) {
  const heading = (
    <>
      {title}
      {mono ? <span className="font-mono font-normal"> {mono}</span> : null}
    </>
  );
  return (
    <li className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4">
      <p className="font-semibold break-words">
        {href ? (
          <Link href={href} className="text-royal underline">
            {heading}
          </Link>
        ) : (
          heading
        )}
      </p>
      {lines.filter(Boolean).map((l) => (
        <p key={l} className="text-sm break-words">
          {l}
        </p>
      ))}
      {action ? <div className="mt-2">{action}</div> : null}
    </li>
  );
}

function EraseButton({
  requestId,
  type,
  recordId,
  name,
  onDone,
}: {
  requestId: string;
  type: "applicant" | "enquiry";
  recordId: string;
  name: string;
  onDone: Done;
}) {
  const { run } = useAction();
  return (
    <Dialog
      title={`Erase ${name}?`}
      description="This cannot be undone."
      triggerLabel={type === "applicant" ? `Erase ${name}` : "Erase this enquiry"}
      triggerVariant="outline"
      actions={
        <>
          <Button type="submit" variant="outline">
            Cancel
          </Button>
          <Button
            type="submit"
            variant="danger"
            onClick={() =>
              void run(
                () =>
                  api.POST("/api/v1/staff/privacy/requests/{id}/erase", {
                    params: { path: { id: requestId } },
                    body: { record_type: type, record_id: recordId },
                  }),
                "Erased",
              ).then((res) => {
                if (res) onDone(res);
              })
            }
          >
            Erase
          </Button>
        </>
      }
    >
      {type === "applicant" ? (
        <p>
          Their name, contact details, identity number, answers, guardians, education and documents are
          removed, and their sign-in account is closed. The application reference and outcome stay, for
          counts.
        </p>
      ) : (
        <p>The enquiry and its staff notes are deleted.</p>
      )}
    </Dialog>
  );
}

function NoteForm({ id, onDone }: { id: string; onDone: Done }) {
  const { busy, errors, run } = useAction();
  const [note, setNote] = useState("");
  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        const res = await run(
          () =>
            api.POST("/api/v1/staff/privacy/requests/{id}/notes", {
              params: { path: { id } },
              body: { note },
            }),
          "Note added",
          "note-",
        );
        if (res) {
          setNote("");
          onDone(res);
        }
      }}
    >
      <Textarea
        id="request-note"
        name="note"
        label="Add a note"
        hint="What you did, for example the correction made on the student's record. Notes cannot be edited."
        rows={2}
        maxLength={5000}
        value={note}
        error={errors["note-note"]}
        onChange={(e) => setNote(e.target.value)}
      />
      <Button type="submit" variant="outline" className="self-start" loading={busy}>
        Add note
      </Button>
    </form>
  );
}

function CloseForm({ id, onDone }: { id: string; onDone: Done }) {
  const { busy, errors, run } = useAction();
  const [status, setStatus] = useState<"completed" | "refused">("completed");
  return (
    <Card>
      <h2 className="text-xl font-semibold">Close the request</h2>
      <form
        noValidate
        className="flex flex-col gap-3"
        onSubmit={async (e) => {
          e.preventDefault();
          const outcome = String(new FormData(e.currentTarget).get("outcome") ?? "");
          const res = await run(
            () =>
              api.POST("/api/v1/staff/privacy/requests/{id}/close", {
                params: { path: { id } },
                body: { status, outcome },
              }),
            status === "completed" ? "Request completed" : "Request refused",
            "close-",
          );
          if (res) onDone(res);
        }}
      >
        <Select
          id="close-status"
          name="status"
          label="Outcome"
          required
          value={status}
          error={errors["close-status"]}
          onChange={(e) => setStatus(e.target.value as typeof status)}
        >
          <option value="completed">Completed</option>
          <option value="refused">Refused</option>
        </Select>
        <Textarea
          id="close-outcome"
          name="outcome"
          label={status === "completed" ? "What was done" : "Why it was refused"}
          hint="This is what you tell the person. Remind them they can complain to the Data Protection Commissioner."
          rows={3}
          maxLength={5000}
          required
          error={errors["close-outcome"]}
        />
        <Button type="submit" variant="secondary" className="self-start" loading={busy}>
          Close request
        </Button>
      </form>
    </Card>
  );
}

// ---------- retention ----------

const modeText: Record<
  Overview["mode"],
  { tone: "info" | "success" | "warning"; title: string; body: string }
> = {
  report: {
    tone: "info",
    title: "Report mode",
    body: "Each day the schedule removes spent sign-in links, ended sessions and similar records, and counts the personal data past its period without erasing it. Erasing starts once the college approves the schedule and the API is set to apply.",
  },
  apply: {
    tone: "success",
    title: "Applying the schedule",
    body: "Each day the schedule removes and erases everything past its period.",
  },
  off: {
    tone: "warning",
    title: "The schedule is off",
    body: "Nothing is removed or counted. Ask the developer to set RETENTION_MODE.",
  },
};

export function Retention() {
  const [o, setO] = useState<Overview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [report, setReport] = useState<RunResult | null>(null);
  const { busy, run } = useAction();

  const load = useCallback(() => {
    void api.GET("/api/v1/staff/privacy/retention").then(({ data, error: err }) => {
      if (data) setO(data);
      else setError(errorMessage(err));
    });
  }, []);
  useEffect(load, [load]);

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the retention schedule">
        {error}
      </Alert>
    );
  }
  if (!o) {
    return (
      <Loading label="Loading the retention schedule">
        <SkeletonText lines={8} />
      </Loading>
    );
  }
  const titles = new Map(o.schedule.map((s) => [s.key, s.title]));
  const ruleName = (k: string) => titles.get(k) ?? ruleLabels[k] ?? k;
  const mode = modeText[o.mode];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Retention</h1>
        <p className="max-w-prose text-ink-muted">
          How long personal data is kept, and what the daily run removed. The written schedule is in{" "}
          <span className="font-mono">docs/retention.md</span>.
        </p>
      </div>
      <Alert tone={mode.tone} title={mode.title}>
        {mode.body}
      </Alert>

      <section aria-labelledby="schedule-title" className="flex flex-col gap-3">
        <h2 id="schedule-title" className="text-xl font-semibold">
          Schedule
        </h2>
        <ul className="grid gap-3 md:grid-cols-2">
          {o.schedule.map((s) => (
            <li key={s.key} className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4">
              <p className="font-semibold">{s.title}</p>
              <p>Kept for: {s.keep}</p>
              <p className="text-sm text-ink-muted">
                {s.housekeeping
                  ? "Removed every day, in report mode too"
                  : "Erased only when the schedule is applied"}
              </p>
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="due-title" className="flex flex-col gap-3">
        <h2 id="due-title" className="text-xl font-semibold">
          What is due now
        </h2>
        <p className="max-w-prose">
          Counts what the schedule would remove today. It removes only spent links and similar records; it
          never erases personal data.
        </p>
        <Button
          variant="secondary"
          className="self-start"
          loading={busy}
          onClick={async () => {
            const res = await run(() => api.POST("/api/v1/staff/privacy/retention/report"), "Counted");
            if (res) {
              setReport(res);
              load();
            }
          }}
        >
          Count what is due
        </Button>
        {report ? (
          <ResultsTable caption="Due now" results={report.results} mode="report" ruleName={ruleName} />
        ) : null}
      </section>

      <section aria-labelledby="runs-title" className="flex flex-col gap-3">
        <h2 id="runs-title" className="text-xl font-semibold">
          Recent runs
        </h2>
        {o.runs.length === 0 ? (
          <p>No runs yet. The first runs a few minutes after the API starts.</p>
        ) : (
          <ul className="flex flex-col gap-3">
            {o.runs.map((run) => (
              <li key={run.id} className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4">
                <p className="font-semibold">
                  {formatNairobi(run.started_at)} · {run.mode === "apply" ? "Applied" : "Report"}
                </p>
                {run.error ? (
                  <p className="text-danger">Stopped: {run.error}</p>
                ) : run.finished_at === null ? (
                  <p>Did not finish.</p>
                ) : null}
                <RunSummary results={run.results} mode={run.mode} ruleName={ruleName} />
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function ResultsTable({
  caption,
  results,
  mode,
  ruleName,
}: {
  caption: string;
  results: RuleResult[];
  mode: "report" | "apply";
  ruleName: (k: string) => string;
}) {
  return (
    <Table
      caption={caption}
      rows={results}
      rowKey={(r) => r.rule}
      columns={[
        { key: "rule", header: "Records", cell: (r) => ruleName(r.rule) },
        { key: "due", header: "Due", numeric: true, cell: (r) => r.due },
        { key: "done", header: "Result", cell: (r) => resultText(r, mode) },
      ]}
    />
  );
}

function RunSummary({
  results,
  mode,
  ruleName,
}: {
  results: RuleResult[];
  mode: "report" | "apply";
  ruleName: (k: string) => string;
}) {
  const busy = results.filter((r) => r.due > 0);
  if (busy.length === 0) return <p className="text-sm">Nothing was due.</p>;
  return (
    <ul className="flex list-disc flex-col gap-0.5 pl-5 text-sm">
      {busy.map((r) => (
        <li key={r.rule}>
          {ruleName(r.rule)}: {resultText(r, mode)}
        </li>
      ))}
    </ul>
  );
}
