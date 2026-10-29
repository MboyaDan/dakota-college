"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog } from "@/components/ui/dialog";
import { Checkbox } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api, errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { useSession } from "./session";
import { useAction } from "./use-action";

// Which schools each member of staff works in (users.manage, D-133).
// Without "work across all schools", student records and academics reach
// only these schools; with none, nothing.

type User = components["schemas"]["AdminUser"];
type Option = { id: string; label: string };

export function StaffSchools() {
  const [users, setUsers] = useState<User[] | null>(null);
  const [schools, setSchools] = useState<Option[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.GET("/api/v1/admin/users").then(({ data, error: err }) => {
      if (data) setUsers(data.users.filter((u) => u.status === "active" && !u.role_keys.includes("student")));
      else setError(errorMessage(err));
    });
    api.GET("/api/v1/staff/content/{kind}", { params: { path: { kind: "schools" } } }).then(({ data }) => {
      if (data) setSchools(data.items.map((s) => ({ id: s.id, label: s.label })));
    });
  }, []);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Staff schools</h1>
        <p className="max-w-prose text-ink-muted">
          Heads of School see students and manage units only in the schools ticked here. Roles that work
          across every school, like the registrar, do not need any.
        </p>
      </div>
      {error ? (
        <Alert tone="danger" title="Could not load staff">
          {error}
        </Alert>
      ) : null}
      {users === null && !error ? (
        <Loading label="Loading staff">
          <Skeleton className="h-40 w-full" />
        </Loading>
      ) : null}
      <ul className="flex flex-col gap-4">
        {(users ?? []).map((u) => (
          <li key={u.id}>
            <UserSchools user={u} schools={schools} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function UserSchools({ user, schools }: { user: User; schools: Option[] }) {
  const { busy, run } = useAction();
  const [saved, setSaved] = useState<string[] | null>(null);
  const [chosen, setChosen] = useState<string[]>([]);

  useEffect(() => {
    api.GET("/api/v1/admin/users/{id}/schools", { params: { path: { id: user.id } } }).then(({ data }) => {
      const ids = (data?.items ?? []).map((s) => s.id);
      setSaved(ids);
      setChosen(ids);
    });
  }, [user.id]);

  const changed =
    saved !== null && (chosen.length !== saved.length || chosen.some((id) => !saved.includes(id)));
  const slug = user.id.slice(0, 8);

  return (
    <Card>
      <form
        className="flex flex-col gap-2"
        onSubmit={async (e) => {
          e.preventDefault();
          const res = await run(
            () =>
              api.PUT("/api/v1/admin/users/{id}/schools", {
                params: { path: { id: user.id } },
                body: { school_ids: chosen },
              }),
            `Schools saved for ${user.full_name}`,
          );
          if (res) setSaved(res.items.map((s) => s.id));
        }}
      >
        <fieldset className="flex flex-col">
          <legend className="flex flex-col">
            <span className="text-lg font-semibold">{user.full_name}</span>
            <span className="text-sm break-all text-ink-muted">
              {user.email} · {user.role_keys.map((r) => r.replace(/_/g, " ")).join(", ") || "no role"}
            </span>
          </legend>
          {saved === null ? (
            <p className="text-sm text-ink-muted">Loading…</p>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2">
              {schools.map((s) => (
                <Checkbox
                  key={s.id}
                  id={`school-${slug}-${s.id.slice(0, 8)}`}
                  name="school"
                  label={s.label}
                  checked={chosen.includes(s.id)}
                  onChange={(e) =>
                    setChosen(e.target.checked ? [...chosen, s.id] : chosen.filter((x) => x !== s.id))
                  }
                />
              ))}
            </div>
          )}
        </fieldset>
        <Button type="submit" variant="secondary" loading={busy} disabled={!changed} className="self-start">
          Save<span className="sr-only"> schools for {user.full_name}</span>
        </Button>
      </form>
      {user.two_step_enabled ? <TwoStepReset user={user} /> : null}
    </Card>
  );
}

/** For a lost phone: turns off the person's two-step sign-in (D-209). */
function TwoStepReset({ user }: { user: User }) {
  const { user: me } = useSession();
  const { run } = useAction();
  const [on, setOn] = useState(true);

  return (
    <div className="flex flex-col items-start gap-2 border-t border-stone pt-3">
      <p className="text-sm">
        {on ? "Uses two-step sign-in." : "Two-step sign-in was reset. They can set it up again."}
      </p>
      {on && me.id !== user.id ? (
        <Dialog
          title={`Reset two-step sign-in for ${user.full_name}?`}
          description="Do this only when they have lost their phone and their recovery codes, and you have confirmed it is really them, in person or by a call to a number you already have."
          triggerLabel="Reset two-step sign-in"
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
                      api.POST("/api/v1/admin/users/{id}/two-step/reset", {
                        params: { path: { id: user.id } },
                      }),
                    `Two-step sign-in reset for ${user.full_name}`,
                  ).then((res) => {
                    if (res) setOn(false);
                  })
                }
              >
                Reset
              </Button>
            </>
          }
        >
          <p>
            They will be signed out everywhere, and can then sign in with their password alone until they set
            two-step sign-in up again on their new phone.
          </p>
        </Dialog>
      ) : null}
    </div>
  );
}
