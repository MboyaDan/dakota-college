"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input, Textarea } from "@/components/ui/field";
import { Loading, SkeletonText } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { api, isApiError, type ApiError } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { fieldErrors, fieldId } from "@/lib/staff/doc";
import { useSession } from "./session";

type Setting = components["schemas"]["Setting"];

const groups: { title: string; prefix: string }[] = [
  { title: "Contact details", prefix: "contact." },
  { title: "Campus", prefix: "campus." },
  { title: "About the college", prefix: "about." },
  { title: "Social media", prefix: "social." },
  { title: "System", prefix: "" },
];

const long = new Set(["about.mission", "about.vision", "campus.directions", "contact.address"]);

export function SettingsForm() {
  const { can } = useSession();
  const { show } = useToast();
  const [settings, setSettings] = useState<Setting[] | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [changed, setChanged] = useState<Set<string>>(new Set());
  const [problem, setProblem] = useState<ApiError["error"] | null>(null);
  const [busy, setBusy] = useState(false);
  const editable = can("content.publish");

  useEffect(() => {
    api.GET("/api/v1/staff/settings").then(({ data }) => {
      if (!data) return;
      setSettings(data.settings);
      setValues(Object.fromEntries(data.settings.map((s) => [s.key, s.value])));
    });
  }, []);

  async function save() {
    setBusy(true);
    setProblem(null);
    const body = { values: Object.fromEntries([...changed].map((k) => [k, values[k] ?? ""])) };
    const { error, response } = await api.PUT("/api/v1/staff/settings", { body });
    setBusy(false);
    if (response.ok) {
      setChanged(new Set());
      show({
        tone: "success",
        title: "Settings saved",
        description: "The site updates within a few minutes.",
      });
    } else {
      setProblem(
        isApiError(error) ? error.error : { code: "network", message: "We could not reach the server." },
      );
    }
  }

  if (!settings) {
    return (
      <Loading label="Loading settings">
        <SkeletonText lines={8} />
      </Loading>
    );
  }
  const errors = fieldErrors(problem?.fields);
  const used = new Set<string>();

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-semibold">Site settings</h1>
        <p className="text-ink-muted">
          Facts shown across the website. Leave a field empty to hide it. Changes need a publisher and apply
          straight away.
        </p>
      </div>
      {problem ? (
        <Alert tone="danger" title="Some settings need attention" live>
          {problem.message}
        </Alert>
      ) : null}
      <form
        className="flex flex-col gap-8"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        {groups.map((g) => {
          const items = settings.filter((s) => !used.has(s.key) && s.key.startsWith(g.prefix));
          items.forEach((s) => used.add(s.key));
          if (items.length === 0) return null;
          return (
            <fieldset
              key={g.title}
              className="flex flex-col gap-4 rounded-card border border-stone bg-white p-4"
            >
              <legend className="px-1 font-serif text-xl font-semibold text-navy-900">{g.title}</legend>
              {items.map((s) => {
                const Field = long.has(s.key) ? Textarea : Input;
                return (
                  <Field
                    key={s.key}
                    id={fieldId(s.key)}
                    name={s.key}
                    label={s.description}
                    error={errors[s.key]}
                    disabled={!editable || busy}
                    value={values[s.key] ?? ""}
                    onChange={(e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
                      setValues((v) => ({ ...v, [s.key]: e.target.value }));
                      setChanged((c) => new Set(c).add(s.key));
                    }}
                  />
                );
              })}
            </fieldset>
          );
        })}
        {editable ? (
          <div className="sticky bottom-0 border-t border-stone bg-ivory py-3">
            <Button type="submit" loading={busy} disabled={changed.size === 0}>
              Save settings
            </Button>
          </div>
        ) : (
          <p className="text-ink-muted">Only a Registrar or Principal can change these.</p>
        )}
      </form>
    </div>
  );
}
