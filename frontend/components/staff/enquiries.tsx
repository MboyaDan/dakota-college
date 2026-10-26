"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Button, buttonClasses } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { EmptyState } from "@/components/ui/empty-state";
import { Select, Textarea } from "@/components/ui/field";
import { Pagination } from "@/components/ui/pagination";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag, type Status } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { cn } from "@/lib/cn";
import { formatNairobi } from "@/lib/staff/datetime";
import { useSession } from "./session";

type Item = components["schemas"]["EnquiryItem"];
type Detail = components["schemas"]["EnquiryDetail"];
type Page = components["schemas"]["EnquiryPage"];
type EnquiryStatus = Item["status"];
type Office = Item["office"];

export const offices: { value: Office; label: string }[] = [
  { value: "admissions", label: "Admissions" },
  { value: "finance", label: "Finance" },
  { value: "general", label: "General" },
];

const topicLabels: Record<Item["query_type"], string> = {
  admissions: "Applying and admissions",
  programmes: "A programme",
  fees: "Fees and payment",
  student_life: "Student life",
  alumni: "Alumni: transcripts and certificates",
  other: "Something else",
};

const statusInfo: Record<EnquiryStatus, { label: string; tone: Status }> = {
  new: { label: "New", tone: "info" },
  open: { label: "Open", tone: "warning" },
  waiting: { label: "Waiting for reply", tone: "warning" },
  closed: { label: "Closed", tone: "success" },
};

export function EnquiryStatusTag({ status }: { status: EnquiryStatus }) {
  return <StatusTag status={statusInfo[status].tone}>{statusInfo[status].label}</StatusTag>;
}

// ---------- inbox ----------

