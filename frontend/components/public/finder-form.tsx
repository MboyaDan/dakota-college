import type { ReactNode } from "react";
import { ChevronDown, Search } from "@/components/icons";
import { buttonClasses } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import {
  durationBands,
  kcseGrades,
  levelLabels,
  levelOrder,
  type FinderFilters,
} from "@/lib/public/programmes";
import type { PublicSchool } from "@/lib/public/types";

// Course finder filters as a plain GET form: it works without JavaScript and
// every result list has its own shareable URL. Filters are optional by
// nature, so the labels carry no "(optional)" suffix.

const control =
  "block w-full min-h-touch rounded-control border border-ink-muted bg-white px-3 py-2 text-base text-ink";

function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="font-semibold">
        {label}
      </label>
      {children}
    </div>
  );
}

function Choice({
  id,
  name,
  value,
  children,
}: {
  id: string;
  name: string;
  value?: string;
  children: ReactNode;
}) {
  return (
    <div className="relative">
      <select id={id} name={name} defaultValue={value ?? ""} className={cn(control, "appearance-none pr-10")}>
        {children}
      </select>
      <ChevronDown
        aria-hidden
        className="pointer-events-none absolute top-1/2 right-3 size-5 -translate-y-1/2 text-ink-muted"
      />
    </div>
  );
}

type FinderFormProps = {
  schools: PublicSchool[];
  filters?: FinderFilters;
  /** The short form on the home page shows search and school only. */
  compact?: boolean;
  idPrefix?: string;
};

export function FinderForm({ schools, filters = {}, compact = false, idPrefix = "finder" }: FinderFormProps) {
  const id = (name: string) => `${idPrefix}-${name}`;
  return (
    <form
      action="/programmes"
      role="search"
      aria-label="Find a course"
      className={cn(
        "grid items-end gap-4",
        compact ? "sm:grid-cols-[2fr_1.5fr_auto]" : "sm:grid-cols-2 lg:grid-cols-3",
      )}
    >
      <Field id={id("q")} label="Search">
        <input
          id={id("q")}
          name="q"
          type="search"
          defaultValue={filters.q}
          placeholder="For example nursing or catering"
          className={control}
        />
      </Field>
      <Field id={id("school")} label="School">
        <Choice id={id("school")} name="school" value={filters.school}>
          <option value="">All schools</option>
          {schools.map((s) => (
            <option key={s.slug} value={s.slug}>
              {s.short_name}
            </option>
          ))}
        </Choice>
      </Field>
      {compact ? null : (
        <>
          <Field id={id("level")} label="Level">
            <Choice id={id("level")} name="level" value={filters.level}>
              <option value="">All levels</option>
              {levelOrder.map((l) => (
                <option key={l} value={l}>
                  {levelLabels[l]}
                </option>
              ))}
            </Choice>
          </Field>
          <Field id={id("duration")} label="How long">
            <Choice id={id("duration")} name="duration" value={filters.duration}>
              <option value="">Any length</option>
              {durationBands.map((b) => (
                <option key={b.value} value={b.value}>
                  {b.label}
                </option>
              ))}
            </Choice>
          </Field>
          <Field id={id("grade")} label="My KCSE mean grade">
            <Choice id={id("grade")} name="grade" value={filters.grade}>
              <option value="">Any grade</option>
              {kcseGrades.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </Choice>
          </Field>
        </>
      )}
      <button type="submit" className={buttonClasses("primary")}>
        <Search aria-hidden className="size-5" />
        {compact ? "Search" : "Show programmes"}
      </button>
    </form>
  );
}
