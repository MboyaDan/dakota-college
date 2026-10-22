import { expect, test, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Needs the Go API with seeded test accounts (cd backend && make up && make
// seed). E2E_STAFF_PASSWORD is the seed password. Enabled with E2E_API=1.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

async function signIn(page: Page, email: string) {
  await page.goto("/staff/login");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
}

test("signed-out visitors are sent to the login page and back", async ({ page }, testInfo) => {
  await page.goto("/staff/content/news");
  await expect(page).toHaveURL(/\/staff\/login\?next=%2Fstaff%2Fcontent%2Fnews/);
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute("content", /noindex/);
  await expectNoSeriousA11yIssues(page);
  expect(await hasHorizontalOverflow(page)).toBe(false);

  // A fresh unknown address each run: repeated failures for one address are
  // rate limited (D-45), which is the point.
  await page
    .getByLabel("Email address")
    .fill(`nobody-${Date.now()}-${testInfo.project.name}@example.invalid`);
  await page.getByLabel("Password").fill("not a real password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "Could not sign in" })).toContainText(
    "The email or password is not correct",
  );

  await page.getByLabel("Email address").fill("editor@example.invalid");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/staff\/content\/news$/);
  await expect(page.getByRole("heading", { level: 1, name: "News" })).toBeVisible();
});

test("credentials never reach the URL, even on a fast submit", async ({ page }) => {
  const urls: string[] = [];
  page.on("request", (r) => urls.push(r.url()));
  await page.goto("/staff/login", { waitUntil: "commit" });
  await page.getByLabel("Email address").fill("editor@example.invalid");
  await page.getByLabel("Password").fill("a-secret-that-must-not-leak");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(
    page
      .getByRole("heading", { level: 1, name: /^Hello/ })
      .or(page.getByRole("alert").filter({ hasText: "Could not sign in" })),
  ).toBeVisible();
  expect(urls.filter((u) => u.includes("a-secret") || u.includes("password="))).toEqual([]);
});

test("navigation shows only what the role can do", async ({ page }) => {
  await signIn(page, "editor@example.invalid");
  const nav = page.getByRole("navigation", { name: "Staff" });
  await expect(nav.getByRole("link", { name: "Programmes" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Waiting for approval" })).toHaveCount(0);
  await expectNoSeriousA11yIssues(page);
  expect(await hasHorizontalOverflow(page)).toBe(false);

  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/staff\/login/);
  await page.goto("/staff");
  await expect(page).toHaveURL(/\/staff\/login/);
});

// Stage 5 done-when (with DQ-14): an editor drafts a programme and a
// registrar publishes it, with no developer involved.
test("editor drafts a programme, registrar publishes it", async ({ browser }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "one full run is enough; layout is checked on both");
  const title = `E2E test programme ${Date.now()} (Sample)`;

  const editorContext = await browser.newContext();
  const editor = await editorContext.newPage();
  await signIn(editor, "editor@example.invalid");
  await editor.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Programmes" }).click();
  await editor.getByRole("link", { name: "New programme" }).first().click();
  await expect(editor.getByRole("heading", { level: 1, name: "New programme" })).toBeVisible();

  await editor.getByLabel("School").selectOption({ label: "School of ICT" });
  await editor.getByLabel("Title", { exact: true }).fill(title);
  await editor.getByRole("button", { name: /^Use “/ }).click();
  await editor.getByLabel("Duration in months").fill("12");
  await editor.getByLabel("Summary").first().fill("A short programme used by automated tests.");
  await editor.getByLabel("What you will learn").fill("How the staff portal works.");
  await editor.getByRole("button", { name: "Add requirement" }).click();
  await editor.getByLabel("Requirement", { exact: true }).fill("KCSE mean grade D or equivalent.");
  await editor.getByRole("button", { name: "Add fee line" }).click();
  await editor.getByLabel("Label").fill("Tuition");
  await editor.getByLabel("Amount (KES)").fill("45,000");
  await expect(editor.getByText("KES 45,000.00")).toBeVisible();
  await expectNoSeriousA11yIssues(editor);

  await editor.getByRole("button", { name: "Save draft" }).click();
  await expect(editor).toHaveURL(/\/staff\/content\/programmes\/[0-9a-f-]{36}$/);
  await expect(editor.getByText("Draft", { exact: true }).first()).toBeVisible();
  // Editors cannot publish (DQ-14); the API would refuse anyway (AC-07).
  await expect(editor.getByRole("button", { name: "Publish" })).toHaveCount(0);
  await editor.getByRole("button", { name: "Preview" }).click();
  await expect(editor.getByText("Preview, not live yet")).toBeVisible();
  await expect(editor.getByText("Tuition: KES 45,000.00 per term")).toBeVisible();
  const programmeUrl = editor.url();

  const registrarContext = await browser.newContext();
  const registrar = await registrarContext.newPage();
  await signIn(registrar, "registrar@example.invalid");
  await registrar
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "Waiting for approval" })
    .click();
  await registrar.getByRole("link", { name: new RegExp(title.replace(/[()]/g, "\\$&")) }).click();
  await expectNoSeriousA11yIssues(registrar);
  await registrar.getByRole("button", { name: "Publish", exact: true }).click();
  await expect(registrar.getByText("Published", { exact: true }).first()).toBeVisible();

  // Editing live content waits for approval.
  await editor.goto(programmeUrl);
  await editor.getByLabel("Title", { exact: true }).fill(`${title} edited`);
  await editor.getByRole("button", { name: "Save changes" }).click();
  await expect(editor.getByText("Changes are waiting for approval")).toBeVisible();

  await registrar.reload();
  await registrar.getByRole("button", { name: "Publish changes" }).click();
  await expect(registrar.getByText("Changes are waiting for approval")).toHaveCount(0);
  await expect(registrar.getByLabel("Title", { exact: true })).toHaveValue(`${title} edited`);

  // Leave nothing live: archive the test programme.
  await registrar.getByRole("button", { name: "Archive" }).click();
  await expect(registrar.getByText("Archived", { exact: true }).first()).toBeVisible();

  await editorContext.close();
  await registrarContext.close();
});
