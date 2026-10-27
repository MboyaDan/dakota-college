import { expect, test, type Browser, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 15a: M-Pesa C2B (AC-11 verified flow) against the real API with
// M-Pesa enabled locally (fake keys). Confirmations are posted to the API
// as Safaricom would: on the secret path, with a fresh code each run.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
const secret = process.env.E2E_MPESA_SECRET ?? "";
const apiURL = process.env.E2E_API_URL ?? "http://127.0.0.1:8180";
const shortCode = process.env.E2E_MPESA_SHORTCODE ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password || !secret || !shortCode,
  "set E2E_API=1, E2E_STAFF_PASSWORD, E2E_MPESA_SECRET and E2E_MPESA_SHORTCODE with M-Pesa enabled",
);

const learnerNumber = "DTC/SAMPLE/0004";

async function checkPage(page: Page) {
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);
}

async function staff(browser: Browser, email: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/staff/login");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  return page;
}

async function student(browser: Browser) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/student/login");
  await page.getByLabel("Student number").fill(learnerNumber);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Hello, Test" })).toBeVisible();
  return page;
}

/** A payment to the college paybill, as Safaricom reports it. */
async function confirmation(page: Page, code: string, amount: string, account: string) {
  const now = new Date(Date.now() + 3 * 60 * 60 * 1000).toISOString().replace(/[-:T]/g, "").slice(0, 14);
  const res = await page.request.post(`${apiURL}/api/v1/payments/c2b/${secret}/confirmation`, {
    data: {
      TransactionType: "Pay Bill",
      TransID: code,
      TransTime: now,
      TransAmount: amount,
      BusinessShortCode: shortCode,
      BillRefNumber: account,
      MSISDN: "2547 ***** 000",
      FirstName: "Test",
      LastName: "Payer",
    },
  });
  expect(res.status()).toBe(200);
}

test("a paybill payment is credited by itself, and an unmatched one is assigned", async ({
  browser,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "changes the sample account: runs once, at phone width");
  test.setTimeout(180_000);
  const stamp = String(Date.now() % 10_000_000).padStart(7, "0");
  const matched = `MPA${stamp}`;
  const stray = `MPB${stamp}`;

  const fin = await staff(browser, "finance@example.invalid");
  // Typed in lower case without slashes: still the student's number.
  await confirmation(fin, matched, "250.00", learnerNumber.toLowerCase().replaceAll("/", ""));
  await confirmation(fin, stray, "75", "NOT-A-STUDENT");
  // A wrong secret is a 404 and leaves nothing behind.
  const wrong = await fin.request.post(
    `${apiURL}/api/v1/payments/c2b/not-the-secret-0123456789abcdef/confirmation`,
    {
      data: { TransID: `MPC${stamp}` },
    },
  );
  expect(wrong.status()).toBe(404);

  await fin
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "M-Pesa", exact: true })
    .click();
  await expect(fin.getByRole("heading", { level: 1, name: "M-Pesa" })).toBeVisible();
  await expect(fin.getByRole("listitem").filter({ hasText: matched }).getByText("Credited")).toBeVisible();
  const waiting = fin.getByRole("listitem").filter({ hasText: stray }).first();
  await expect(waiting.getByText(/No student has the account number/)).toBeVisible();
  await checkPage(fin);

  // Finance assigns the stray payment to the student, with a note.
  await waiting.getByRole("button", { name: /^Assign to a student/ }).click();
  await waiting.getByRole("searchbox", { name: "Find the student" }).fill(learnerNumber);
  await waiting.getByRole("button", { name: "Find", exact: true }).click();
  await waiting.getByRole("button", { name: new RegExp(`Choose ${learnerNumber}`) }).click();
  await waiting
    .getByRole("textbox", { name: "How do you know whose payment it is?" })
    .fill("The parent phoned with the M-Pesa code.");
  await waiting.getByRole("button", { name: "Assign and issue receipt" }).click();
  await expect(fin.getByText("Assigned and receipted")).toBeVisible();
  await expect(fin.getByRole("region", { name: /Waiting/ }).getByText(stray)).toHaveCount(0);

  // The student sees both, receipted, and how to pay next time.
  const learner = await student(browser);
  await learner.goto("/student/fees");
  await expect(learner.getByRole("heading", { name: "How to pay" })).toBeVisible();
  await expect(learner.getByText(shortCode, { exact: true })).toBeVisible();
  await expect(learner.getByText(learnerNumber, { exact: true }).first()).toBeVisible();
  for (const code of [matched, stray]) {
    const item = learner.getByRole("listitem").filter({ hasText: code });
    await expect(item.getByText("Confirmed")).toBeVisible();
    await expect(item.getByRole("link", { name: /Download receipt/ })).toBeVisible();
  }
  await checkPage(learner);
});

test("the M-Pesa page reads well at every width", async ({ browser }) => {
  const fin = await staff(browser, "finance@example.invalid");
  await fin.goto("/staff/finance/mpesa");
  await expect(fin.getByRole("heading", { name: "Check a day against the M-Pesa statement" })).toBeVisible();
  await expect(fin.getByText(/^Loading/)).toHaveCount(0);
  await checkPage(fin);
  // Setup is only for those who may register the URLs.
  await expect(fin.getByRole("button", { name: "Register callback URLs" })).toHaveCount(0);
  const admin = await staff(browser, "admin@example.invalid");
  await admin.goto("/staff/finance/mpesa");
  await expect(admin.getByRole("button", { name: "Register callback URLs" })).toBeVisible();
  await expect(admin.getByRole("button", { name: "Send test payment" })).toBeVisible();
  await checkPage(admin);
});
