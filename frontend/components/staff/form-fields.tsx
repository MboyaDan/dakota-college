"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2 } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { Checkbox, Input, Select, Textarea } from "@/components/ui/field";
import { api } from "@/lib/api/client";
import { centsToInput, formatKes, parseKesToCents } from "@/lib/money";
import { isoToNairobiInput, nairobiInputToIso } from "@/lib/staff/datetime";
import { fieldId, getAt, type Doc } from "@/lib/staff/doc";
import { slugify, type ContentKind, type FieldSpec } from "@/lib/staff/kinds";
import { ImageField } from "./image-field";

type FieldsProps = {
  fields: FieldSpec[];
  doc: Doc;
  /** Dotted path prefix for nested lists, e.g. "fees.0". */
  prefix?: string;
  errors: Record<string, string>;
  onChange: (path: string, value: unknown) => void;
  disabled?: boolean;
};

export function FormFields({ fields, doc, prefix = "", errors, onChange, disabled }: FieldsProps) {
  return (
    <>
      {fields.map((f) => (
        <FieldControl
          key={f.name}
          spec={f}
          doc={doc}
          prefix={prefix}
          errors={errors}
          onChange={onChange}
          disabled={disabled}
        />
      ))}
    </>
  );
}

function FieldControl({
  spec,
  doc,
  prefix,
  errors,
  onChange,
  disabled,
}: Omit<FieldsProps, "fields"> & { spec: FieldSpec; prefix: string }) {
  const path = prefix ? `${prefix}.${spec.name}` : spec.name;
  const value = getAt(doc, path);
  const common = {
    id: fieldId(path),
    name: path,
    label: spec.label,
    hint: spec.hint,
    error: errors[path],
    disabled,
  };
  const required = "required" in spec ? spec.required : undefined;

  switch (spec.type) {
    case "text":
    case "link":
      return (
        <Input
          {...common}
          required={required}
          value={(value as string) ?? ""}
          onChange={(e) => onChange(path, e.target.value)}
        />
      );
    case "nullableText":
      return (
        <Input
          {...common}
          value={(value as string | null) ?? ""}
          onChange={(e) => onChange(path, e.target.value === "" ? null : e.target.value)}
        />
      );
    case "slug":
      return (
        <SlugField
          spec={spec}
          common={common}
          value={(value as string) ?? ""}
          source={getAt(doc, prefix ? `${prefix}.${spec.from}` : (spec.from ?? ""))}
          path={path}
          onChange={onChange}
        />
      );
    case "textarea":
      return (
        <Textarea
          {...common}
          required={required}
          rows={spec.rows}
          value={(value as string) ?? ""}
          onChange={(e) => onChange(path, e.target.value)}
        />
      );
    case "number":
      return (
        <Input
          {...common}
          required={required}
          type="number"
          inputMode="numeric"
          value={value === null || value === undefined ? "" : String(value)}
          onChange={(e) => {
            const v = e.target.value;
            onChange(path, v === "" ? (spec.nullable ? null : 0) : Number.parseInt(v, 10));
          }}
        />
      );
    case "select":
      return (
        <Select
          {...common}
          required={required}
          value={(value as string | null) ?? ""}
          onChange={(e) => onChange(path, e.target.value === "" ? null : e.target.value)}
        >
          {spec.nullable ? <option value="">Not set</option> : null}
          {spec.options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </Select>
      );
    case "date":
      return (
        <Input
          {...common}
          required={required}
          type="date"
          value={(value as string | null) ?? ""}
          onChange={(e) =>
            onChange(path, e.target.value === "" ? (spec.nullable ? null : "") : e.target.value)
          }
        />
      );
    case "datetime":
      return (
        <Input
          {...common}
          required={required}
          type="datetime-local"
          value={isoToNairobiInput(value as string | null)}
          onChange={(e) => onChange(path, nairobiInputToIso(e.target.value) ?? (spec.nullable ? null : ""))}
        />
      );
    case "checkbox":
      return (
        <Checkbox {...common} checked={Boolean(value)} onChange={(e) => onChange(path, e.target.checked)} />
      );
    case "money":
      return <MoneyField common={common} cents={(value as number) ?? 0} path={path} onChange={onChange} />;
    case "image":
      return (
        <ImageField
          id={common.id}
          label={spec.label}
          hint={spec.hint}
          error={errors[path]}
          value={(value as string | null) || null}
          onChange={(id) => onChange(path, id ?? (required ? "" : null))}
        />
      );
    case "ref":
      return (
        <RefField
          spec={spec}
          common={common}
          value={(value as string | null) ?? ""}
          path={path}
          onChange={onChange}
        />
      );
    case "list":
      return (
        <ListField
          spec={spec}
          doc={doc}
          path={path}
          errors={errors}
          onChange={onChange}
          disabled={disabled}
        />
      );
  }
}

type Common = { id: string; name: string; label: string; hint?: string; error?: string; disabled?: boolean };

function SlugField({
  common,
  value,
  source,
  path,
  onChange,
}: {
  spec: FieldSpec;
  common: Common;
  value: string;
  source: unknown;
  path: string;
  onChange: (p: string, v: unknown) => void;
}) {
  return (
    <div className="flex flex-col gap-2">
      <Input
        {...common}
        required
        hint={common.hint ?? "Lowercase words joined by hyphens. It becomes part of the page address."}
        value={value}
        onChange={(e) => onChange(path, e.target.value)}
      />
      {typeof source === "string" && source && slugify(source) !== value ? (
        <Button
          variant="ghost"
          className="self-start px-0"
          onClick={() => onChange(path, slugify(source))}
          disabled={common.disabled}
        >
          Use “{slugify(source)}”
        </Button>
      ) : null}
    </div>
  );
}

function MoneyField({
  common,
  cents,
  path,
  onChange,
}: {
  common: Common;
  cents: number;
  path: string;
  onChange: (p: string, v: unknown) => void;
}) {
  const [text, setText] = useState(centsToInput(cents));
  const [localError, setLocalError] = useState<string | undefined>();
  return (
    <Input
      {...common}
      inputMode="decimal"
      required
      hint={
        parseKesToCents(text) !== null
          ? formatKes(parseKesToCents(text)!)
          : "Shillings, for example 45,000 or 1,250.50."
      }
      error={localError ?? common.error}
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        const parsed = parseKesToCents(e.target.value);
        setLocalError(parsed === null ? "Enter an amount in shillings, like 45,000 or 1,250.50." : undefined);
        if (parsed !== null) onChange(path, parsed);
      }}
    />
  );
}

