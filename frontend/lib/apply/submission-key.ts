// One Idempotency-Key per submit attempt, kept for this tab, so a retry
// after a lost connection (even after a reload) is recognised as the same
// attempt. Storage can be unavailable; then the key lives for the page.

const memory = new Map<string, string>();
const storageKey = (id: string) => `dtc-submit-key:${id}`;

function newKey(): string {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export const submissionKey = {
  get(id: string): string {
    let key = memory.get(id);
    try {
      key ??= sessionStorage.getItem(storageKey(id)) ?? undefined;
    } catch {}
    key ??= newKey();
    memory.set(id, key);
    try {
      sessionStorage.setItem(storageKey(id), key);
    } catch {}
    return key;
  },
  clear(id: string) {
    memory.delete(id);
    try {
      sessionStorage.removeItem(storageKey(id));
    } catch {}
  },
};
