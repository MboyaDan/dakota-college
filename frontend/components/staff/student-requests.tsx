"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Select, Textarea } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { Tabs } from "@/components/ui/tabs";
import { requestKinds, requestStatus } from "@/lib/student/requests";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { useSession } from "./session";
import { useAction } from "./use-action";

// What students ask for (D-142, D-145): service requests and corrections to
// personal details. students.view to read; students.update to act.

type ServiceRequest = components["schemas"]["StaffServiceRequest"];
type ChangeRequest = components["schemas"]["StaffChangeRequest"];

export function StudentRequests() {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Student requests</h1>
        <p className="max-w-prose text-ink-muted">
          Letters, transcripts and other services students ask for, and corrections to their personal details.
          Students are emailed when you reply.
        </p>
      </div>
      <Tabs
        label="Student requests"
        items={[
          { id: "services", label: "Services", content: <Services /> },
          { id: "changes", label: "Changes of details", content: <Changes /> },
        ]}
      />
    </div>
  );
}

function StudentLink({ id, name, number }: { id: string; name: string; number: string }) {
  return (
    <Link href={`/staff/students/${id}`} className="font-semibold text-royal underline">
      {name} <span className="font-mono font-normal">{number}</span>
    </Link>
  );
}

function Services() {
  const { can } = useSession();
  const update = can("students.update");
  const issue = can("academics.results.release");
  const [status, setStatus] = useState("open");
  const [items, setItems] = useState<ServiceRequest[] | null>(null);
  useEffect(() => {
    let active = true;
    api
      .GET("/api/v1/staff/service-requests", {
        params: { query: { status: (status || undefined) as ServiceRequest["status"] | undefined } },
      })
      .then(({ data }) => {
        if (active) setItems(data?.items ?? []);
      });
    return () => {
      active = false;
    };
  }, [status]);

  return (
    <div className="flex flex-col gap-4">
      <Select
        id="service-status"
        name="status"
        label="Show"
        required
        value={status}
        onChange={(e) => setStatus(e.target.value)}
      >
        <option value="open">Received, not started</option>
        <option value="in_progress">In progress</option>
        <option value="done">Done</option>
        <option value="declined">Declined</option>
        <option value="">All</option>
      </Select>
      {items === null ? (
        <Loading label="Loading requests">
          <Skeleton className="h-32 w-full" />
        </Loading>
      ) : items.length === 0 ? (
        <p>No requests here.</p>
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((r) => (
            <li key={r.id}>
              <ServiceCard
                r={r}
                update={update}
                issue={issue}
                onSaved={async () => {
                  const { data } = await api.GET("/api/v1/staff/service-requests", {
                    params: {
                      query: { status: (status || undefined) as ServiceRequest["status"] | undefined },
                    },
                  });
                  setItems(data?.items ?? []);
                }}
              />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function ServiceCard({
  r,
  update,
  issue,
  onSaved,
}: {
  r: ServiceRequest;
  update: boolean;
  issue: boolean;
  onSaved: () => Promise<void>;
}) {
  const { busy, errors, run } = useAction();
  const [next, setNext] = useState<"in_progress" | "done" | "declined">(
    r.status === "open" ? "in_progress" : "done",
  );
  const [response, setResponse] = useState(r.response);
  const id = r.id.slice(0, 8);
  const closed = r.status === "done" || r.status === "declined";
  return (
    <article className="flex flex-col gap-2 rounded-card border border-stone bg-white p-4">
      <div className="flex flex-wrap items-center gap-2">
        <StudentLink id={r.student_id} name={r.student_name} number={r.student_number} />
        <StatusTag status={requestStatus[r.status].tone}>{requestStatus[r.status].label}</StatusTag>
      </div>
      <p className="text-sm text-ink-muted">
        {requestKinds.find((k) => k.value === r.kind)?.label} · sent {formatNairobi(r.created_at)}
        {r.handled_by_name ? ` · ${r.handled_by_name}` : ""}
      </p>
      <p className="whitespace-pre-line">{r.details}</p>
      {r.response && closed ? (
        <p className="border-l-4 border-gold pl-3 whitespace-pre-line">
          <strong>Reply:</strong> {r.response}
        </p>
      ) : null}
      {issue && r.kind === "transcript" && !closed ? (
        <div className="flex flex-col gap-1 rounded-control bg-ivory p-3">
          <p>
            Issuing the official transcript marks this request done, with a reply telling the student to
            download it from Results in their portal.
          </p>
          <Button
            variant="secondary"
            className="self-start"
            loading={busy}
            onClick={async () => {
              if (
                await run(
                  () =>
                    api.POST("/api/v1/staff/students/{id}/transcripts", {
                      params: { path: { id: r.student_id } },
                      body: { service_request_id: r.id },
                    }),
                  "Transcript issued and request done",
                )
              ) {
                await onSaved();
              }
            }}
          >
            Issue the transcript<span className="sr-only"> for {r.student_name}</span>
          </Button>
        </div>
      ) : null}
      {update && !closed ? (
        <form
          noValidate
          className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[12rem_1fr_auto]"
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run(
                () =>
                  api.POST("/api/v1/staff/service-requests/{id}", {
                    params: { path: { id: r.id } },
                    body: { status: next, response },
                  }),
                "Request updated",
              )
            ) {
              await onSaved();
            }
          }}
        >
          <Select
            id={`svc-${id}-status`}
            name="status"
            label="Move to"
            required
            value={next}
            error={errors.status}
            onChange={(e) => setNext(e.target.value as typeof next)}
          >
            <option value="in_progress">In progress</option>
            <option value="done">Done</option>
            <option value="declined">Declined</option>
          </Select>
          <Textarea
            id={`svc-${id}-response`}
            name="response"
            label="Reply to the student"
            rows={2}
            maxLength={2000}
            value={response}
            error={errors.response}
            hint="Needed to decline. For example: collect it from the office on Friday."
            onChange={(e) => setResponse(e.target.value)}
          />
          <Button type="submit" variant="secondary" loading={busy}>
            Save<span className="sr-only"> request from {r.student_name}</span>
          </Button>
        </form>
      ) : null}
    </article>
  );
}

const fieldLabel: Record<string, string> = {
  first_name: "First name",
  middle_name: "Middle name",
  last_name: "Last name",
  date_of_birth: "Date of birth",
  identity_document: "ID number",
};

function Changes() {
  const { can } = useSession();
  const update = can("students.update");
  const [items, setItems] = useState<ChangeRequest[] | null>(null);
  const load = () =>
    api.GET("/api/v1/staff/profile-requests").then(({ data }) => setItems(data?.items ?? []));
  useEffect(() => {
    void load();
  }, []);

  if (items === null) {
    return (
      <Loading label="Loading changes">
        <Skeleton className="h-32 w-full" />
      </Loading>
    );
  }
  if (items.length === 0) return <p>No changes asked for.</p>;
  return (
    <ul className="flex flex-col gap-3">
      {items.map((c) => (
        <li key={c.id}>
          <ChangeCard c={c} update={update} onDone={(r) => setItems(r)} />
        </li>
      ))}
    </ul>
  );
}

function ChangeCard({
  c,
  update,
  onDone,
}: {
  c: ChangeRequest;
  update: boolean;
  onDone: (items: ChangeRequest[]) => void;
}) {
  const { busy, errors, run } = useAction();
  const [decision, setDecision] = useState("");
  const id = c.id.slice(0, 8);
  const decide = async (approve: boolean) => {
    const r = await run(
      () =>
        approve
          ? api.POST("/api/v1/staff/profile-requests/{id}/approve", {
              params: { path: { id: c.id } },
              body: { decision },
            })
          : api.POST("/api/v1/staff/profile-requests/{id}/decline", {
              params: { path: { id: c.id } },
              body: { decision },
            }),
      approve ? "Change approved" : "Change declined",
    );
    if (r) onDone(r.items);
  };
  return (
    <article className="flex flex-col gap-2 rounded-card border border-stone bg-white p-4">
      <div className="flex flex-wrap items-center gap-2">
        <StudentLink id={c.student_id} name={c.student_name} number={c.student_number} />
        <StatusTag
          status={c.status === "pending" ? "warning" : c.status === "approved" ? "success" : "danger"}
        >
          {c.status === "pending" ? "Waiting" : c.status === "approved" ? "Approved" : "Declined"}
        </StatusTag>
      </div>
      <p className="text-sm text-ink-muted">
        Asked {formatNairobi(c.created_at)}
        {c.decided_by_name ? ` · decided by ${c.decided_by_name}` : ""}
      </p>
      <dl className="grid grid-cols-1 gap-x-4 gap-y-1 sm:grid-cols-[10rem_1fr]">
        {Object.entries(c.new_values).map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-sm text-ink-muted">{fieldLabel[k] ?? k}</dt>
            <dd>
              {c.old_values[k] || "(empty)"} → <strong>{v}</strong>
            </dd>
          </div>
        ))}
      </dl>
      <p>
        <strong>Why:</strong> {c.reason}
      </p>
      {c.decision ? <p className="text-sm">Decision: {c.decision}</p> : null}
      {update && c.status === "pending" ? (
        <div className="flex flex-col gap-3">
          <Textarea
            id={`chg-${id}-decision`}
            name="decision"
            label="Note to the student"
            rows={2}
            maxLength={2000}
            value={decision}
            error={errors.decision}
            hint="Needed to decline. Check the change against their documents first."
            onChange={(e) => setDecision(e.target.value)}
          />
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" loading={busy} onClick={() => void decide(true)}>
              Approve<span className="sr-only"> change for {c.student_name}</span>
            </Button>
            <Button variant="outline" disabled={busy} onClick={() => void decide(false)}>
              Decline<span className="sr-only"> change for {c.student_name}</span>
            </Button>
          </div>
        </div>
      ) : null}
    </article>
  );
}
