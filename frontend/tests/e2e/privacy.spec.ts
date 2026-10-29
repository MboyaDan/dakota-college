import { readFile } from "node:fs/promises";
import { expect, test, type Browser } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 17e: data requests and the retention log against the real API and
// the dev seed. The seeded applicant is exported, never erased: other specs
// use it. Each run adds requests to the dev register; they are harmless.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

async function staff(browser: Browser, email: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/staff/login");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  return page;
}

const nav = (page: Awaited<ReturnType<typeof staff>>) => page.getByRole("navigation", { name: "Staff" });

test("an access request: record, check identity, export, complete", async ({ browser }) => {
  const page = await staff(browser, "registrar@example.invalid");
  await nav(page).getByRole("link", { name: "Data requests", exact: true }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Data requests" })).toBeVisible();
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);

  await page.getByRole("button", { name: "Record a request" }).click();
  // Nothing to find the records by: the form says what to add.
  await page.getByLabel("Who is asking").fill("Test Applicant 001");
  await page.getByLabel("How to reach them").fill("applicant001@example.invalid");
  await page.getByRole("button", { name: "Record request" }).click();
  await expect(page.getByText("Give the email address or phone number we hold for the person")).toBeVisible();
  await page.getByRole("textbox", { name: "Email address (optional)" }).fill("applicant001@example.invalid");
  await page.getByRole("button", { name: "Record request" }).click();

  await expect(page.getByRole("heading", { level: 1, name: /^DTC-DSR-\d{4}-\d{4}$/ })).toBeVisible();
  await expect(page.getByText("Due in 7 days")).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Applications" })).toBeVisible();
  await expect(page.getByRole("button", { name: /^Download their data/ })).toHaveCount(0);
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);

  await page.getByLabel("How did you check?").fill("Called back on the number on the application");
  await page.getByRole("button", { name: "Record identity check" }).click();
  const [file] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "Download their data (JSON)" }).click(),
  ]);
  expect(file.suggestedFilename()).toMatch(/^dakota-dtc-dsr-\d{4}-\d{4}-data\.json$/);
  const data = JSON.parse(await readFile((await file.path()) ?? "", "utf8"));
  expect(data.applications).toHaveLength(1);
  expect(JSON.stringify(data)).toContain("applicant001@example.invalid");
  expect(JSON.stringify(data)).not.toContain("password_hash");
  await expect(page.getByText("Data exported")).toBeVisible();

  await page.getByLabel("What was done").fill("Sent the file by hand at the front office.");
  await page.getByRole("button", { name: "Close request" }).click();
  await expect(page.getByText("Completed: what they were told")).toBeVisible();
  await expect(page.getByRole("button", { name: "Close request" })).toHaveCount(0);
});

test("an erasure request with no records is refused with a reason", async ({ browser }) => {
  const page = await staff(browser, "principal@example.invalid");
  await page.goto("/staff/privacy");
  await page.getByRole("button", { name: "Record a request" }).click();
  await page.getByLabel("What do they ask for?").selectOption("erasure");
  await page.getByLabel("Who is asking").fill("Test Person 001");
  await page.getByLabel("How to reach them").fill("+254 700 000 999");
  await page.getByRole("textbox", { name: "Phone number (optional)" }).fill("+254 700 000 999");
  await page.getByRole("button", { name: "Record request" }).click();
  await expect(page.getByText("No records carry these details.")).toBeVisible();
  await page.getByLabel("How did you check?").fill("Saw national ID");
  await page.getByRole("button", { name: "Record identity check" }).click();
  await expect(page.getByRole("heading", { name: "Step 2: erase what can be erased" })).toBeVisible();
  await page.getByLabel("Outcome").selectOption("refused");
  await page.getByLabel("Why it was refused").fill("The college holds no data about you in this system.");
  await page.getByRole("button", { name: "Close request" }).click();
  await expect(page.getByText("Refused: what they were told")).toBeVisible();
});

test("the retention page shows the schedule and counts what is due", async ({ browser }) => {
  const page = await staff(browser, "registrar@example.invalid");
  await nav(page).getByRole("link", { name: "Retention", exact: true }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Retention" })).toBeVisible();
  await expect(page.getByText("Report mode", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Schedule" })).toBeVisible();
  await expect(page.getByText("Kept for:")).toHaveCount(8);
  await page.getByRole("button", { name: "Count what is due" }).click();
  await expect(page.getByRole("region", { name: "Due now" })).toBeVisible();
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);
});

test("staff without privacy.manage see neither page", async ({ browser }) => {
  const page = await staff(browser, "admissions@example.invalid");
  await expect(nav(page).getByRole("link", { name: "Data requests" })).toHaveCount(0);
  await page.goto("/staff/privacy");
  await expect(page.getByText(/permission/i).first()).toBeVisible();
});
