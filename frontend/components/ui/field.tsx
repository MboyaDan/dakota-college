import type { ComponentProps, ReactNode } from "react";
import { ChevronDown, CircleAlert } from "@/components/icons";
import { cn } from "@/lib/cn";

// Form controls: label above the field, optional hint, and an error that says
// what to do. Ids derive from `name` unless given, so no client hooks are
// needed and the controls work in server components.

type FieldOwnProps = {
  label: string;
  hint?: ReactNode;
  /** Plain-language instruction, e.g. "Enter your phone number, like 0712 345 678." */
  error?: string;
};

const controlBase =
  "block w-full min-h-touch rounded-control border border-ink-muted bg-white px-3 py-2 text-base text-ink placeholder:text-ink-muted/80 disabled:cursor-not-allowed disabled:bg-stone/40 aria-invalid:border-danger aria-invalid:border-2";

function ids(name: string | undefined, id: string | undefined) {
  const controlId = id ?? `field-${name ?? "unnamed"}`;
  return { controlId, hintId: `${controlId}-hint`, errorId: `${controlId}-error` };
}

function describedBy(hint: unknown, error: unknown, hintId: string, errorId: string) {
  return [hint ? hintId : null, error ? errorId : null].filter(Boolean).join(" ") || undefined;
}

function FieldShell({
  label,
  hint,
  error,
  required,
  controlId,
  hintId,
  errorId,
  children,
}: FieldOwnProps & {
  required?: boolean;
  controlId: string;
  hintId: string;
  errorId: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={controlId} className="font-semibold text-ink">
        {label}
        {required ? null : <span className="font-normal text-ink-muted"> (optional)</span>}
      </label>
      {hint ? (
        <p id={hintId} className="text-sm text-ink-muted">
          {hint}
        </p>
      ) : null}
      {children}
      {error ? (
        <p id={errorId} className="flex items-start gap-1.5 text-sm font-medium text-danger">
          <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
          <span>{error}</span>
        </p>
      ) : null}
    </div>
  );
}

export function Input({
  label,
  hint,
  error,
  id,
  name,
  className,
  required,
  ...rest
}: FieldOwnProps & ComponentProps<"input">) {
  const { controlId, hintId, errorId } = ids(name, id);
  return (
    <FieldShell {...{ label, hint, error, required, controlId, hintId, errorId }}>
      <input
        id={controlId}
        name={name}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(hint, error, hintId, errorId)}
        className={cn(controlBase, className)}
        {...rest}
      />
    </FieldShell>
  );
}

export function Textarea({
  label,
  hint,
  error,
  id,
  name,
  className,
  required,
  rows = 5,
  ...rest
}: FieldOwnProps & ComponentProps<"textarea">) {
  const { controlId, hintId, errorId } = ids(name, id);
  return (
    <FieldShell {...{ label, hint, error, required, controlId, hintId, errorId }}>
      <textarea
        id={controlId}
        name={name}
        rows={rows}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(hint, error, hintId, errorId)}
        className={cn(controlBase, "leading-normal", className)}
        {...rest}
      />
    </FieldShell>
  );
}

export function Select({
  label,
  hint,
  error,
  id,
  name,
  className,
  required,
  children,
  ...rest
}: FieldOwnProps & ComponentProps<"select">) {
  const { controlId, hintId, errorId } = ids(name, id);
  return (
    <FieldShell {...{ label, hint, error, required, controlId, hintId, errorId }}>
      <div className="relative">
        <select
          id={controlId}
          name={name}
          required={required}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(hint, error, hintId, errorId)}
          className={cn(controlBase, "appearance-none pr-10", className)}
          {...rest}
        >
          {children}
        </select>
        <ChevronDown
          aria-hidden
          className="pointer-events-none absolute top-1/2 right-3 size-5 -translate-y-1/2 text-ink-muted"
        />
      </div>
    </FieldShell>
  );
}

export function Checkbox({
  label,
  hint,
  error,
  id,
  name,
  className,
  ...rest
}: FieldOwnProps & Omit<ComponentProps<"input">, "type">) {
  const { controlId, hintId, errorId } = ids(name, id);
  return (
    <div className={cn("flex flex-col gap-1", className)}>
      <div className="flex min-h-touch items-start gap-3 py-2">
        <input
          type="checkbox"
          id={controlId}
          name={name}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(hint, error, hintId, errorId)}
          className="mt-0.5 size-6 shrink-0 accent-royal"
          {...rest}
        />
        <div className="flex flex-col gap-1">
          <label htmlFor={controlId} className="font-medium text-ink">
            {label}
          </label>
          {hint ? (
            <p id={hintId} className="text-sm text-ink-muted">
              {hint}
            </p>
          ) : null}
        </div>
      </div>
      {error ? (
        <p id={errorId} className="flex items-start gap-1.5 text-sm font-medium text-danger">
          <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
          <span>{error}</span>
        </p>
      ) : null}
    </div>
  );
}
