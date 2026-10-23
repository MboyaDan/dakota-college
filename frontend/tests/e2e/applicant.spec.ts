import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 8b: applicants sign in by emailed link (D-96), against the real API.
// The link tests read the email from Mailpit (E2E_MAILPIT_URL).
const password = process.env.E2E_STAFF_PASSWORD ?? "";
const mailpit = process.env.E2E_MAILPIT_URL ?? "";
test.skip(process.env.E2E_API !== "1", "set E2E_API=1 with the backend running");

/** Waits for the newest email to `to` sent after `since`, and returns its sign-in link path. */
async function linkFromEmail(request: APIRequestContext, to: string, since: number): Promise<string> {
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

async function followLink(page: Page, path: string) {
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: "Continue your application" })).toBeVisible();
  // The token is taken out of the address bar before anything else happens.
  await expect(page).toHaveURL(/\/apply\/continue$/);
  await expectNoSeriousA11yIssues(page);
  await page.getByRole("button", { name: "Continue to my application" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Your account" })).toBeVisible();
}

test("the applicant pages work at phone width without a session", async ({ page }) => {
  for (const [path, heading] of [
    ["/apply/start", "Start or continue your application"],
    ["/apply/resume", "Continue with your reference number"],
    ["/apply/continue", "Continue your application"],
    ["/apply/account", "Your account"],
  ]) {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
    expect(await hasHorizontalOverflow(page)).toBe(false);
    await expectNoSeriousA11yIssues(page);
  }
  await expect(page.getByText("Please sign in")).toBeVisible();
  await page.goto("/apply/continue");
  await expect(page.getByText("This link is incomplete")).toBeVisible();
});

test("a fixable mistake is explained next to the field", async ({ page }) => {
  await page.goto("/apply/start");
  await page.getByLabel("Email address").fill("not an email");
  await page.getByRole("button", { name: "Email me a link" }).click();
  await expect(page.locator("#field-email-error")).toBeVisible();
  await expect(page.getByLabel("Email address")).toHaveAttribute("aria-invalid", "true");
});

test("a new applicant signs in with the emailed link", async ({ page, request }, testInfo) => {
  test.skip(!mailpit, "set E2E_MAILPIT_URL to read the sign-in email");
  const email = `e2e-applicant-${testInfo.project.name}-${Date.now()}@example.invalid`;
  const since = Date.now() - 2_000;

  await page.goto("/apply");
  await page.getByRole("link", { name: "Email me a sign-in link" }).click();
  await page.getByLabel("Your name").fill("Test Applicant E2E");
  await page.getByLabel("Email address").fill(email);
  await page.getByRole("button", { name: "Email me a link" }).click();
  await expect(page.getByText("Check your email")).toBeVisible();
  await expectNoSeriousA11yIssues(page);

  const path = await linkFromEmail(request, email, since);
  await followLink(page, path);
  await expect(page.getByText(`Signed in as ${email}`)).toBeVisible();
  await expect(page.getByText("No applications yet")).toBeVisible();
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);

  // The link works once.
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/apply$/);
  await page.goto(path);
  await page.getByRole("button", { name: "Continue to my application" }).click();
  await expect(page.getByText("This link has expired or was already used.")).toBeVisible();

  // An applicant session does not open the staff portal.
  await followLink(page, await requestAnotherLink(page, request, email));
  await page.goto("/staff");
  await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
});

async function requestAnotherLink(page: Page, request: APIRequestContext, email: string) {
  const since = Date.now() - 1_000;
  await page.goto("/apply/start");
  await page.getByLabel("Email address").fill(email);
  await page.getByRole("button", { name: "Email me a link" }).click();
  await expect(page.getByText("Check your email")).toBeVisible();
  return linkFromEmail(request, email, since);
}

test("an applicant continues with their reference number", async ({ page, request }, testInfo) => {
  test.skip(!mailpit, "set E2E_MAILPIT_URL to read the sign-in email");
  // One project only: signing in spends every link for the address, so two
  // projects using the seeded applicant at once would spend each other's.
  test.skip(testInfo.project.name !== "mobile", "runs once, at phone width");
  const since = Date.now() - 2_000;

  await page.goto("/apply/resume");
  await page.getByLabel("Reference number").fill("dtc-app-sample-000001");
  await page.getByLabel("Email address on the application").fill("applicant001@example.invalid");
  await page.getByRole("button", { name: "Email me a link" }).click();
  await expect(page.getByText("Check your email")).toBeVisible();

  await followLink(page, await linkFromEmail(request, "applicant001@example.invalid", since));
  const card = page.getByRole("listitem").filter({ hasText: "DTC-APP-SAMPLE-000001" });
  await expect(
    card.getByRole("heading", { name: "Diploma in Example Information Technology (Sample)" }),
  ).toBeVisible();
  await expect(card.getByText("Not submitted")).toBeVisible();
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);
});

test.describe("staff message templates", () => {
  test.skip(!password, "set E2E_STAFF_PASSWORD");

  async function signIn(page: Page, email: string) {
    await page.goto("/staff/login");
    await page.getByLabel("Email address").fill(email);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  }

  test("the registrar rewords a text message and sees the preview", async ({ page }, testInfo) => {
    await signIn(page, "registrar@example.invalid");
    await page.getByRole("link", { name: "Messages to applicants" }).click();
    await expect(page.getByRole("heading", { level: 1, name: "Messages to applicants" })).toBeVisible();
    await expectNoSeriousA11yIssues(page);

    const card = page.locator("section").filter({
      has: page.getByRole("heading", { name: "Application received" }),
      hasText: "Text message",
    });
    await expect(card.getByText("Not sent yet: no SMS provider")).toBeVisible();
    const body = card.getByLabel("Text message");

    await body.fill("Ref {reference}, fees {fees}");
    await expect(card.getByText("Remove {fees}: this message cannot fill it in.")).toBeVisible();

    const text = `Dakota Training College (${testInfo.project.name}): we received {reference}. Track it at {link}`;
    await body.fill(text);
    await card.getByText("Preview with sample details").click();
    await expect(card.getByText("we received DTC-APP-2026-000123")).toBeVisible();
    await card.getByRole("button", { name: "Save text message" }).click();
    await expect(page.getByText("Message saved")).toBeVisible();
    await expect(card.getByText(/Last changed by Test Registrar 001/)).toBeVisible();
    expect(await hasHorizontalOverflow(page)).toBe(false);
  });

  test("content editors do not see the message templates", async ({ page }) => {
    await signIn(page, "editor@example.invalid");
    await expect(page.getByRole("link", { name: "Messages to applicants" })).toHaveCount(0);
    await page.goto("/staff/messages");
    await expect(page.getByText("You do not have permission to do this.")).toBeVisible();
  });
});
