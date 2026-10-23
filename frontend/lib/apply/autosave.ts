"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export type SaveOutcome =
  | { ok: true }
  /** The API refused some values; nothing is retried until they change. */
  | { ok: false; fields: Record<string, string> }
  /** Could not save (network, server); kept as unsaved and retried on the next change or flush. */
  | { ok: false; message: string };

export type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "saved"; at: Date }
  | { kind: "invalid" }
  | { kind: "failed"; message: string };

/** Wait after the last change before saving, so typing does not send a request per key. */
export const AUTOSAVE_DELAY_MS = 1200;

/**
 * Keeps a form step's values and saves them a moment after each change.
 * Saves run one at a time in order, so an older save never overwrites a
 * newer one. `flush` saves at once (for "Save and continue") and says
 * whether everything is saved.
 */
export function useAutosave<T>(initial: T, save: (value: T) => Promise<SaveOutcome>) {
  const [value, setValue] = useState(initial);
  const [state, setState] = useState<SaveState>({ kind: "idle" });
  const [fields, setFields] = useState<Record<string, string>>({});
  const latest = useRef(value);
  const unsaved = useRef(false);
  const invalid = useRef(false);
  const queue = useRef<Promise<boolean>>(Promise.resolve(true));
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const saveRef = useRef(save);
  useEffect(() => {
    saveRef.current = save;
  }, [save]);

  const run = useCallback((): Promise<boolean> => {
    clearTimeout(timer.current);
    queue.current = queue.current.then(async () => {
      let saved = false;
      // A change made while saving is saved straight after, in order.
      while (unsaved.current) {
        unsaved.current = false;
        setState({ kind: "saving" });
        const outcome = await saveRef.current(latest.current);
        if ("fields" in outcome) {
          invalid.current = true;
          setFields(outcome.fields);
          setState({ kind: "invalid" });
          return false;
        }
        if ("message" in outcome) {
          unsaved.current = true;
          setState({ kind: "failed", message: outcome.message });
          return false;
        }
        invalid.current = false;
        setFields({});
        saved = true;
      }
      if (saved) setState({ kind: "saved", at: new Date() });
      return !invalid.current;
    });
    return queue.current;
  }, []);

  /** Changes the values and schedules a save. */
  const update = useCallback(
    (next: T | ((prev: T) => T)) => {
      setValue((prev) => {
        const v = typeof next === "function" ? (next as (p: T) => T)(prev) : next;
        latest.current = v;
        return v;
      });
      unsaved.current = true;
      clearTimeout(timer.current);
      timer.current = setTimeout(() => void run(), AUTOSAVE_DELAY_MS);
    },
    [run],
  );

  // Leaving the step (or the page, within the app) saves what is pending.
  useEffect(
    () => () => {
      if (unsaved.current) void run();
    },
    [run],
  );

  return { value, update, state, fields, flush: run };
}
