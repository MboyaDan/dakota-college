"use client";

import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";
import { Alert } from "@/components/ui/alert";
import { Button, LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Checkbox, Input, Select, Textarea } from "@/components/ui/field";
import { Loading, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { useAction } from "@/components/staff/use-action";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { describeEntry, relationships, sortTimetable, studentStatus, weekdays } from "@/lib/staff/academics";
import { describeBalance } from "@/lib/finance";
import { formatKes } from "@/lib/money";
import { formatDate, formatNairobi } from "@/lib/staff/datetime";
import { requestKinds, requestStatus } from "@/lib/student/requests";

// The student's own pages (Stage 12b). Every call reaches only the
// signed-in student's record (AC-08).

type Dashboard = components["schemas"]["StudentDashboard"];
type Profile = components["schemas"]["StudentProfile"];
type Registration = components["schemas"]["StudentRegistration"];
type Entry = components["schemas"]["StudentTimetableEntry"];
type Notice = components["schemas"]["Notice"];
type Request = components["schemas"]["ServiceRequest"];
type GuardianInput = components["schemas"]["GuardianInput"];

/** Loads one API answer; shows structure while loading and a plain error. */
export function useLoad<T>(load: () => Promise<{ data?: T; error?: unknown }>) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    void load().then(({ data: d, error: err }) => {
      if (!active) return;
      if (d) setData(d);
      else setError(errorMessage(err));
    });
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load once per page
  }, []);
  return { data, setData, error };
}

export function Page({ title, intro, children }: { title: string; intro?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">{title}</h1>
        {intro ? <p className="max-w-prose text-ink-muted">{intro}</p> : null}
      </div>
      {children}
    </div>
  );
}

export function Loaded<T>({
  state,
  label,
  children,
}: {
  state: { data: T | null; error: string | null };
  label: string;
  children: (d: T) => ReactNode;
}) {
  if (state.error) {
    return (
      <Alert tone="danger" title="Could not load this page">
        {state.error}
      </Alert>
    );
  }
  if (!state.data) {
    return (
      <Loading label={label}>
        <SkeletonText lines={8} />
      </Loading>
    );
  }
  return <>{children(state.data)}</>;
}

// ---------- home ----------

function NoticeCard({ n }: { n: Notice }) {
  return (
    <article className="rounded-card border border-stone bg-white p-4">
      <h3 className="text-lg font-semibold text-navy-900">{n.title}</h3>
      <p className="text-sm text-ink-muted">
        {n.audience_name ? `${n.audience_name} · ` : ""}
        {n.author_name}, {formatNairobi(n.created_at)}
      </p>
      <p className="mt-2 whitespace-pre-line">{n.body}</p>
    </article>
  );
}

/** Today's weekday as the API numbers it (1 is Monday), in Nairobi. */
function todayNumber(now = new Date()): number {
  const day = new Date(now.getTime() + 3 * 60 * 60 * 1000).getUTCDay();
  return day === 0 ? 7 : day;
}

