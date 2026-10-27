import { errorMessage } from "./client";
import type { components } from "./schema";

type FileLink = components["schemas"]["FileLink"];

/**
 * Opens a five-minute file link in a new tab: the tab opens on the click,
 * while the browser still allows it, and is pointed at the link once known.
 * Returns a message to show when the link could not be made.
 */
export async function openFileLink(
  call: () => Promise<{ data?: FileLink; error?: unknown }>,
): Promise<string | null> {
  const tab = window.open("", "_blank");
  const { data, error } = await call();
  if (!data) {
    tab?.close();
    return errorMessage(error);
  }
  if (tab) tab.location.href = data.url;
  else window.location.href = data.url;
  return null;
}
