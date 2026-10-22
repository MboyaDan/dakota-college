"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { LinkButton } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { kinds, type ContentKind } from "@/lib/staff/kinds";
import { StatusBadge } from "./content-editor";
import { useSession } from "./session";

type Summary = components["schemas"]["ContentSummary"];

export function ContentList({ kind }: { kind: ContentKind }) {
  const config = kinds[kind];
  const { can } = useSession();
  const [items, setItems] = useState<Summary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const canCreate = kind === "faq-categories" ? can("content.publish") : can("content.write");

  useEffect(() => {
    let active = true;
    api.GET("/api/v1/staff/content/{kind}", { params: { path: { kind } } }).then(({ data, error: err }) => {
      if (!active) return;
      if (data) setItems(data.items);
      else setError(errorMessage(err));
    });
    return () => {
      active = false;
    };
  }, [kind]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-semibold">{config.title}</h1>
          <p className="max-w-prose text-ink-muted">{config.description}</p>
        </div>
        {canCreate ? (
          <LinkButton href={`/staff/content/${kind}/new`}>New {config.singular}</LinkButton>
        ) : null}
      </div>

      {error ? (
        <Alert tone="danger" title="Could not load the list">
          {error}
        </Alert>
      ) : null}
      {!items && !error ? (
        <Loading label={`Loading ${config.title.toLowerCase()}`}>
          <div className="flex flex-col gap-2">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        </Loading>
      ) : null}
      {items && items.length === 0 ? (
        <EmptyState
          title={`No ${config.title.toLowerCase()} yet`}
          description={
            canCreate
              ? `Create the first ${config.singular}. It stays a draft until a publisher publishes it.`
              : "Nothing has been added yet."
          }
          action={
            canCreate ? (
              <LinkButton href={`/staff/content/${kind}/new`} variant="outline">
                New {config.singular}
              </LinkButton>
            ) : undefined
          }
        />
      ) : null}
      {items && items.length > 0 ? (
        <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
          {items.map((i) => (
            <li key={i.id}>
              <Link
                href={`/staff/content/${kind}/${i.id}`}
                className="flex min-h-touch flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 hover:bg-ivory"
              >
                <span className="flex-1 font-semibold text-royal">
                  {i.label}
                  {i.context ? <span className="font-normal text-ink-muted"> · {i.context}</span> : null}
                </span>
                {i.has_pending ? (
                  <span className="text-sm font-semibold text-gold-text">Changes waiting</span>
                ) : null}
                {i.is_sample ? <span className="text-sm text-ink-muted">Sample</span> : null}
                {kind !== "faq-categories" ? <StatusBadge status={i.status} /> : null}
                {i.updated_at && !i.updated_at.startsWith("0001") ? (
                  <span className="text-sm text-ink-muted">Updated {formatNairobi(i.updated_at)}</span>
                ) : null}
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
