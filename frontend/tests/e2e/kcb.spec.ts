import { expect, test, type Browser, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 15b: KCB Buni notifications against the real API with KCB enabled
// locally. Notifications are posted as KCB would, on the secret path. Until
// KCB's signature can be checked, a matched one waits in Payments (D-175).
const password = process.env.E2E_STAFF_PASSWORD ?? "";
const secret = process.env.E2E_KCB_SECRET ?? "";
const apiURL = process.env.E2E_API_URL ?? "http://127.0.0.1:8180";
test.skip(
  process.env.E2E_API !== "1" || !password || !secret,
  "set E2E_API=1, E2E_STAFF_PASSWORD and E2E_KCB_SECRET with KCB notifications enabled",
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

async function notify(page: Page, ref: string, amount: string, customerRef: string) {
  const now = new Date(Date.now() + 3 * 60 * 60 * 1000).toISOString().replace(/[-:T]/g, "").slice(0, 14);
  const res = await page.request.post(`${apiURL}/api/v1/payments/kcb/${secret}/notification`, {
    headers: { Signature: "ZTJlLXRlc3Q=" },
    data: {
      transactionReference: ref,
      requestId: crypto.randomUUID(),
      channelCode: "202",
      timestamp: now,
      transactionAmount: amount,
      currency: "KES",
      customerReference: customerRef,
      customerName: "Test Payer",
      customerMobileNumber: "254700000000",
      balance: "",
      narration: "Fees",
      creditAccountIdentifier: "TEST",
      organizationShortCode: "000000",
      tillNumber: "000000",
    },
  });
  expect(res.status()).toBe(200);
  expect((await res.json()).statusCode).toBe("0");
}

test("a KCB payment waits for Finance to confirm it, and a stray one is assigned", async ({
  browser,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "changes the sample account: runs once, at phone width");
  test.setTimeout(180_000);
  const stamp = String(Date.now() % 100_000_000).padStart(8, "0");
  const matched = `FTA${stamp}`;
  const stray = `FTB${stamp}`;

  const fin = await staff(browser, "finance@example.invalid");
  await notify(fin, matched, "300.00", learnerNumber);
  await notify(fin, stray, "45", "INV-0001");

  await fin
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "KCB bank", exact: true })
    .click();
  await expect(fin.getByRole("heading", { level: 1, name: "KCB bank" })).toBeVisible();
  await expect(fin.getByText("Every KCB payment is confirmed by Finance")).toBeVisible();
  await expect(
    fin.getByRole("listitem").filter({ hasText: matched }).getByText("Waiting for confirmation"),
  ).toBeVisible();
  await checkPage(fin);

  // The matched one is confirmed in Payments after checking the statement.
  await fin
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "Payments", exact: true })
    .click();
  await expect(fin.getByRole("heading", { level: 1, name: "Payments" })).toBeVisible();
  // Visible only: the KCB page just left is kept, hidden, with its own search.
  await fin.getByRole("searchbox", { name: "Search" }).filter({ visible: true }).fill(matched);
  await fin.getByRole("button", { name: "Search", exact: true }).filter({ visible: true }).click();
  await expect(fin.getByRole("status").filter({ hasText: "1 payment" })).toBeVisible();
  await expect(fin.getByText("KCB notification")).toBeVisible();
  await fin.getByRole("button", { name: "Confirm", exact: true }).click();
  await fin.getByRole("dialog").getByRole("button", { name: "Confirm and issue receipt" }).click();
  await expect(fin.getByText("Payment confirmed and receipted")).toBeVisible();

  // The stray one is assigned from the KCB page.
  await fin
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "KCB bank", exact: true })
    .click();
  const waiting = fin.getByRole("listitem").filter({ hasText: stray }).first();
  await expect(waiting.getByText(/No student has the reference/)).toBeVisible();
  await waiting.getByRole("button", { name: /^Assign to a student/ }).click();
  await waiting.getByRole("searchbox", { name: "Find the student" }).fill(learnerNumber);
  await waiting.getByRole("button", { name: "Find", exact: true }).click();
  await waiting.getByRole("button", { name: new RegExp(`Choose ${learnerNumber}`) }).click();
  await waiting
    .getByRole("textbox", { name: "How do you know whose payment it is?" })
    .fill("Found on the KCB statement; the sponsor named the student.");
  await waiting.getByRole("button", { name: "Assign and issue receipt" }).click();
  await expect(fin.getByText("Assigned and receipted")).toBeVisible();
  await expect(fin.getByRole("listitem").filter({ hasText: matched }).getByText("Confirmed")).toBeVisible();
});

test("the KCB page reads well at every width", async ({ browser }) => {
  const fin = await staff(browser, "finance@example.invalid");
  await fin.goto("/staff/finance/kcb");
  await expect(fin.getByRole("heading", { name: "Check a day against the KCB statement" })).toBeVisible();
  await expect(fin.getByText(/^Loading/)).toHaveCount(0);
  await checkPage(fin);
});
