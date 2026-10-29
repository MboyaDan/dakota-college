/**
 * Reads named text fields from a form when it is sent. Sign-in forms leave
 * their inputs uncontrolled so text typed before the page's scripts load is
 * not wiped when React takes over the page (hydration).
 */
export function fields<K extends string>(form: HTMLFormElement, ...names: K[]): Record<K, string> {
  const data = new FormData(form);
  return Object.fromEntries(names.map((n) => [n, String(data.get(n) ?? "")])) as Record<K, string>;
}