export function EnquiryInbox() {
  const { can } = useSession();
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [page, setPage] = useState<Page | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Finance staff without admissions work open on their own office.
  const defaultOffice = can("finance.view") && !can("admissions.view") ? "finance" : "";
  const office = params.get("office") ?? defaultOffice;
  const status = params.get("status") ?? "";
  const assigned = params.get("assigned") ?? "";
  const q = params.get("q") ?? "";
  const pageNumber = Math.max(1, Number(params.get("page")) || 1);

  const hrefWith = useCallback(
    (changes: Record<string, string>) => {
      const next = new URLSearchParams(params.toString());
      if (!next.has("office") && defaultOffice) next.set("office", defaultOffice);
      for (const [k, v] of Object.entries(changes)) {
        // An empty office is kept: it means "all offices", overriding the default.
        if (v || k === "office") next.set(k, v);
        else next.delete(k);
      }
      if (!("page" in changes)) next.delete("page");
      const s = next.toString();
      return s ? `${pathname}?${s}` : pathname;
    },
    [params, pathname, defaultOffice],
  );

  useEffect(() => {
    let active = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- show the loading state for each new query
    setPage(null);
    api
      .GET("/api/v1/staff/enquiries", {
        params: {
          query: {
            office: (office || undefined) as Office | undefined,
            status: (status || undefined) as EnquiryStatus | undefined,
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
  }, [office, status, assigned, q, pageNumber]);

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

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Enquiries</h1>
        <p className="max-w-prose text-ink-muted">
          Questions sent from the website contact form. Each office gets its own topics by email too.
        </p>
      </div>

      <nav aria-label="Offices">
        <ul className="flex flex-wrap gap-2">
          {[{ value: "", label: "All offices" }, ...offices].map((o) => (
            <li key={o.value || "all"}>
              <Link
                href={hrefWith({ office: o.value })}
                aria-current={office === o.value ? "page" : undefined}
                className={cn(
                  "inline-flex min-h-touch items-center rounded-control border-2 px-4 font-semibold",
                  office === o.value
                    ? "border-navy-900 bg-navy-900 text-white"
                    : "border-royal text-royal hover:bg-royal/5",
                )}
              >
                {o.label}
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
          {(Object.keys(statusInfo) as EnquiryStatus[]).map((s) => (
            <option key={s} value={s}>
              {statusInfo[s].label}
            </option>
          ))}
        </Select>
        <Select name="assigned" label="Assigned to" defaultValue={assigned} required>
          <option value="">Anyone</option>
          <option value="me">Me</option>
          <option value="none">Nobody yet</option>
        </Select>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="enquiry-q" className="font-semibold">
            Search
          </label>
          <input
            id="enquiry-q"
            name="q"
            type="search"
            defaultValue={q}
            placeholder="Name, email, phone or words in the message"
            className="block min-h-touch w-full rounded-control border border-ink-muted bg-white px-3 py-2"
          />
        </div>
        <button type="submit" className={buttonClasses("primary")}>
          Show
        </button>
      </form>

      {error ? (
        <Alert tone="danger" title="Could not load enquiries">
          {error}
        </Alert>
      ) : null}
      {!page && !error ? (
        <Loading label="Loading enquiries">
          <div className="flex flex-col gap-2">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-20 w-full" />
            ))}
          </div>
        </Loading>
      ) : null}
      {page && page.items.length === 0 ? (
        <EmptyState
          title="No enquiries here"
          description={
            office || status || assigned || q
              ? "Nothing matches these filters. Try another office or status."
              : "Questions sent from the website will appear here."
          }
        />
      ) : null}
      {page && page.items.length > 0 ? (
        <>
          <p className="text-sm text-ink-muted" role="status">
            {page.total === 1 ? "1 enquiry" : `${page.total} enquiries`}
          </p>
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {page.items.map((e) => (
              <li key={e.id}>
                <Link
                  href={`/staff/enquiries/${e.id}`}
                  className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:gap-4"
                >
                  <span className="flex flex-1 flex-col">
                    <span className="font-semibold text-royal">{e.full_name}</span>
                    <span className="text-sm text-ink-muted">
                      {topicLabels[e.query_type]}
                      {e.programme_title ? ` · ${e.programme_title}` : ""}
                    </span>
                  </span>
                  <EnquiryStatusTag status={e.status} />
                  <span className="text-sm sm:w-40">{e.assignee_name ?? "Not assigned"}</span>
                  <span className="text-sm text-ink-muted sm:w-44">
                    Received {formatNairobi(e.created_at)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
          <Pagination
            page={page.page}
            totalPages={page.total_pages}
            hrefFor={(p) => hrefWith({ page: p === 1 ? "" : String(p) })}
            label="Enquiry pages"
          />
        </>
      ) : null}
    </div>
  );
}

// ---------- one enquiry ----------

function historyText(h: Detail["history"][number]): string {
  const who = h.actor_name ?? "Someone";
  const d = h.details as Record<string, unknown>;
  switch (h.action) {
    case "enquiry.assigned":
      if (!d.to) return `${who} removed the owner`;
      return d.to_name && d.to_name === h.actor_name
        ? `${who} took it on`
        : `${who} assigned it to ${String(d.to_name ?? "a colleague")}`;
    case "enquiry.status_changed": {
      const to = statusInfo[d.to as EnquiryStatus]?.label.toLowerCase() ?? String(d.to);
      return `${who} marked it ${to}`;
    }
    case "enquiry.note_added":
      return `${who} added a note`;
    default:
      return `${who}: ${h.action}`;
  }
}

export function EnquiryView({ id }: { id: string }) {
  const { user, can } = useSession();
  const { show } = useToast();
  const manage = can("enquiries.manage");
  const [enquiry, setEnquiry] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [people, setPeople] = useState<{ id: string; full_name: string }[]>([]);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState("");
  const [noteError, setNoteError] = useState<string | undefined>();

  useEffect(() => {
    let active = true;
    api.GET("/api/v1/staff/enquiries/{id}", { params: { path: { id } } }).then(({ data, error: err }) => {
      if (!active) return;
      if (data) setEnquiry(data);
      else setError(errorMessage(err));
    });
    if (manage) {
      api.GET("/api/v1/staff/enquiries/assignees").then(({ data }) => {
        if (active && data) setPeople(data.items);
      });
    }
    return () => {
      active = false;
    };
  }, [id, manage]);

  async function run(action: () => Promise<{ data?: Detail; error?: unknown }>, success: string) {
    setBusy(true);
    const { data, error: err } = await action();
    setBusy(false);
    if (data) {
      setEnquiry(data);
      show({ tone: "success", title: success });
      return true;
    }
    show({ tone: "danger", title: errorMessage(err) });
    return false;
  }

  const setStatus = (status: "open" | "waiting" | "closed") =>
    run(
      () => api.POST("/api/v1/staff/enquiries/{id}/status", { params: { path: { id } }, body: { status } }),
      `Marked ${statusInfo[status].label.toLowerCase()}`,
    );

  const assign = (assignee: string | null) =>
    run(
      () =>
        api.POST("/api/v1/staff/enquiries/{id}/assign", {
          params: { path: { id } },
          body: { assignee_id: assignee },
        }),
      assignee ? "Assigned" : "Owner removed",
    );

  async function addNote(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!note.trim()) {
      setNoteError("Write the note before saving it.");
      return;
    }
    setNoteError(undefined);
    const ok = await run(
      () =>
        api.POST("/api/v1/staff/enquiries/{id}/notes", { params: { path: { id } }, body: { body: note } }),
      "Note added",
    );
    if (ok) setNote("");
  }

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the enquiry">
        {error}
      </Alert>
    );
  }
  if (!enquiry) {
    return (
      <Loading label="Loading the enquiry">
        <SkeletonText lines={8} />
      </Loading>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "Enquiries", href: "/staff/enquiries" }, { label: enquiry.full_name }]} />
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-3xl font-semibold">{enquiry.full_name}</h1>
        <EnquiryStatusTag status={enquiry.status} />
      </div>
      <p className="text-ink-muted">
        {topicLabels[enquiry.query_type]} · {offices.find((o) => o.value === enquiry.office)?.label} office ·
        received {formatNairobi(enquiry.created_at)}
      </p>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="flex flex-col gap-6">
          <Card>
            <h2 className="text-xl font-semibold">Message</h2>
            {enquiry.programme_title ? (
              <p className="text-sm">
                About{" "}
                {enquiry.programme_slug ? (
                  <a href={`/programmes/${enquiry.programme_slug}`} className="text-royal underline">
                    {enquiry.programme_title}
                  </a>
                ) : (
                  enquiry.programme_title
                )}
              </p>
            ) : null}
            <p className="whitespace-pre-line">{enquiry.message}</p>
          </Card>

          <section aria-labelledby="notes-title" className="flex flex-col gap-3">
            <h2 id="notes-title" className="text-xl font-semibold">
              Internal notes
            </h2>
            <p className="text-sm text-ink-muted">
              Only staff can see notes. The person who asked never does.
            </p>
            {enquiry.notes.length ? (
              <ul className="flex flex-col gap-2">
                {enquiry.notes.map((n) => (
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
            {manage ? (
              <form onSubmit={addNote} className="flex flex-col gap-3">
                <Textarea
                  name="note"
                  label="Add a note"
                  required
                  rows={3}
                  maxLength={5000}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  error={noteError}
                  hint="For example who you called and what you agreed."
                />
                <Button type="submit" variant="secondary" loading={busy} className="self-start">
                  Save note
                </Button>
              </form>
            ) : null}
          </section>
        </div>

        <div className="flex flex-col gap-6">
          <Card>
            <h2 className="text-xl font-semibold">Reply to</h2>
            <dl className="flex flex-col gap-2">
              {enquiry.email ? (
                <div>
                  <dt className="text-sm text-ink-muted">Email</dt>
                  <dd>
                    <a href={`mailto:${enquiry.email}`} className="text-royal underline">
                      {enquiry.email}
                    </a>
                  </dd>
                </div>
              ) : null}
              {enquiry.phone ? (
                <div>
                  <dt className="text-sm text-ink-muted">Phone</dt>
                  <dd>
                    <a href={`tel:${enquiry.phone.replace(/[^\d+]/g, "")}`} className="text-royal underline">
                      {enquiry.phone}
                    </a>
                  </dd>
                </div>
              ) : null}
            </dl>
          </Card>

          <Card>
            <h2 className="text-xl font-semibold">Owner</h2>
            <p>{enquiry.assignee_name ?? "Not assigned yet."}</p>
            {manage ? (
              <form
                className="flex flex-col gap-3"
                onSubmit={(e) => {
                  e.preventDefault();
                  const v = String(new FormData(e.currentTarget).get("assignee") ?? "");
                  void assign(v || null);
                }}
              >
                <Select name="assignee" label="Assign to" defaultValue={enquiry.assigned_to ?? ""} required>
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
                  {enquiry.assigned_to !== user.id ? (
                    <Button variant="outline" disabled={busy} onClick={() => void assign(user.id)}>
                      Assign to me
                    </Button>
                  ) : null}
                </div>
              </form>
            ) : null}
          </Card>

          {manage ? (
            <Card>
              <h2 className="text-xl font-semibold">Status</h2>
              <div className="flex flex-wrap gap-2">
                {(["open", "waiting", "closed"] as const)
                  .filter((s) => s !== enquiry.status)
                  .map((s) => (
                    <Button key={s} variant="outline" disabled={busy} onClick={() => void setStatus(s)}>
                      {s === "open" && enquiry.status === "closed"
                        ? "Reopen"
                        : `Mark ${statusInfo[s].label.toLowerCase()}`}
                    </Button>
                  ))}
              </div>
            </Card>
          ) : null}

          <section aria-labelledby="history-title" className="flex flex-col gap-2">
            <h2 id="history-title" className="text-xl font-semibold">
              History
            </h2>
            <ol className="flex flex-col gap-2 text-sm">
              <li>Received from the website, {formatNairobi(enquiry.created_at)}</li>
              {enquiry.history.map((h, i) => (
                <li key={i}>
                  {historyText(h)}, {formatNairobi(h.at)}
                </li>
              ))}
            </ol>
          </section>
        </div>
      </div>
    </div>
  );
}
