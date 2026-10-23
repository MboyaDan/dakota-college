"use client";

import { useEffect, useState } from "react";

/**
 * True once React has hydrated. Submit buttons stay disabled until then, so a
 * fast click cannot send the form natively (which would put the fields in
 * the URL with method GET). Forms also use method="post" as a backstop.
 */
export function useHydrated() {
  const [hydrated, setHydrated] = useState(false);
  // eslint-disable-next-line react-hooks/set-state-in-effect -- runs once to detect hydration
  useEffect(() => setHydrated(true), []);
  return hydrated;
}
