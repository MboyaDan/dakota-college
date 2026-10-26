"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Loading, SkeletonText } from "@/components/ui/skeleton";
import { Table } from "@/components/ui/table";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { describeEntry, sortTimetable, weekdays } from "@/lib/staff/academics";
import { formatDate } from "@/lib/staff/datetime";
import { ClassNotices } from "./notices";

// A trainer's own classes (academics.teach, AC-08): only what they teach.

type Teaching = components["schemas"]["Teaching"];
type Offering = components["schemas"]["Offering"];

export function MyTeaching() {
  const [teaching, setTeaching] = useState<Teaching | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.GET("/api/v1/staff/teaching").then(({ data, error: err }) => {
      if (data) setTeaching(data);
      else setError(errorMessage(err));
    });
  }, []);

  if (error) {
    return (
      <Alert tone="danger" title="Could not load your classes">
        {error}
      </Alert>
    );
  }
  if (!teaching) {
    return (
      <Loading label="Loading your classes">
        <SkeletonText lines={8} />
      </Loading>
    );
  }
  const weekly = sortTimetable(teaching.timetable).filter((e) => e.kind === "class");
  const exams = sortTimetable(teaching.timetable).filter((e) => e.kind === "exam");

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">My teaching</h1>
        <p className="max-w-prose text-ink-muted">The classes you are assigned to, and when they meet.</p>
      </div>

      {teaching.offerings.length === 0 ? (
        <EmptyState
          title="No classes assigned to you"
          description="When the registrar or your Head of School assigns you a class, it appears here."
        />
      ) : (
        <>
          <section aria-labelledby="week-title" className="flex flex-col gap-3">
            <h2 id="week-title" className="text-xl font-semibold">
              Your week
            </h2>
            {weekly.length === 0 ? <p>No weekly classes are scheduled yet.</p> : null}
            <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {weekdays
                .map((day, i) => ({ day, entries: weekly.filter((e) => e.weekday === i + 1) }))
                .filter((d) => d.entries.length > 0)
                .map((d) => (
                  <li key={d.day}>
                    <Card className="h-full">
                      <h3 className="text-lg font-semibold">{d.day}</h3>
                      <ul className="flex flex-col gap-2">
                        {d.entries.map((e) => (
                          <li key={e.id}>
                            <span className="font-semibold">
                              {e.starts_at} to {e.ends_at}
                            </span>{" "}
                            <span className="font-mono">{e.unit_code}</span> {e.unit_title}
                            {e.room ? <span className="block text-sm text-ink-muted">{e.room}</span> : null}
                          </li>
                        ))}
                      </ul>
                    </Card>
                  </li>
                ))}
            </ul>
          </section>

          {exams.length > 0 ? (
            <section aria-labelledby="exams-title" className="flex flex-col gap-3">
              <h2 id="exams-title" className="text-xl font-semibold">
                Exams
              </h2>
              <ul className="flex flex-col gap-2">
                {exams.map((e) => (
                  <li key={e.id} className="rounded-control border border-stone bg-white px-3 py-2">
                    <span className="font-mono">{e.unit_code}</span> {describeEntry(e)}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <section aria-labelledby="classes-title" className="flex flex-col gap-3">
            <h2 id="classes-title" className="text-xl font-semibold">
              Your classes
            </h2>
            <ul className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
              {teaching.offerings.map((o) => (
                <li key={o.id}>
                  <Link
                    href={`/staff/teaching/${o.id}`}
                    className="flex min-h-touch flex-col gap-1 px-4 py-3 hover:bg-ivory sm:flex-row sm:items-center sm:gap-4"
                  >
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="font-semibold text-royal">
                        <span className="font-mono">{o.unit_code}</span> {o.unit_title}
                      </span>
                      <span className="text-sm text-ink-muted">
                        {o.term_name}: {formatDate(o.starts_on)} to {formatDate(o.ends_on)}
                      </span>
                    </span>
                    <span className="text-sm">
                      {o.registered === 1 ? "1 student" : `${o.registered} students`}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          </section>
        </>
      )}
    </div>
  );
}

export function MyClass({ id }: { id: string }) {
  const [offering, setOffering] = useState<Offering | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.GET("/api/v1/staff/teaching/{id}", { params: { path: { id } } }).then(({ data, error: err }) => {
      if (data) setOffering(data);
      else setError(errorMessage(err));
    });
  }, [id]);

  if (error) {
    return (
      <Alert tone="danger" title="Could not load the class">
        {error}
      </Alert>
    );
  }
  if (!offering) {
    return (
      <Loading label="Loading the class">
        <SkeletonText lines={8} />
      </Loading>
    );
  }
  const title = `${offering.unit_code} ${offering.unit_title}`;
  const registered = offering.class_list.filter((c) => c.status === "registered");

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb items={[{ label: "My teaching", href: "/staff/teaching" }, { label: title }]} />
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold break-words">{title}</h1>
        <p className="text-ink-muted">
          {offering.term_name}: {formatDate(offering.starts_on)} to {formatDate(offering.ends_on)}
          {offering.trainers.length > 1
            ? ` · with ${offering.trainers.map((t) => t.full_name).join(", ")}`
            : ""}
        </p>
      </div>
      <section aria-labelledby="my-timetable-title" className="flex flex-col gap-2">
        <h2 id="my-timetable-title" className="text-xl font-semibold">
          When it meets
        </h2>
        {offering.timetable.length === 0 ? <p>Not scheduled yet.</p> : null}
        <ul className="flex flex-col gap-1">
          {sortTimetable(offering.timetable).map((e) => (
            <li key={e.id}>{describeEntry(e)}</li>
          ))}
        </ul>
      </section>
      <Table
        caption={`Class list (${registered.length} students)`}
        rows={registered}
        rowKey={(c) => c.registration_id}
        empty="Nobody is registered yet."
        columns={[
          {
            key: "number",
            header: "Student number",
            cell: (c) => <span className="font-mono">{c.student_number}</span>,
          },
          { key: "name", header: "Name", cell: (c) => c.name },
          { key: "cohort", header: "Cohort", cell: (c) => c.cohort_name ?? "None" },
        ]}
      />
      <ClassNotices offeringId={offering.id} />
    </div>
  );
}
