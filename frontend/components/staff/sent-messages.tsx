"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Button, buttonClasses } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Select } from "@/components/ui/field";
import { Pagination } from "@/components/ui/pagination";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { StatusTag, Tag, type Status } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { useSession } from "./session";

type Item = components["schemas"]["SentMessage"];
type MessageStatus = Item["status"];
type Page = { items: Item[]; total: number; page: number; page_size: number };

const statusInfo: Record<MessageStatus, { label: string; tone: Status }> = {
  queued: { label: "Waiting to send", tone: "info" },
  sending: { label: "Sending", tone: "info" },
  sent: { label: "Sent", tone: "success" },
  failed: { label: "Failed", tone: "danger" },
  skipped: { label: "Not sent", tone: "warning" },
};

/** The log of every email and text message the queue sent or tried (notifications.manage). */
export function SentMessages() {
  const { can } = useSession();
  const { show } = useToast();
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [page, setPage] = useState<Page | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [reload, setReload] = useState(0);

  const status = params.get("status") ?? "";
  const channel = params.get("channel") ?? "";
  const q = params.get("q") ?? "";
  const pageNumber = Math.max(1, Number(params.get("page")) || 1);

  const hrefWith = useCallback(
    (changes: Record<string, string>) => {
      const next = new URLSearchParams(params.toString());
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
      .GET("/api/v1/staff/notifications", {
        params: {
          query: {
            status: (status || undefined) as MessageStatus | undefined,
            channel: (channel || undefined) as Item["channel"] | undefined,
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
  }, [status, channel, q, pageNumber, reload]);

  function search(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    router.push(
      hrefWith({
        status: String(data.get("status") ?? ""),
        channel: String(data.get("channel") ?? ""),
        q: String(data.get("q") ?? "").trim(),
      }),
    );
  }

  async function resend(m: Item) {
    setBusy(m.id);
    const { response, error: err } = await api.POST("/api/v1/staff/notifications/{id}/resend", {
      params: { path: { id: m.id } },
    });
    setBusy(null);
    if (response.ok) {
      show({ tone: "success", title: "Sending again", description: `${m.name} to ${m.recipient}.` });
      setReload((n) => n + 1);
    } else {
      show({ tone: "danger", title: "Not sent again", description: errorMessage(err) });
    }
  }

  function relatedHref(m: Item): string | null {
    if (m.related_type === "application" && m.related_id && can("admissions.view")) {
      return `/staff/applications/${m.related_id}`;
    }
    if (m.related_type === "enquiry" && m.related_id && can("enquiries.view")) {
      return `/staff/enquiries/${m.related_id}`;
    }
    return null;
  }

  const totalPages = page ? Math.max(1, Math.ceil(page.total / page.page_size)) : 1;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-3xl font-semibold">Sent messages</h1>
        <p className="max-w-prose">
          Every email the website sends, newest first. A message that fails is tried again by itself, up to
          six times over about five and a half hours. If it still fails, you can send it again here. The text
          of a message is not shown, and it is erased once the message is sent.
        </p>
        <p>
          <Link href="/staff/messages" className="font-semibold text-royal underline">
            Change the wording of messages
          </Link>
        </p>
      </div>

      {/* noValidate: "Any" is an empty choice, which a required select would refuse. */}
      <form
        noValidate
        onSubmit={search}
        className="grid items-end gap-4 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_2fr_auto]"
        key={`${status}|${channel}|${q}`}
      >
        <Select name="status" label="Status" defaultValue={status} required>
          <option value="">Any status</option>
          {(Object.keys(statusInfo) as MessageStatus[]).map((s) => (
            <option key={s} value={s}>
              {statusInfo[s].label}
            </option>
          ))}
        </Select>
        <Select name="channel" label="Sent by" defaultValue={channel} required>
          <option value="">Email or text</option>
          <option value="email">Email</option>
          <option value="sms">Text message</option>
        </Select>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="sent-q" className="font-semibold">
            Recipient
          </label>
          <input
            id="sent-q"
            name="q"
            type="search"
            maxLength={80}
            defaultValue={q}
            placeholder="Part of an email address or phone number"
            className="block min-h-touch w-full rounded-control border border-ink-muted bg-white px-3 py-2"
          />
        </div>
        <button type="submit" className={buttonClasses("primary")}>
          Show
        </button>
      </form>

      {error ? (
        <Alert tone="danger" title="Could not load sent messages">
          {error}
        </Alert>
      ) : null}
      {!page && !error ? (
        <Loading label="Loading sent messages">
          <div className="flex flex-col gap-2">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-20 w-full" />
            ))}
          </div>
        </Loading>
      ) : null}
      {page && page.items.length === 0 ? (
        <EmptyState
          title="No messages here"
          description={
            status || channel || q
              ? "Nothing matches these filters. Try another status or recipient."
              : "Emails appear here as soon as the website sends them."
          }
        />
      ) : null}
      {page && page.items.length > 0 ? (
        <>
          <p className="text-sm text-ink-muted" role="status">
            {page.total === 1 ? "1 message" : `${page.total} messages`}
          </p>
          <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {page.items.map((m) => {
              const href = relatedHref(m);
              return (
                <li key={m.id} className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-start sm:gap-4">
                  <div className="flex min-w-0 flex-1 flex-col gap-1">
                    <span className="font-semibold">{m.name}</span>
                    <span className="text-sm break-all">
                      To {m.recipient}
                      {href ? (
                        <>
                          {" · "}
                          <Link href={href} className="text-royal underline">
                            {m.related_type === "application" ? "Open the application" : "Open the enquiry"}
                          </Link>
                        </>
                      ) : null}
                    </span>
                    <span className="text-sm text-ink-muted">
                      {m.sent_at
                        ? `Sent ${formatNairobi(m.sent_at)}`
                        : `Created ${formatNairobi(m.created_at)}`}
                      {m.attempts > 1 ? ` · ${m.attempts} attempts` : ""}
                    </span>
                    {m.status === "queued" && m.next_attempt_at ? (
                      <span className="text-sm">Trying again {formatNairobi(m.next_attempt_at)}.</span>
                    ) : null}
                    {m.status === "skipped" ? (
                      <span className="text-sm text-ink-muted">
                        Text messages are not sent until an SMS provider is set up.
                      </span>
                    ) : m.error && m.status !== "sent" ? (
                      <span className="text-sm break-words text-ink-muted">Last error: {m.error}</span>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap items-center gap-2 sm:flex-col sm:items-end">
                    <Tag tone="navy">{m.channel === "email" ? "Email" : "Text message"}</Tag>
                    <StatusTag status={statusInfo[m.status].tone}>{statusInfo[m.status].label}</StatusTag>
                    {m.can_resend ? (
                      <Button
                        variant="outline"
                        disabled={busy === m.id}
                        onClick={() => void resend(m)}
                        aria-label={`Send again: ${m.name} to ${m.recipient}`}
                      >
                        {busy === m.id ? "Sending…" : "Send again"}
                      </Button>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>
          <Pagination
            page={page.page}
            totalPages={totalPages}
            hrefFor={(p) => hrefWith({ page: p === 1 ? "" : String(p) })}
            label="Sent message pages"
          />
        </>
      ) : null}
    </div>
  );
}