type Option = { id: string; label: string };
const refCache = new Map<ContentKind, Promise<Option[]>>();

function loadOptions(kind: ContentKind): Promise<Option[]> {
  if (!refCache.has(kind)) {
    refCache.set(
      kind,
      api
        .GET("/api/v1/staff/content/{kind}", { params: { path: { kind } } })
        .then(({ data }) =>
          (data?.items ?? [])
            .filter((i) => i.status !== "archived")
            .map((i) => ({ id: i.id, label: i.context ? `${i.label} (${i.context})` : i.label })),
        ),
    );
  }
  return refCache.get(kind)!;
}

function RefField({
  spec,
  common,
  value,
  path,
  onChange,
}: {
  spec: Extract<FieldSpec, { type: "ref" }>;
  common: Common;
  value: string;
  path: string;
  onChange: (p: string, v: unknown) => void;
}) {
  const [options, setOptions] = useState<Option[] | null>(null);
  useEffect(() => {
    let active = true;
    loadOptions(spec.refKind).then((o) => active && setOptions(o));
    return () => {
      active = false;
    };
  }, [spec.refKind]);
  return (
    <Select
      {...common}
      required={spec.required}
      value={value}
      onChange={(e) => onChange(path, e.target.value === "" ? (spec.nullable ? null : "") : e.target.value)}
    >
      <option value="">{options === null ? "Loading…" : spec.nullable ? "None" : "Choose…"}</option>
      {(options ?? []).map((o) => (
        <option key={o.id} value={o.id}>
          {o.label}
        </option>
      ))}
    </Select>
  );
}

function ListField({
  spec,
  doc,
  path,
  errors,
  onChange,
  disabled,
}: {
  spec: Extract<FieldSpec, { type: "list" }>;
  doc: Doc;
  path: string;
  errors: Record<string, string>;
  onChange: (p: string, v: unknown) => void;
  disabled?: boolean;
}) {
  const items = (getAt(doc, path) as Record<string, unknown>[] | undefined) ?? [];
  return (
    <fieldset className="flex flex-col gap-4 rounded-card border border-stone bg-white p-4">
      <legend className="px-1 font-serif text-xl font-semibold text-navy-900">{spec.label}</legend>
      {spec.hint ? <p className="text-sm text-ink-muted">{spec.hint}</p> : null}
      {errors[path] ? <p className="text-sm font-medium text-danger">{errors[path]}</p> : null}
      {items.length === 0 ? <p className="text-ink-muted">None yet.</p> : null}
      {items.map((_, i) => (
        <div key={i} className="flex flex-col gap-4 rounded-control border border-stone p-4">
          <div className="flex items-center justify-between gap-2">
            <p className="font-semibold">
              {spec.itemLabel} {i + 1}
            </p>
            <Button
              variant="ghost"
              disabled={disabled}
              icon={<Trash2 aria-hidden className="size-4" />}
              onClick={() =>
                onChange(
                  path,
                  items.filter((_, j) => j !== i),
                )
              }
            >
              Remove
              <span className="sr-only">
                {" "}
                {spec.itemLabel.toLowerCase()} {i + 1}
              </span>
            </Button>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <FormFields
              fields={spec.fields}
              doc={doc}
              prefix={`${path}.${i}`}
              errors={errors}
              onChange={onChange}
              disabled={disabled}
            />
          </div>
        </div>
      ))}
      <Button
        variant="outline"
        className="self-start"
        disabled={disabled}
        icon={<Plus aria-hidden className="size-4" />}
        onClick={() => onChange(path, [...items, spec.newItem()])}
      >
        Add {spec.itemLabel.toLowerCase()}
      </Button>
    </fieldset>
  );
}
