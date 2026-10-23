import { expect, type APIRequestContext } from "@playwright/test";

// Reads the emails the API sends, caught by Mailpit (E2E_MAILPIT_URL).
export const mailpit = process.env.E2E_MAILPIT_URL ?? "";

/** Waits for the newest email to `to` sent after `since`, and returns its sign-in link path. */
export async function linkFromEmail(request: APIRequestContext, to: string, since: number): Promise<string> {
  let path = "";
  await expect
    .poll(
      async () => {
        const search = await request.get(`${mailpit}/api/v1/search`, {
          params: { query: `to:"${to}"`, limit: "1" },
        });
        const found = (await search.json()) as { messages: { ID: string; Created: string }[] };
        const latest = found.messages[0];
        if (!latest || Date.parse(latest.Created) < since) return "";
        const message = (await (await request.get(`${mailpit}/api/v1/message/${latest.ID}`)).json()) as {
          Text: string;
        };
        const m = message.Text.match(/https?:\/\/[^/\s]+(\/apply\/continue\?token=[\w-]+)/);
        path = m?.[1] ?? "";
        return path;
      },
      { timeout: 15_000, message: `sign-in email to ${to}` },
    )
    .not.toBe("");
  return path;
}
