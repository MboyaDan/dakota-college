import { readFile } from "node:fs/promises";
import { expect, test, type Browser, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 16c: the reports page and its CSV downloads against the real API and
// the dev seed. Each role sees only the sections its permissions allow (D-185)
// and an export button only with reports.export (D-183).
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

const sections = ["Admissions", "Students", "Results", "Finance", "Enquiries"];

async function staff(browser: Browser, email: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/staff/login");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  return page;
}

async function openReports(page: Page) {
  await page
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "Reports", exact: true })
    .click();
  await expect(page.getByRole("heading", { level: 1, name: "Reports" })).toBeVisible();
  await expect(page.getByText(/^\d{1,2} \w+ \d{4} to \d{1,2} \w+ \d{4}$/)).toBeVisible();
}

async function shownSections(page: Page) {
  const shown: string[] = [];
  for (const name of sections) {
    if (await page.getByRole("heading", { level: 2, name, exact: true }).isVisible()) shown.push(name);
  }
  return shown;
}

async function download(page: Page, button: string) {
  const [file] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: button }).click(),
  ]);
  const text = await readFile((await file.path()) ?? "", "utf8");
  return { name: file.suggestedFilename(), lines: text.split("\r\n").filter(Boolean) };
}

test("the Principal sees every section and downloads admissions", async ({ browser }) => {
  const page = await staff(browser, "principal@example.invalid");
  await expect(page.getByRole("heading", { level: 2, name: "Reports" })).toBeVisible();
  await openReports(page);
  expect(await shownSections(page)).toEqual(sections);
  for (const b of ["admissions", "enrolments", "pass rates", "fee collection"]) {
    await expect(page.getByRole("button", { name: `Download ${b} (CSV)` })).toBeVisible();
  }
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);

  const csv = await download(page, "Download admissions (CSV)");
  expect(csv.name).toMatch(/^dakota-admissions-\d{4}-\d{2}-\d{2}-to-\d{4}-\d{2}-\d{2}\.csv$/);
  expect(csv.lines[0]).toBe(
    "﻿Programme,Intake,Submitted,In review,Information requested,Offered,Accepted,Enrolled,Rejected,Withdrawn,Expired",
  );
  expect(csv.lines.at(-1)).toMatch(/^Total,,/);
  await expect(page.getByRole("status").getByText(csv.name)).toBeVisible();
});

test("Finance sees no admissions and downloads fee collection", async ({ browser }) => {
  const page = await staff(browser, "finance@example.invalid");
  await openReports(page);
  // Finance holds schools.all (D-133) and enquiries.view (D-94), not admissions.view.
  expect(await shownSections(page)).toEqual(["Students", "Results", "Finance", "Enquiries"]);
  await expect(page.getByRole("button", { name: /^Download/ })).toHaveCount(3);
  await expect(page.getByText(/-KES/)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Download admissions (CSV)" })).toHaveCount(0);

  const csv = await download(page, "Download fee collection (CSV)");
  expect(csv.name).toMatch(/^dakota-fee-collection-/);
  expect(csv.lines[0]).toBe("﻿Month,Billed (KES),Collected (KES),Charges (KES),Credits (KES)");
  expect(csv.lines.at(-1)).toMatch(/^Total,\d+\.\d{2},/);
});

test("a Head of School sees admissions without downloads", async ({ browser }) => {
  const page = await staff(browser, "head.medical@example.invalid");
  await openReports(page);
  expect(await shownSections(page)).toEqual(["Admissions"]);
  await expect(page.getByRole("button", { name: /^Download/ })).toHaveCount(0);
});

test("a role with no report data sees why", async ({ browser }) => {
  const page = await staff(browser, "ict.admin@example.invalid");
  await openReports(page);
  await expect(page.getByText("No reports for your role")).toBeVisible();
  expect(await shownSections(page)).toEqual([]);
  await expectNoSeriousA11yIssues(page);
});

test("a period that ends before it starts says what to fix", async ({ browser }) => {
  const page = await staff(browser, "registrar@example.invalid");
  await page.goto("/staff/reports?from=2026-05-01&to=2026-01-01");
  await expect(page.getByText("The end date must be on or after the start date.")).toBeVisible();
  await expectNoSeriousA11yIssues(page);
});
