// Helpers for editing nested documents by path, e.g. "fees.0.amount_cents".
// API field paths use brackets ("fees[0].amount_cents"); toPath converts them.

export type Doc = Record<string, unknown>;

export function toPath(apiField: string): string {
  return apiField.replace(/\[(\d+)\]/g, ".$1");
}

export function getAt(doc: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((cur, key) => {
    if (cur === null || cur === undefined) return undefined;
    return (cur as Record<string, unknown>)[key];
  }, doc);
}

/** Returns a copy of doc with value set at path (copies along the path only). */
export function setAt<T>(doc: T, path: string, value: unknown): T {
  const [head, ...rest] = path.split(".");
  const container: unknown = Array.isArray(doc) ? [...doc] : { ...(doc as object) };
  const c = container as Record<string, unknown>;
  c[head] =
    rest.length === 0 ? value : setAt(c[head] ?? (/^\d+$/.test(rest[0]) ? [] : {}), rest.join("."), value);
  return container as T;
}

/** Field problems from the API, keyed by dotted path. */
export function fieldErrors(
  fields: { field: string; message: string }[] | undefined,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const f of fields ?? []) {
    const key = toPath(f.field);
    if (!(key in out)) out[key] = f.message;
  }
  return out;
}

/** Turns a dotted path into a safe element id. */
export function fieldId(path: string): string {
  return `f-${path.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
}
