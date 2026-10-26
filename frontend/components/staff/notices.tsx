"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input, Select, Textarea } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Notices to students (D-144): managers post to everyone (registrar's
// office), a school, a cohort or a class within their schools; trainers post
// to their own classes (on the class page).

type StaffNotice = components["schemas"]["StaffNotice"];
type NoticeInput = components["schemas"]["NoticeInput"];
type Option = { id: string; label: string };

const audienceLabel: Record<StaffNotice["audience"], string> = {
  all: "Everyone",
  school: "School",
  cohort: "Cohort",
  offering: "Class",
};

export function NoticeList({
  notices,
  canWithdraw,
  onWithdraw,
}: {
  notices: StaffNotice[];
  canWithdraw: (n: StaffNotice) => boolean;
  onWithdraw: (n: StaffNotice) => void;
}) {
  if (notices.length === 0) return <p>No notices yet.</p>;
  return (
    <ul className="flex flex-col gap-2">
      {notices.map((n) => (
        <li key={n.id} className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold">{n.title}</span>
            {n.withdrawn ? <StatusTag status="info">Withdrawn</StatusTag> : null}
          </div>
          <p className="text-sm text-ink-muted">
            {audienceLabel[n.audience]}
            {n.audience_name ? `: ${n.audience_name}` : ""} · {n.author_name}, {formatNairobi(n.created_at)}
            {n.ends_on ? ` · until ${formatDate(n.ends_on)}` : ""}
          </p>
          <p className="whitespace-pre-line">{n.body}</p>
          {!n.withdrawn && canWithdraw(n) ? (
            <Button variant="ghost" className="self-start" onClick={() => onWithdraw(n)}>
              Withdraw<span className="sr-only"> {n.title}</span>
            </Button>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

/** Title, body and end date; the audience is chosen by the caller. */
export function NoticeFields({
  value,
  onChange,
  errors,
  prefix,
}: {
  value: NoticeInput;
  onChange: (v: NoticeInput) => void;
  errors: Record<string, string>;
  prefix: string;
}) {
  return (
    <>
      <Input
        id={`${prefix}-title`}
        name="title"
        label="Title"
        required
        maxLength={160}
        value={value.title}
        error={errors.title}
        onChange={(e) => onChange({ ...value, title: e.target.value })}
      />
      <Textarea
        id={`${prefix}-body`}
        name="body"
        label="Notice"
        required
        rows={4}
        maxLength={5000}
        value={value.body}
        error={errors.body}
        onChange={(e) => onChange({ ...value, body: e.target.value })}
      />
      <Input
        id={`${prefix}-ends`}
        name="ends_on"
        type="date"
        label="Show until"
        value={value.ends_on ?? ""}
        error={errors.ends_on}
        hint="Leave empty to show it until it is withdrawn."
        onChange={(e) => onChange({ ...value, ends_on: e.target.value })}
      />
    </>
  );
}

const blank: NoticeInput = { audience: "school", title: "", body: "", ends_on: "" };

function useOptions(load: () => Promise<Option[]>) {
  const [items, setItems] = useState<Option[]>([]);
  useEffect(() => {
    void load().then(setItems);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load once
  }, []);
  return items;
}

export function StaffNotices() {
  const { user, can } = useSession();
  const allSchools = can("schools.all");
  const { busy, errors, run } = useAction();
  const [notices, setNotices] = useState<StaffNotice[] | null>(null);
  const [draft, setDraft] = useState<NoticeInput>(blank);
  const schools = useOptions(async () =>
    (
      (await api.GET("/api/v1/staff/content/{kind}", { params: { path: { kind: "schools" } } })).data
        ?.items ?? []
    ).map((s) => ({ id: s.id, label: s.label })),
  );
  const cohorts = useOptions(async () =>
    ((await api.GET("/api/v1/staff/cohorts")).data?.items ?? []).map((c) => ({ id: c.id, label: c.name })),
  );
  const classes = useOptions(async () =>
    ((await api.GET("/api/v1/staff/offerings")).data?.items ?? []).map((o) => ({
      id: o.id,
      label: `${o.unit_code} ${o.unit_title} (${o.term_name})`,
    })),
  );

  useEffect(() => {
    api.GET("/api/v1/staff/notices").then(({ data }) => setNotices(data?.items ?? []));
  }, []);

  const targets: Record<
    string,
    { key: "school_id" | "cohort_id" | "offering_id"; label: string; options: Option[] }
  > = {
    school: { key: "school_id", label: "School", options: schools },
    cohort: { key: "cohort_id", label: "Cohort", options: cohorts },
    offering: { key: "offering_id", label: "Class", options: classes },
  };
  const target = draft.audience && draft.audience !== "all" ? targets[draft.audience] : null;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Notices</h1>
        <p className="max-w-prose text-ink-muted">
          Notices appear in the student portal for the students they are addressed to.
        </p>
      </div>
      <Card>
        <h2 className="text-xl font-semibold">Post a notice</h2>
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            const body: NoticeInput = {
              audience: draft.audience,
              title: draft.title,
              body: draft.body,
              ends_on: draft.ends_on,
            };
            if (target) body[target.key] = draft[target.key] || null;
            const r = await run(() => api.POST("/api/v1/staff/notices", { body }), "Notice posted");
            if (r) {
              setNotices(r.items);
              setDraft({ ...blank, audience: draft.audience });
            }
          }}
        >
          <Select
            id="notice-audience"
            name="audience"
            label="Who is it for"
            required
            value={draft.audience}
            error={errors.audience}
            onChange={(e) => setDraft({ ...draft, audience: e.target.value as NoticeInput["audience"] })}
          >
            {allSchools ? <option value="all">Everyone</option> : null}
            <option value="school">A school</option>
            <option value="cohort">A cohort</option>
            <option value="offering">A class</option>
          </Select>
          {target ? (
            <Select
              id="notice-target"
              name={target.key}
              label={target.label}
              required
              value={draft[target.key] ?? ""}
              error={errors[target.key]}
              onChange={(e) => setDraft({ ...draft, [target.key]: e.target.value })}
            >
              <option value="">Choose</option>
              {target.options.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.label}
                </option>
              ))}
            </Select>
          ) : null}
          <NoticeFields value={draft} onChange={setDraft} errors={errors} prefix="notice" />
          <Button type="submit" loading={busy} className="self-start">
            Post notice
          </Button>
        </form>
      </Card>
      <section aria-labelledby="notices-title" className="flex flex-col gap-3">
        <h2 id="notices-title" className="text-xl font-semibold">
          Recent notices
        </h2>
        {notices === null ? (
          <Loading label="Loading notices">
            <Skeleton className="h-32 w-full" />
          </Loading>
        ) : (
          <NoticeList
            notices={notices}
            canWithdraw={(n) => allSchools || n.audience !== "all" || n.created_by === user.id}
            onWithdraw={async (n) => {
              const r = await run(
                () => api.POST("/api/v1/staff/notices/{id}/withdraw", { params: { path: { id: n.id } } }),
                "Notice withdrawn",
              );
              if (r) setNotices(r.items);
            }}
          />
        )}
      </section>
    </div>
  );
}

/** Notices of one class, on the trainer's class page. */
export function ClassNotices({ offeringId }: { offeringId: string }) {
  const { user } = useSession();
  const { busy, errors, run } = useAction();
  const [notices, setNotices] = useState<StaffNotice[] | null>(null);
  const [draft, setDraft] = useState<NoticeInput>({ ...blank, audience: "offering" });
  const path = { params: { path: { id: offeringId } } };

  useEffect(() => {
    api
      .GET("/api/v1/staff/teaching/{id}/notices", { params: { path: { id: offeringId } } })
      .then(({ data }) => setNotices(data?.items ?? []));
  }, [offeringId]);

  return (
    <section aria-labelledby="class-notices-title" className="flex flex-col gap-3">
      <h2 id="class-notices-title" className="text-xl font-semibold">
        Notices to this class
      </h2>
      {notices === null ? (
        <Loading label="Loading notices">
          <Skeleton className="h-24 w-full" />
        </Loading>
      ) : (
        <NoticeList
          notices={notices}
          canWithdraw={(n) => n.created_by === user.id}
          onWithdraw={async (n) => {
            const r = await run(
              () =>
                api.POST("/api/v1/staff/teaching/{id}/notices/{item}/withdraw", {
                  params: { path: { id: offeringId, item: n.id } },
                }),
              "Notice withdrawn",
            );
            if (r) setNotices(r.items);
          }}
        />
      )}
      <Card>
        <h3 className="text-lg font-semibold">Post to this class</h3>
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            const r = await run(
              () => api.POST("/api/v1/staff/teaching/{id}/notices", { ...path, body: draft }),
              "Notice posted",
            );
            if (r) {
              setNotices(r.items);
              setDraft({ ...blank, audience: "offering" });
            }
          }}
        >
          <NoticeFields value={draft} onChange={setDraft} errors={errors} prefix="class-notice" />
          <Button type="submit" variant="secondary" loading={busy} className="self-start">
            Post notice
          </Button>
        </form>
      </Card>
    </section>
  );
}
