"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState } from "@/components/ui/empty-state";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { kindOrder, kinds, type ContentKind } from "@/lib/staff/kinds";

type Summary = components["schemas"]["ContentSummary"];
export type QueueItem = Summary & { kind: ContentKind; reason: "changes" | "draft" };

/** Drafts and waiting changes across every content type. */
export async function loadQueue(): Promise<QueueItem[]> {
  const lists = await Promise.all(
    kindOrder
      .filter((k) => k !== "faq-categories")
      .map(async (kind) => {
        const { data } = await api.GET("/api/v1/staff/content/{kind}", { params: { path: { kind } } });
        return (data?.items ?? []).map((i) => ({ ...i, kind }));
      }),
  );
  return lists
    .flat()
    .filter((i) => i.has_pending || i.status === "draft")
    .map((i) => ({ ...i, reason: i.has_pending ? ("changes" as const) : ("draft" as const) }))
    .sort((a, b) => b.updated_at.localeCompare(a.updated_at));
}

export function ApprovalQueue() {
  const [items, setItems] = useState<QueueItem[] | null>(null);
  useEffect(() => {
    void loadQueue().then(setItems);
  }, []);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Waiting for approval</h1>
        <p className="max-w-prose text-ink-muted">
          Drafts and changes to live content. Open one, check the preview, then publish.
        </p>
      </div>
      {!items ? (
        <Loading label="Loading the queue">
          <Skeleton className="h-40 w-full" />
        </Loading>
      ) : items.length === 0 ? (
        <EmptyState
          title="Nothing is waiting"
          description="When an editor saves a draft or changes live content, it appears here."
        />
      ) : (
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
          {items.map((i) => (
            <li key={`${i.kind}-${i.id}`}>
              <Link
                href={`/staff/content/${i.kind}/${i.id}`}
                className="flex min-h-touch flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 hover:bg-ivory"
              >
                <span className="flex-1">
                  <span className="font-semibold text-royal">{i.label}</span>
                  <span className="text-ink-muted"> · {kinds[i.kind].singular}</span>
                </span>
                <span className="text-sm font-semibold text-gold-text">
                  {i.reason === "changes" ? "Changes to live content" : "New draft"}
                </span>
                <span className="text-sm text-ink-muted">Updated {formatNairobi(i.updated_at)}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
