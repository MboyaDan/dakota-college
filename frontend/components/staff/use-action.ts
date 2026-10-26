"use client";

import { useState } from "react";
import { useToast } from "@/components/ui/toast";
import { errorMessage, isApiError } from "@/lib/api/client";

type Result<T> = { data?: T; error?: unknown; response: Response };

/**
 * Runs one API change: shows a toast, keeps per-field messages from a 422
 * (prefixed by `form` so two forms on a page keep theirs apart), and
 * returns the data on success.
 */
export function useAction() {
  const { show } = useToast();
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  async function run<T>(call: () => Promise<Result<T>>, success: string, form = ""): Promise<T | null> {
    setBusy(true);
    setErrors({});
    const { data, error, response } = await call();
    setBusy(false);
    if (response.ok) {
      if (success) show({ tone: "success", title: success });
      return (data ?? ({} as T)) as T;
    }
    if (response.status === 422 && isApiError(error) && error.error.fields?.length) {
      const fields: Record<string, string> = {};
      for (const f of error.error.fields) fields[form + f.field] ??= f.message;
      setErrors(fields);
      return null;
    }
    show({ tone: "danger", title: errorMessage(error) });
    return null;
  }

  return { busy, errors, setErrors, run };
}
