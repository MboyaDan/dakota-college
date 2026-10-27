"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/field";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api, errorMessage } from "@/lib/api/client";
import { weightTotal, type Scheme } from "@/lib/staff/results";
import { useAction } from "./use-action";

// How a programme assesses its units (DQ-07, D-150), set by academics.manage
// within the viewer's schools. The backend checks the rules; the running
// total only helps.

type Option = { id: string; label: string };

// Starting rows when someone switches a competency programme to marks. They
// are a form starting point to edit, not college policy (A-35).
const starter: Pick<Scheme, "components" | "pass_mark" | "grade_bands"> = {
  components: [
    { name: "Continuous assessment", weight: 30 },
    { name: "Final exam", weight: 70 },
  ],
  pass_mark: 40,
  grade_bands: [
    { grade: "A", min: 70 },
    { grade: "B", min: 60 },
    { grade: "C", min: 50 },
    { grade: "D", min: 40 },
    { grade: "E", min: 0 },
  ],
};

const whole = (v: string) => (v.trim() === "" || !Number.isFinite(Number(v)) ? 0 : Math.trunc(Number(v)));

export function ProgrammeAssessment({ programmes }: { programmes: Option[] }) {
  const [programme, setProgramme] = useState("");
  const [scheme, setScheme] = useState<Scheme | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const { busy, errors, run } = useAction();

  useEffect(() => {
    if (!programme) return;
    let active = true;
    api
      .GET("/api/v1/staff/programmes/{id}/assessment", { params: { path: { id: programme } } })
      .then(({ data, error }) => {
        if (!active) return;
        if (data) setScheme(data);
        else setLoadError(errorMessage(error));
      });
    return () => {
      active = false;
    };
  }, [programme]);

  const marks = scheme?.kind === "marks";
  const total = scheme ? weightTotal(scheme.components) : 0;
  const set = (patch: Partial<Scheme>) => setScheme(scheme ? { ...scheme, ...patch } : scheme);

  return (
    <div className="flex flex-col gap-4">
      <p className="max-w-prose">
        How each programme&apos;s units are marked. Classes that already have marks keep the assessment they
        started with; a change applies to classes marked after it.
      </p>
      <Select
        id="assessment-programme"
        name="programme"
        label="Programme"
        required
        value={programme}
        onChange={(e) => {
          setScheme(null);
          setLoadError(null);
          setProgramme(e.target.value);
        }}
      >
        <option value="">Choose a programme</option>
        {programmes.map((p) => (
          <option key={p.id} value={p.id}>
            {p.label}
          </option>
        ))}
      </Select>
      {loadError ? (
        <Alert tone="danger" title="Could not load the assessment">
          {loadError}
        </Alert>
      ) : null}
      {programme && !scheme && !loadError ? (
        <Loading label="Loading the assessment">
          <Skeleton className="h-40 w-full" />
        </Loading>
      ) : null}
      {programme && scheme ? (
        <form
          noValidate
          className="flex flex-col gap-5"
          onSubmit={async (e) => {
            e.preventDefault();
            const body: Scheme = marks
              ? { ...scheme, is_default: undefined }
              : { kind: "competency", components: [], pass_mark: 0, grade_bands: [] };
            const saved = await run(
              () =>
                api.PUT("/api/v1/staff/programmes/{id}/assessment", {
                  params: { path: { id: programme } },
                  body,
                }),
              "Assessment saved",
              "scheme.",
            );
            if (saved) setScheme(saved);
          }}
        >
          {scheme.is_default ? (
            <Alert tone="warning" title="Not set yet">
              This programme uses the placeholder assessment below. Replace it with the college&apos;s own and
              save.
            </Alert>
          ) : null}
          <Select
            id="assessment-kind"
            name="kind"
            label="How students are assessed"
            required
            value={scheme.kind}
            error={errors["scheme.kind"]}
            onChange={(e) => {
              const kind = e.target.value as Scheme["kind"];
              set(kind === "marks" && scheme.components.length === 0 ? { kind, ...starter } : { kind });
            }}
          >
            <option value="marks">Marks and grades</option>
            <option value="competency">Competent or not yet competent</option>
          </Select>

          {marks ? (
            <>
              <fieldset className="flex flex-col gap-3">
                <legend className="mb-1 font-semibold">Parts of the assessment</legend>
                <p className="text-sm text-ink-muted">1 to 6 parts. Their weights add up to 100.</p>
                {scheme.components.map((c, i) => (
                  <div
                    key={i}
                    className="grid grid-cols-1 items-end gap-3 rounded-card border border-stone bg-white p-3 sm:grid-cols-[1fr_8rem_auto]"
                  >
                    <Input
                      id={`part-${i}-name`}
                      name={`part-${i}-name`}
                      label="Part"
                      required
                      maxLength={60}
                      value={c.name}
                      onChange={(e) =>
                        set({
                          components: scheme.components.map((x, j) =>
                            j === i ? { ...x, name: e.target.value } : x,
                          ),
                        })
                      }
                    />
                    <Input
                      id={`part-${i}-weight`}
                      name={`part-${i}-weight`}
                      label="Weight (%)"
                      required
                      inputMode="numeric"
                      value={String(c.weight)}
                      onChange={(e) =>
                        set({
                          components: scheme.components.map((x, j) =>
                            j === i ? { ...x, weight: whole(e.target.value) } : x,
                          ),
                        })
                      }
                    />
                    <Button
                      variant="ghost"
                      disabled={scheme.components.length <= 1}
                      onClick={() => set({ components: scheme.components.filter((_, j) => j !== i) })}
                    >
                      Remove<span className="sr-only"> {c.name || `part ${i + 1}`}</span>
                    </Button>
                  </div>
                ))}
                <p
                  aria-live="polite"
                  className={total === 100 ? "text-success" : "font-semibold text-danger"}
                >
                  The weights add up to {total}
                  {total === 100 ? "." : ". They must add up to 100."}
                </p>
                {errors["scheme.components"] ? (
                  <p className="font-medium text-danger">{errors["scheme.components"]}</p>
                ) : null}
                <Button
                  variant="outline"
                  className="self-start"
                  disabled={scheme.components.length >= 6}
                  onClick={() => set({ components: [...scheme.components, { name: "", weight: 0 }] })}
                >
                  Add a part
                </Button>
              </fieldset>

              <div className="sm:max-w-xs">
                <Input
                  id="assessment-pass"
                  name="pass_mark"
                  label="Pass mark"
                  required
                  inputMode="numeric"
                  hint="The lowest total that passes, from 0 to 100."
                  value={String(scheme.pass_mark)}
                  error={errors["scheme.pass_mark"]}
                  onChange={(e) => set({ pass_mark: whole(e.target.value) })}
                />
              </div>

              <fieldset className="flex flex-col gap-3">
                <legend className="mb-1 font-semibold">Grades</legend>
                <p className="text-sm text-ink-muted">
                  From the highest grade to the lowest. Each grade starts at the total shown; the last starts
                  at 0.
                </p>
                {scheme.grade_bands.map((b, i) => (
                  <div
                    key={i}
                    className="grid grid-cols-2 items-end gap-3 rounded-card border border-stone bg-white p-3 sm:grid-cols-[8rem_10rem_auto]"
                  >
                    <Input
                      id={`grade-${i}-name`}
                      name={`grade-${i}-name`}
                      label="Grade"
                      required
                      maxLength={3}
                      value={b.grade}
                      onChange={(e) =>
                        set({
                          grade_bands: scheme.grade_bands.map((x, j) =>
                            j === i ? { ...x, grade: e.target.value } : x,
                          ),
                        })
                      }
                    />
                    <Input
                      id={`grade-${i}-min`}
                      name={`grade-${i}-min`}
                      label="From total"
                      required
                      inputMode="numeric"
                      value={String(b.min)}
                      onChange={(e) =>
                        set({
                          grade_bands: scheme.grade_bands.map((x, j) =>
                            j === i ? { ...x, min: whole(e.target.value) } : x,
                          ),
                        })
                      }
                    />
                    <Button
                      variant="ghost"
                      className="col-span-2 justify-self-start sm:col-span-1"
                      disabled={scheme.grade_bands.length <= 1}
                      onClick={() => set({ grade_bands: scheme.grade_bands.filter((_, j) => j !== i) })}
                    >
                      Remove<span className="sr-only"> grade {b.grade || i + 1}</span>
                    </Button>
                  </div>
                ))}
                {errors["scheme.grade_bands"] ? (
                  <p className="font-medium text-danger">{errors["scheme.grade_bands"]}</p>
                ) : null}
                <Button
                  variant="outline"
                  className="self-start"
                  disabled={scheme.grade_bands.length >= 12}
                  onClick={() => set({ grade_bands: [...scheme.grade_bands, { grade: "", min: 0 }] })}
                >
                  Add a grade
                </Button>
              </fieldset>
            </>
          ) : (
            <p>Trainers record each student as Competent or Not yet competent. Competent is a pass.</p>
          )}

          <Button type="submit" variant="secondary" loading={busy} className="self-start">
            Save assessment
          </Button>
        </form>
      ) : null}
    </div>
  );
}