export function StudentHome() {
  const state = useLoad<Dashboard>(() => api.GET("/api/v1/student/dashboard"));
  return (
    <Loaded state={state} label="Loading your page">
      {(d) => {
        const today = todayNumber();
        const classes = sortTimetable(d.timetable).filter((e) => e.kind === "class");
        const upcoming = [
          ...classes.filter((e) => (e.weekday ?? 0) >= today),
          ...classes.filter((e) => (e.weekday ?? 0) < today),
        ].slice(0, 4);
        const st = studentStatus[d.status];
        return (
          <Page title={`Hello, ${d.name.split(" ")[0]}`}>
            <Card>
              <p>
                <span className="font-mono font-semibold">{d.student_number}</span> · {d.programme_title}
                {/* Intake cohorts are named "Programme · Intake": show only what is new. */}
                {d.cohort_name ? ` · ${d.cohort_name.replace(`${d.programme_title} · `, "")}` : ""}
              </p>
              <StatusTag status={st.tone}>{st.label}</StatusTag>
            </Card>

            {d.registration_open > 0 || d.change_request_pending || d.open_requests > 0 ? (
              <section aria-labelledby="todo-title" className="flex flex-col gap-2">
                <h2 id="todo-title" className="text-xl font-semibold">
                  To see to
                </h2>
                <ul className="flex flex-col gap-2">
                  {d.registration_open > 0 ? (
                    <li>
                      <Alert tone="warning" title="Register for your units">
                        {d.registration_open === 1 ? "1 unit is" : `${d.registration_open} units are`} open
                        for you to register.{" "}
                        <Link href="/student/units" className="text-royal underline">
                          Register now
                        </Link>
                      </Alert>
                    </li>
                  ) : null}
                  {d.change_request_pending ? (
                    <li>
                      <Alert tone="info" title="Change of details waiting">
                        The registrar has not decided on your change of details yet.
                      </Alert>
                    </li>
                  ) : null}
                  {d.open_requests > 0 ? (
                    <li>
                      <Alert tone="info" title="Requests in progress">
                        {d.open_requests === 1 ? "1 request is" : `${d.open_requests} requests are`} being
                        handled.{" "}
                        <Link href="/student/requests" className="text-royal underline">
                          See requests
                        </Link>
                      </Alert>
                    </li>
                  ) : null}
                </ul>
              </section>
            ) : null}

            <section aria-labelledby="next-title" className="flex flex-col gap-2">
              <h2 id="next-title" className="text-xl font-semibold">
                Next classes
              </h2>
              {upcoming.length === 0 ? (
                <p>No classes scheduled yet.</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {upcoming.map((e) => (
                    <li key={e.id} className="rounded-control border border-stone bg-white px-3 py-2">
                      <span className="font-semibold">{describeEntry(e)}</span>
                      <span className="block text-sm text-ink-muted">
                        <span className="font-mono">{e.unit_code}</span> {e.unit_title}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              <Link
                href="/student/timetable"
                className="min-h-touch content-center self-start text-royal underline"
              >
                Full timetable
              </Link>
            </section>

            <section aria-labelledby="home-notices-title" className="flex flex-col gap-2">
              <h2 id="home-notices-title" className="text-xl font-semibold">
                Latest notices
              </h2>
              {d.notices.length === 0 ? (
                <p>No notices.</p>
              ) : (
                d.notices.map((n) => <NoticeCard key={n.id} n={n} />)
              )}
            </section>

            <Card className="flex flex-col gap-2">
              <h2 className="text-xl font-semibold">Fees</h2>
              {d.fee_balance_cents === null ? (
                <p>
                  Nothing has been billed to you yet. Your invoices appear under Fees once they are issued.
                </p>
              ) : (
                <>
                  <p className="text-lg font-semibold tabular-nums">{describeBalance(d.fee_balance_cents)}</p>
                  {d.fee_overdue_cents > 0 ? (
                    <p className="font-semibold text-danger tabular-nums">
                      Overdue now: {formatKes(d.fee_overdue_cents)}
                    </p>
                  ) : null}
                </>
              )}
              <Link
                href="/student/fees"
                className="min-h-touch content-center self-start text-royal underline"
              >
                Fees and statement
              </Link>
            </Card>
          </Page>
        );
      }}
    </Loaded>
  );
}

// ---------- timetable ----------

export function StudentTimetable() {
  const state = useLoad<{ items: Entry[] }>(() => api.GET("/api/v1/student/timetable"));
  return (
    <Page title="Timetable" intro="Your classes and exams for the units you are registered for.">
      <Loaded state={state} label="Loading your timetable">
        {({ items }) => {
          const sorted = sortTimetable(items);
          const exams = sorted.filter((e) => e.kind === "exam");
          if (items.length === 0) {
            return (
              <EmptyState
                title="Nothing scheduled yet"
                description="Your classes appear here once you are registered for units and the college has set their times."
              />
            );
          }
          return (
            <>
              <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {weekdays
                  .map((day, i) => ({
                    day,
                    entries: sorted.filter((e) => e.kind === "class" && e.weekday === i + 1),
                  }))
                  .filter((d) => d.entries.length > 0)
                  .map((d) => (
                    <li key={d.day}>
                      <Card className="h-full">
                        <h2 className="text-lg font-semibold">{d.day}</h2>
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
              {exams.length > 0 ? (
                <section aria-labelledby="exams-title" className="flex flex-col gap-2">
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
            </>
          );
        }}
      </Loaded>
    </Page>
  );
}

// ---------- units (registration) ----------

export function StudentUnits() {
  const state = useLoad<Registration>(() => api.GET("/api/v1/student/registration"));
  const { busy, run } = useAction();
  const act = async (id: string, drop: boolean, label: string) => {
    const r = await run(
      () =>
        drop
          ? api.POST("/api/v1/student/registration/{id}/drop", { params: { path: { id } } })
          : api.POST("/api/v1/student/registration/{id}", { params: { path: { id } } }),
      label,
    );
    if (r) state.setData(r);
  };
  return (
    <Page title="Units" intro="Register for the units of your programme while registration is open.">
      <Loaded state={state} label="Loading your units">
        {(r) => (
          <>
            {!r.can_register ? (
              <Alert tone="warning" title="Registration is not open to you">
                Only active students can register. Contact the registrar&apos;s office.
              </Alert>
            ) : null}
            <section aria-labelledby="open-title" className="flex flex-col gap-3">
              <h2 id="open-title" className="text-xl font-semibold">
                Open for registration
              </h2>
              {r.open.length === 0 ? (
                <p>No registration is open for your programme right now.</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {r.open.map((o) => (
                    <li
                      key={o.offering_id}
                      className="flex flex-col gap-2 rounded-card border border-stone bg-white p-4 sm:flex-row sm:items-center sm:justify-between"
                    >
                      <span className="flex flex-col">
                        <span className="font-semibold">
                          <span className="font-mono">{o.unit_code}</span> {o.unit_title}
                        </span>
                        <span className="text-sm text-ink-muted">
                          {o.term_name} · {o.is_core ? "core" : "elective"}
                          {o.registration_closes_on
                            ? ` · closes ${formatDate(o.registration_closes_on)}`
                            : ""}
                        </span>
                      </span>
                      {o.registered ? (
                        <span className="flex items-center gap-3">
                          <StatusTag status="success">Registered</StatusTag>
                          <Button
                            variant="ghost"
                            disabled={busy}
                            onClick={() => void act(o.offering_id, true, `${o.unit_code} dropped`)}
                          >
                            Drop<span className="sr-only"> {o.unit_code}</span>
                          </Button>
                        </span>
                      ) : r.can_register ? (
                        <Button
                          variant="secondary"
                          loading={busy}
                          onClick={() => void act(o.offering_id, false, `Registered for ${o.unit_code}`)}
                        >
                          Register<span className="sr-only"> for {o.unit_code}</span>
                        </Button>
                      ) : null}
                    </li>
                  ))}
                </ul>
              )}
            </section>
            <section aria-labelledby="mine-title" className="flex flex-col gap-3">
              <h2 id="mine-title" className="text-xl font-semibold">
                Your registrations
              </h2>
              {r.current.length === 0 ? (
                <p>None yet.</p>
              ) : (
                <ul className="flex flex-col gap-1">
                  {r.current.map((c) => (
                    <li key={c.id}>
                      {c.term_name}: <span className="font-mono">{c.unit_code}</span> {c.unit_title}
                      {c.status === "dropped" ? <span className="text-ink-muted"> (dropped)</span> : null}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </>
        )}
      </Loaded>
    </Page>
  );
}

// ---------- notices ----------

export function StudentNotices() {
  const state = useLoad<{ items: Notice[] }>(() => api.GET("/api/v1/student/notices"));
  return (
    <Page title="Notices" intro="From the college, your school, your cohort and your classes.">
      <Loaded state={state} label="Loading notices">
        {({ items }) =>
          items.length === 0 ? (
            <EmptyState title="No notices" description="Notices from the college appear here." />
          ) : (
            <div className="flex flex-col gap-3">
              {items.map((n) => (
                <NoticeCard key={n.id} n={n} />
              ))}
            </div>
          )
        }
      </Loaded>
    </Page>
  );
}

// ---------- requests ----------

export function StudentRequests() {
  const state = useLoad<{ items: Request[] }>(() => api.GET("/api/v1/student/requests"));
  const { busy, errors, setErrors, run } = useAction();
  const [kind, setKind] = useState<Request["kind"]>("letter");
  const [details, setDetails] = useState("");
  return (
    <Page
      title="Requests"
      intro="Ask the registrar's office for letters, transcripts and other services, and follow them here."
    >
      <Card>
        <h2 className="text-xl font-semibold">New request</h2>
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            if (!details.trim()) {
              setErrors({ details: "Say what you need, and what it is for." });
              return;
            }
            const r = await run(
              () => api.POST("/api/v1/student/requests", { body: { kind, details } }),
              "Request sent",
            );
            if (r) {
              state.setData(r);
              setDetails("");
            }
          }}
        >
          <Select
            id="request-kind"
            name="kind"
            label="What do you need?"
            required
            value={kind}
            error={errors.kind}
            onChange={(e) => setKind(e.target.value as Request["kind"])}
          >
            {requestKinds.map((k) => (
              <option key={k.value} value={k.value}>
                {k.label}
              </option>
            ))}
          </Select>
          <Textarea
            id="request-details"
            name="details"
            label="Details"
            required
            rows={4}
            maxLength={2000}
            value={details}
            error={errors.details}
            hint="For example who the letter is for, or why you need to defer."
            onChange={(e) => setDetails(e.target.value)}
          />
          <Button type="submit" loading={busy} className="self-start">
            Send request
          </Button>
        </form>
      </Card>
      <Loaded state={state} label="Loading your requests">
        {({ items }) =>
          items.length === 0 ? (
            <p>You have not made any requests.</p>
          ) : (
            <ul className="flex flex-col gap-2">
              {items.map((r) => (
                <li key={r.id} className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-semibold">
                      {requestKinds.find((k) => k.value === r.kind)?.label}
                    </span>
                    <StatusTag status={requestStatus[r.status].tone}>
                      {requestStatus[r.status].label}
                    </StatusTag>
                  </div>
                  <p className="text-sm text-ink-muted">
                    Sent {formatNairobi(r.created_at)}
                    {r.updated_at !== r.created_at ? ` · updated ${formatNairobi(r.updated_at)}` : ""}
                  </p>
                  <p className="whitespace-pre-line">{r.details}</p>
                  {r.response ? (
                    <p className="border-l-4 border-gold pl-3 whitespace-pre-line">
                      <strong>Reply:</strong> {r.response}
                    </p>
                  ) : null}
                </li>
              ))}
            </ul>
          )
        }
      </Loaded>
    </Page>
  );
}

// ---------- profile ----------

const blankGuardian: GuardianInput = {
  full_name: "",
  relationship: "mother",
  phone: "",
  email: "",
  is_primary: false,
};

export function StudentProfile() {
  const state = useLoad<Profile>(() => api.GET("/api/v1/student/profile"));
  const { busy, errors, setErrors, run } = useAction();
  const [phone, setPhone] = useState<string | null>(null);
  const [contact, setContact] = useState<GuardianInput>(blankGuardian);
  const [change, setChange] = useState({
    first_name: "",
    middle_name: "",
    last_name: "",
    date_of_birth: "",
    identity_document: "",
    reason: "",
  });
  const done = (p: Profile | null) => {
    if (p) state.setData(p);
    return p !== null;
  };
  return (
    <Page title="Your profile">
      <Loaded state={state} label="Loading your profile">
        {(p) => {
          const pending = p.change_requests.find((c) => c.status === "pending");
          return (
            <>
              <Card>
                <h2 className="text-xl font-semibold">Your details</h2>
                <dl className="grid grid-cols-1 gap-x-4 gap-y-2 sm:grid-cols-[12rem_1fr]">
                  <dt className="text-sm text-ink-muted">Name</dt>
                  <dd>{[p.first_name, p.middle_name, p.last_name].filter(Boolean).join(" ")}</dd>
                  <dt className="text-sm text-ink-muted">Student number</dt>
                  <dd className="font-mono">{p.student_number}</dd>
                  <dt className="text-sm text-ink-muted">Programme</dt>
                  <dd>
                    {p.programme_title} ({p.school_name})
                  </dd>
                  <dt className="text-sm text-ink-muted">Cohort</dt>
                  <dd>{p.cohort_name ?? "None"}</dd>
                  <dt className="text-sm text-ink-muted">Date of birth</dt>
                  <dd>{p.date_of_birth ? formatDate(p.date_of_birth) : "Not given"}</dd>
                  <dt className="text-sm text-ink-muted">ID number</dt>
                  <dd>{p.identity_document || "Not given"}</dd>
                  <dt className="text-sm text-ink-muted">Email</dt>
                  <dd className="break-all">{p.email}</dd>
                </dl>
              </Card>

              <Card>
                <h2 className="text-xl font-semibold">Phone</h2>
                <form
                  noValidate
                  className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[1fr_auto]"
                  onSubmit={async (e) => {
                    e.preventDefault();
                    done(
                      await run(
                        () =>
                          api.PUT("/api/v1/student/profile/phone", {
                            body: { phone: phone ?? p.phone ?? "" },
                          }),
                        "Phone saved",
                        "phone.",
                      ),
                    );
                  }}
                >
                  <Input
                    id="my-phone"
                    name="phone"
                    type="tel"
                    label="Phone number"
                    required
                    autoComplete="tel"
                    value={phone ?? p.phone ?? ""}
                    error={errors["phone.phone"]}
                    onChange={(e) => setPhone(e.target.value)}
                  />
                  <Button type="submit" variant="secondary" loading={busy}>
                    Save phone
                  </Button>
                </form>
              </Card>

              <section aria-labelledby="contacts-title" className="flex flex-col gap-3">
                <h2 id="contacts-title" className="text-xl font-semibold">
                  Parents, guardians and next of kin
                </h2>
                {p.guardians.length === 0 ? <p>None recorded.</p> : null}
                <ul className="flex flex-col gap-2">
                  {p.guardians.map((g) => (
                    <li
                      key={g.id}
                      className="flex flex-col gap-1 rounded-card border border-stone bg-white p-4 sm:flex-row sm:items-center sm:justify-between"
                    >
                      <span>
                        <span className="font-semibold">{g.full_name}</span>{" "}
                        <span className="text-ink-muted">
                          ({relationships.find((r) => r.value === g.relationship)?.label})
                          {g.is_primary ? " · main contact" : ""}
                        </span>
                        <span className="block text-sm break-all">
                          {[g.phone, g.email].filter(Boolean).join(" · ")}
                        </span>
                      </span>
                      <Button
                        variant="ghost"
                        onClick={async () =>
                          done(
                            await run(
                              () =>
                                api.DELETE("/api/v1/student/guardians/{item}", {
                                  params: { path: { item: g.id } },
                                }),
                              "Contact removed",
                            ),
                          )
                        }
                      >
                        Remove<span className="sr-only"> {g.full_name}</span>
                      </Button>
                    </li>
                  ))}
                </ul>
                <Card>
                  <h3 className="text-lg font-semibold">Add a contact</h3>
                  <form
                    noValidate
                    className="grid grid-cols-1 gap-3 sm:grid-cols-2"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      if (
                        done(
                          await run(
                            () => api.POST("/api/v1/student/guardians", { body: contact }),
                            "Contact added",
                            "contact.",
                          ),
                        )
                      )
                        setContact(blankGuardian);
                    }}
                  >
                    <Input
                      id="contact-name"
                      name="full_name"
                      label="Full name"
                      required
                      maxLength={120}
                      value={contact.full_name}
                      error={errors["contact.full_name"]}
                      onChange={(e) => setContact({ ...contact, full_name: e.target.value })}
                    />
                    <Select
                      id="contact-relationship"
                      name="relationship"
                      label="Relationship to you"
                      required
                      value={contact.relationship}
                      onChange={(e) =>
                        setContact({
                          ...contact,
                          relationship: e.target.value as GuardianInput["relationship"],
                        })
                      }
                    >
                      {relationships.map((r) => (
                        <option key={r.value} value={r.value}>
                          {r.label}
                        </option>
                      ))}
                    </Select>
                    <Input
                      id="contact-phone"
                      name="phone"
                      type="tel"
                      label="Phone"
                      value={contact.phone}
                      error={errors["contact.phone"]}
                      hint="A phone number or an email address is needed."
                      onChange={(e) => setContact({ ...contact, phone: e.target.value })}
                    />
                    <Input
                      id="contact-email"
                      name="email"
                      type="email"
                      label="Email"
                      value={contact.email}
                      error={errors["contact.email"]}
                      onChange={(e) => setContact({ ...contact, email: e.target.value })}
                    />
                    <Checkbox
                      id="contact-primary"
                      name="is_primary"
                      label="Main contact"
                      checked={contact.is_primary}
                      onChange={(e) => setContact({ ...contact, is_primary: e.target.checked })}
                    />
                    <div className="flex items-end">
                      <Button type="submit" variant="secondary" loading={busy}>
                        Add contact
                      </Button>
                    </div>
                  </form>
                </Card>
              </section>

              <section aria-labelledby="change-title" className="flex flex-col gap-3">
                <h2 id="change-title" className="text-xl font-semibold">
                  Correct your name, date of birth or ID number
                </h2>
                {pending ? (
                  <Alert tone="info" title="Waiting for the registrar">
                    You asked on {formatNairobi(pending.created_at)} to change{" "}
                    {Object.entries(pending.new_values)
                      .map(([k, v]) => `${k.replace(/_/g, " ")} to “${v}”`)
                      .join(", ")}
                    . You will get an email when it is decided.
                  </Alert>
                ) : (
                  <div>
                    <Dialog
                      title="Ask for a correction"
                      description="The registrar checks these against your documents. Fill in only what is wrong."
                      triggerLabel="Ask for a correction"
                      triggerVariant="outline"
                      actions={
                        <>
                          <Button type="submit" variant="outline">
                            Cancel
                          </Button>
                          <Button
                            type="submit"
                            onClick={async () => {
                              if (!change.reason.trim()) {
                                setErrors({ "change.reason": "Say why the details are wrong." });
                                return;
                              }
                              done(
                                await run(
                                  () => api.POST("/api/v1/student/profile/change-request", { body: change }),
                                  "Sent to the registrar",
                                  "change.",
                                ),
                              );
                            }}
                          >
                            Send to the registrar
                          </Button>
                        </>
                      }
                    >
                      <div className="flex flex-col gap-3">
                        <Input
                          id="change-first"
                          name="first_name"
                          label="First name"
                          value={change.first_name}
                          onChange={(e) => setChange({ ...change, first_name: e.target.value })}
                        />
                        <Input
                          id="change-middle"
                          name="middle_name"
                          label="Middle name"
                          value={change.middle_name}
                          onChange={(e) => setChange({ ...change, middle_name: e.target.value })}
                        />
                        <Input
                          id="change-last"
                          name="last_name"
                          label="Last name"
                          value={change.last_name}
                          onChange={(e) => setChange({ ...change, last_name: e.target.value })}
                        />
                        <Input
                          id="change-dob"
                          name="date_of_birth"
                          type="date"
                          label="Date of birth"
                          value={change.date_of_birth}
                          onChange={(e) => setChange({ ...change, date_of_birth: e.target.value })}
                        />
                        <Input
                          id="change-id"
                          name="identity_document"
                          label="ID number"
                          value={change.identity_document}
                          onChange={(e) => setChange({ ...change, identity_document: e.target.value })}
                        />
                        <Textarea
                          id="change-reason"
                          name="reason"
                          label="Why"
                          required
                          rows={2}
                          maxLength={2000}
                          value={change.reason}
                          onChange={(e) => setChange({ ...change, reason: e.target.value })}
                        />
                      </div>
                    </Dialog>
                  </div>
                )}
                {Object.keys(errors).some((k) => k.startsWith("change.")) ? (
                  <Alert tone="danger" title="Not sent" live>
                    {Object.entries(errors)
                      .filter(([k]) => k.startsWith("change."))
                      .map(([, v]) => v)
                      .join(" ")}
                  </Alert>
                ) : null}
                {p.change_requests
                  .filter((c) => c.status !== "pending")
                  .map((c) => (
                    <p key={c.id} className="text-sm">
                      {formatNairobi(c.created_at)}: {c.status === "approved" ? "approved" : "declined"}
                      {c.decision ? `, ${c.decision}` : ""}
                    </p>
                  ))}
              </section>
              <LinkButton href="/student" variant="outline" className="self-start">
                Back to home
              </LinkButton>
            </>
          );
        }}
      </Loaded>
    </Page>
  );
}
