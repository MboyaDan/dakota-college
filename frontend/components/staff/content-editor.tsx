"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Loading, Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { useToast } from "@/components/ui/toast";
import { api, isApiError, type ApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatNairobi } from "@/lib/staff/datetime";
import { fieldErrors, fieldId, setAt, type Doc } from "@/lib/staff/doc";
import { kinds, type ContentKind } from "@/lib/staff/kinds";
import { FormFields } from "./form-fields";
import { Preview } from "./preview";
import { useSession } from "./session";

type Item = components["schemas"]["ContentItem"];
type ContentDocument = components["schemas"]["ContentDocument"];

const statusLabel = { draft: "Draft", published: "Published", archived: "Archived" } as const;
const statusTone = { draft: "warning", published: "success", archived: "info" } as const;

export function StatusBadge({ status }: { status: Item["status"] }) {
  return <StatusTag status={statusTone[status]}>{statusLabel[status]}</StatusTag>;
}

/** Create (id undefined) or edit one content item. */
export function ContentEditor({ kind, id }: { kind: ContentKind; id?: string }) {
  const config = kinds[kind];
  const router = useRouter();
  const { can } = useSession();
  const { show } = useToast();

  const [item, setItem] = useState<Item | null>(null);
  const [doc, setDoc] = useState<Doc | null>(id ? null : config.empty());
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [problem, setProblem] = useState<ApiError["error"] | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [view, setView] = useState<"edit" | "preview">("edit");
  const summaryRef = useRef<HTMLDivElement>(null);

  const [reloadKey, setReloadKey] = useState(0);
  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  useEffect(() => {
    if (!id) return;
    let active = true;
    api
      .GET("/api/v1/staff/content/{kind}/{id}", { params: { path: { kind, id } } })
      .then(({ data, response }) => {
        if (!active) return;
        if (!data) {
          setNotFound(response.status === 404);
          return;
        }
        setItem(data);
        setDoc((data.pending ?? data.live) as Doc);
        setDirty(false);
      });
    return () => {
      active = false;
    };
  }, [kind, id, reloadKey]);

  // Warn before leaving with unsaved changes.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  useEffect(() => {
    if (problem?.fields?.length) summaryRef.current?.focus();
  }, [problem]);

  const errors = fieldErrors(problem?.fields);
  const workflow = item ? item.workflow : config.kind !== "faq-categories";
  const canSave = workflow ? can("content.write") : can("content.publish");
  const canPublish = can("content.publish") && workflow;

  function onChange(path: string, value: unknown) {
    setDoc((d) => setAt(d ?? {}, path, value));
    setDirty(true);
  }

  async function run(label: string, fn: () => Promise<{ data?: unknown; error?: unknown }>, success: string) {
    setBusy(label);
    setProblem(null);
    const { data, error } = await fn();
    setBusy(null);
    if (error || !data) {
      setProblem(
        isApiError(error)
          ? error.error
          : {
              code: "network",
              message: "We could not reach the server. Check your connection and try again.",
            },
      );
      return null;
    }
    show({ tone: "success", title: success });
    return data;
  }

  async function save() {
    if (!doc) return;
    const document = doc as unknown as ContentDocument;
    if (!id) {
      const created = await run(
        "save",
        () => api.POST("/api/v1/staff/content/{kind}", { params: { path: { kind } }, body: { document } }),
        `Draft ${config.singular} saved`,
      );
      if (created) {
        setDirty(false);
        router.replace(`/staff/content/${kind}/${(created as { id: string }).id}`);
      }
      return;
    }
    const saved = await run(
      "save",
      () =>
        api.PUT("/api/v1/staff/content/{kind}/{id}", {
          params: { path: { kind, id } },
          body: { document, base_version: item!.version },
        }),
      item?.status === "published"
        ? "Changes saved. They go live when a publisher approves them."
        : "Changes saved",
    );
    if (saved) {
      setItem(saved as Item);
      setDirty(false);
    }
  }

  async function act(action: "publish" | "unpublish" | "archive", success: string) {
    if (!id || !item) return;
    const path = { params: { path: { kind, id } } };
    const result = await run(
      action,
      () =>
        action === "publish"
          ? api.POST("/api/v1/staff/content/{kind}/{id}/publish", {
              ...path,
              body: { base_version: item.version },
            })
          : action === "unpublish"
            ? api.POST("/api/v1/staff/content/{kind}/{id}/unpublish", path)
            : api.POST("/api/v1/staff/content/{kind}/{id}/archive", path),
      success,
    );
    if (result) {
      setItem(result as Item);
      setDoc(((result as Item).pending ?? (result as Item).live) as Doc);
    }
  }

  async function discard() {
    if (!id) return;
    const ok = await run(
      "discard",
      async () => {
        const r = await api.DELETE("/api/v1/staff/content/{kind}/{id}/pending", {
          params: { path: { kind, id } },
        });
        return { data: r.response.ok ? {} : undefined, error: r.error };
      },
      "Waiting changes discarded",
    );
    if (ok) reload();
  }

  async function remove() {
    if (!id || !window.confirm(`Delete this ${config.singular}? This cannot be undone.`)) return;
    const ok = await run(
      "delete",
      async () => {
        const r = await api.DELETE("/api/v1/staff/content/{kind}/{id}", { params: { path: { kind, id } } });
        return { data: r.response.ok ? {} : undefined, error: r.error };
      },
      `${config.singular[0].toUpperCase()}${config.singular.slice(1)} deleted`,
    );
    if (ok) {
      setDirty(false);
      router.replace(`/staff/content/${kind}`);
    }
  }

  if (notFound) {
    return (
      <Alert tone="warning" title={`This ${config.singular} does not exist`}>
        It may have been deleted.{" "}
        <Link href={`/staff/content/${kind}`} className="text-royal underline">
          Back to {config.title.toLowerCase()}
        </Link>
      </Alert>
    );
  }
  if (!doc) {
    return (
      <Loading label={`Loading ${config.singular}`}>
        <Skeleton className="mb-4 h-10 w-1/2" />
        <SkeletonText lines={6} />
      </Loading>
    );
  }

  const heading = id ? `Edit ${config.singular}` : `New ${config.singular}`;
  const conflict = problem?.code === "edit_conflict";

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb
        items={[
          { label: "Overview", href: "/staff" },
          { label: config.title, href: `/staff/content/${kind}` },
          { label: heading },
        ]}
      />
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-3xl font-semibold">{heading}</h1>
        {item ? <StatusBadge status={item.status} /> : null}
        {item?.is_sample ? <span className="text-sm text-ink-muted">(sample data)</span> : null}
      </div>

      {item?.pending ? (
        <Alert tone="info" title="Changes are waiting for approval">
          The site still shows the published version. A publisher must publish these changes
          {item.pending_updated_at ? ` (saved ${formatNairobi(item.pending_updated_at)})` : ""}.
        </Alert>
      ) : null}
      {item?.status === "published" && !item.pending && canSave ? (
        <p className="text-ink-muted">
          This is live. Your changes will wait for a publisher before they appear on the site.
        </p>
      ) : null}

      {problem ? (
        <div ref={summaryRef} tabIndex={-1} className="focus:outline-none">
          <Alert tone="danger" title={conflict ? "Someone else changed this" : "We could not do that"} live>
            <p>{problem.message}</p>
            {problem.fields?.length ? (
              <ul className="mt-2 list-disc pl-5">
                {problem.fields.map((f) => (
                  <li key={f.field}>
                    <a
                      href={`#${fieldId(f.field.replace(/\[(\d+)\]/g, ".$1"))}`}
                      className="text-royal underline"
                    >
                      {f.message}
                    </a>
                  </li>
                ))}
              </ul>
            ) : null}
            {conflict ? (
              <Button
                variant="outline"
                className="mt-3"
                onClick={() => {
                  setProblem(null);
                  reload();
                }}
              >
                Reload the latest version
              </Button>
            ) : null}
          </Alert>
        </div>
      ) : null}

      <div className="flex gap-2" role="group" aria-label="View">
        <Button
          variant={view === "edit" ? "secondary" : "outline"}
          aria-pressed={view === "edit"}
          onClick={() => setView("edit")}
        >
          Edit
        </Button>
        <Button
          variant={view === "preview" ? "secondary" : "outline"}
          aria-pressed={view === "preview"}
          onClick={() => setView("preview")}
        >
          Preview
        </Button>
      </div>

      {view === "edit" ? (
        <form
          className="grid max-w-3xl gap-5"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          <FormFields
            fields={config.fields}
            doc={doc}
            errors={errors}
            onChange={onChange}
            disabled={!canSave || busy !== null}
          />
          {canSave ? (
            <div className="sticky bottom-0 flex flex-wrap gap-3 border-t border-stone bg-ivory py-3">
              <Button
                type="submit"
                variant={canPublish ? "secondary" : "primary"}
                loading={busy === "save"}
                disabled={busy !== null || (!dirty && Boolean(id))}
              >
                {id ? "Save changes" : "Save draft"}
              </Button>
              {dirty ? <span className="self-center text-sm text-ink-muted">Unsaved changes</span> : null}
            </div>
          ) : (
            <p className="text-ink-muted">You can view this but not change it.</p>
          )}
        </form>
      ) : (
        <section
          aria-label="Preview"
          className="max-w-3xl rounded-card border-2 border-dashed border-stone bg-white p-6"
        >
          <p className="mb-4 text-sm font-semibold text-gold-text">Preview, not live yet</p>
          <Preview kind={kind} doc={doc} />
        </section>
      )}

      {item ? (
        <section
          aria-labelledby="workflow-heading"
          className="flex max-w-3xl flex-col gap-3 rounded-card border border-stone bg-white p-4"
        >
          <h2 id="workflow-heading" className="text-xl font-semibold">
            Publishing
          </h2>
          {dirty ? <p className="text-ink-muted">Save your changes before publishing.</p> : null}
          <div className="flex flex-wrap gap-3">
            {canPublish && (item.status !== "published" || item.pending) ? (
              <Button
                loading={busy === "publish"}
                disabled={busy !== null || dirty}
                onClick={() => void act("publish", "Published. The site updates within a few minutes.")}
              >
                {item.pending ? "Publish changes" : "Publish"}
              </Button>
            ) : null}
            {canPublish && item.status === "published" ? (
              <Button
                variant="outline"
                loading={busy === "unpublish"}
                disabled={busy !== null || dirty}
                onClick={() => void act("unpublish", "Taken off the site")}
              >
                Take off the site
              </Button>
            ) : null}
            {canPublish && item.status !== "archived" ? (
              <Button
                variant="outline"
                loading={busy === "archive"}
                disabled={busy !== null || dirty}
                onClick={() => void act("archive", "Archived")}
              >
                Archive
              </Button>
            ) : null}
            {can("content.write") && item.pending ? (
              <Button
                variant="outline"
                loading={busy === "discard"}
                disabled={busy !== null}
                onClick={() => void discard()}
              >
                Discard waiting changes
              </Button>
            ) : null}
            {canSave && (!workflow || (item.status === "draft" && !item.published_at)) ? (
              <Button
                variant="danger"
                loading={busy === "delete"}
                disabled={busy !== null}
                onClick={() => void remove()}
              >
                Delete
              </Button>
            ) : null}
          </div>
          {!canPublish && workflow ? (
            <p className="text-sm text-ink-muted">
              A Registrar or Principal publishes. Save your draft and let them know.
            </p>
          ) : null}
        </section>
      ) : null}
    </div>
  );
}
