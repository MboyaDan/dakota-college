import { expect, test, type Browser, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 14b: fees from invoice to receipt (AC-10, AC-11) against the real
// API and the dev seed: the test finance officer, the sample fee structure
// for the sample ICT diploma, and the test students' first invoices.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

const learnerNumber = "DTC/SAMPLE/0003";

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

async function student(browser: Browser, number: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/student/login");
  await page.getByLabel("Student number").fill(number);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Hello, Test" })).toBeVisible();
  return page;
}

const nav = (page: Page, name: string) =>
  page.getByRole("navigation", { name: "Staff" }).getByRole("link", { name, exact: true }).click();

/** "Owes KES 13,500.00" → 1350000; "In credit KES 5.00" → -500; "Nothing owed" → 0. */
function cents(text: string): number {
  const digits = Number(text.replace(/[^0-9]/g, "") || "0");
  return text.includes("credit") ? -digits : digits;
}

// Visible only: Next.js keeps the previous visit to a page in the DOM, hidden.
const summaryValue = (page: Page, term: string) =>
  page
    .locator(`xpath=//dt[normalize-space()="${term}"]/following-sibling::dd[1]`)
    .filter({ visible: true })
    .first();

async function openAccount(page: Page) {
  await nav(page, "Finance");
  await page.getByRole("searchbox", { name: "Find a student" }).fill(learnerNumber);
  await page.getByRole("button", { name: "Find", exact: true }).click();
  // A page visited before shows its last data at once, then refreshes; wait for the fresh answer.
  const fresh = page.waitForResponse(
    (r) => r.url().includes("/api/v1/staff/finance/students/") && r.request().method() === "GET",
  );
  // The search result comes before the same student in the arrears list.
  await page
    .getByRole("link", { name: new RegExp(learnerNumber) })
    .first()
    .click();
  await fresh;
  await expect(page.getByRole("heading", { level: 1, name: /^Fees: / })).toBeVisible();
  await expect(summaryValue(page, "Balance")).toBeVisible();
}

/** Downloads a PDF the page links to, with the page's own session. */
async function pdfAt(page: Page, href: string) {
  const res = await page.request.get(href);
  expect(res.status()).toBe(200);
  expect(res.headers()["content-type"]).toBe("application/pdf");
  expect((await res.body()).subarray(0, 4).toString()).toBe("%PDF");
}

test("a payment counts only once finance confirms it, and the student sees it", async ({
  browser,
}, testInfo) => {
  // Every run changes the same sample account, so the flow runs once, at phone width.
  test.skip(testInfo.project.name !== "mobile", "runs once, at phone width");
  test.setTimeout(240_000);
  const stamp = String(Date.now() % 10_000_000).padStart(7, "0");
  const code = `E2E${stamp}`;

  const fin = await staff(browser, "finance@example.invalid");
  await expect(fin.getByRole("heading", { name: "Finance", level: 2 })).toBeVisible();
  await openAccount(fin);

  // The first invoice: issue it if it is still a draft (the seed fills it).
  const draft = fin.getByRole("link", { name: /Draft · First fees, at enrolment/ });
  if ((await draft.count()) > 0) {
    await draft.click();
    await expect(fin.getByRole("heading", { level: 1, name: "Draft invoice" })).toBeVisible();
    await checkPage(fin);
    await fin.getByRole("button", { name: "Issue invoice" }).click();
    await fin.getByRole("dialog").getByRole("button", { name: "Issue", exact: true }).click();
    await expect(fin.getByRole("heading", { level: 1, name: /DTC\/INV\/\d{4}\/\d{5}/ })).toBeVisible();
    await expect(fin.getByRole("link", { name: "Download invoice (PDF)" })).toBeVisible();
    await fin
      .getByRole("link", { name: /^Fees:|Test Student 003/ })
      .first()
      .click();
    await expect(fin.getByRole("heading", { level: 1, name: /^Fees: / })).toBeVisible();
  }
  await checkPage(fin);
  const before = cents(await summaryValue(fin, "Balance").innerText());

  // AC-11: an M-Pesa payment with its code waits and does not count.
  await fin.getByRole("combobox", { name: "How was it paid?", exact: true }).selectOption("mpesa");
  await fin.getByRole("textbox", { name: "Amount (KES)", exact: true }).first().fill("1,234.50");
  await fin.getByRole("textbox", { name: "M-Pesa transaction code" }).fill(code.toLowerCase());
  await fin.getByRole("button", { name: "Record payment" }).click();
  await expect(fin.getByText("Payment recorded. It counts once it is confirmed.")).toBeVisible();
  await expect(fin.getByText(code).first()).toBeVisible();
  expect(cents(await summaryValue(fin, "Balance").innerText())).toBe(before);
  expect(cents(await summaryValue(fin, "Waiting for confirmation").innerText())).toBeGreaterThanOrEqual(
    123450,
  );

  // The student sees it as being confirmed; their balance has not moved.
  const learner = await student(browser, learnerNumber);
  await learner.getByRole("navigation").getByRole("link", { name: "Fees", exact: true }).click();
  await expect(learner.getByRole("heading", { level: 1, name: "Fees" })).toBeVisible();
  const pendingItem = learner.getByRole("listitem").filter({ hasText: code });
  await expect(pendingItem.getByText("Being confirmed")).toBeVisible();
  await checkPage(learner);

  // Finance finds it in the queue and confirms it against the statement.
  await nav(fin, "Payments");
  await fin.getByRole("searchbox", { name: "Search" }).fill(code);
  await fin.getByRole("button", { name: "Search", exact: true }).click();
  await expect(fin.getByRole("status").filter({ hasText: "1 payment" })).toBeVisible();
  await checkPage(fin);
  await fin.getByRole("button", { name: "Confirm", exact: true }).click();
  await fin.getByRole("dialog").getByRole("button", { name: "Confirm and issue receipt" }).click();
  await expect(fin.getByText("Payment confirmed and receipted")).toBeVisible();
  const receipt = fin.getByRole("link", { name: /Receipt DTC\/RCT\/\d{4}\/\d{5}/ });
  await expect(receipt).toBeVisible();
  await pdfAt(fin, (await receipt.getAttribute("href"))!);

  // Now it counts: the balance falls by exactly the amount.
  await openAccount(fin);
  expect(cents(await summaryValue(fin, "Balance").innerText())).toBe(before - 123450);
  await pdfAt(fin, (await fin.getByRole("link", { name: "Download statement (PDF)" }).getAttribute("href"))!);

  // The student has the receipt and the same balance.
  await learner.reload();
  const paidItem = learner.getByRole("listitem").filter({ hasText: code });
  await expect(paidItem.getByText("Confirmed")).toBeVisible();
  await pdfAt(
    learner,
    (await paidItem.getByRole("link", { name: /Download receipt/ }).getAttribute("href"))!,
  );
  await pdfAt(learner, "/api/v1/student/fees/statement");
  await learner.getByRole("navigation").getByRole("link", { name: "Home", exact: true }).click();
  await expect(learner.getByRole("link", { name: "Fees and statement" })).toBeVisible();

  // A bank payment that is not on the statement is rejected with a reason.
  const slip = `SLIP ${stamp}`;
  await fin.getByRole("combobox", { name: "How was it paid?", exact: true }).selectOption("bank");
  await fin.getByRole("textbox", { name: "Amount (KES)", exact: true }).first().fill("500");
  await fin.getByRole("textbox", { name: "Bank slip or transfer reference" }).fill(slip);
  await fin.getByRole("button", { name: "Record payment" }).click();
  await expect(fin.getByText("Payment recorded. It counts once it is confirmed.")).toBeVisible();
  const bankItem = fin.getByRole("listitem").filter({ hasText: slip });
  await bankItem.getByRole("button", { name: /^Reject/ }).click();
  await bankItem.getByLabel("Why is it rejected?").fill("Not on the bank statement for that date.");
  await bankItem.getByRole("button", { name: "Reject payment" }).click();
  await expect(bankItem.getByText("Rejected")).toBeVisible();
  expect(cents(await summaryValue(fin, "Balance").innerText())).toBe(before - 123450);

  // A draft "other" invoice: lines, plan and a running total; then voided.
  await fin.getByRole("combobox", { name: "Invoice for", exact: true }).selectOption("other");
  await fin.getByRole("button", { name: "Open a draft invoice" }).click();
  await expect(fin.getByRole("heading", { level: 1, name: "Draft invoice" })).toBeVisible();
  await fin.getByRole("button", { name: "Add a fee" }).click();
  await fin.getByRole("textbox", { name: "Fee", exact: true }).fill("(Sample) Replacement ID card");
  await fin.getByRole("combobox", { name: "Type", exact: true }).selectOption("other");
  await fin.getByRole("textbox", { name: "Amount (KES)", exact: true }).fill("12.5x");
  await fin.getByRole("button", { name: "Save draft" }).click();
  await expect(fin.getByText(/Enter an amount above zero/)).toBeVisible();
  await fin.getByRole("textbox", { name: "Amount (KES)", exact: true }).fill("250");
  await fin.getByRole("button", { name: "Add an instalment" }).click();
  await fin.getByRole("button", { name: "Add an instalment" }).click();
  await fin.getByRole("textbox", { name: "Share (%)" }).nth(0).fill("60");
  await fin.getByRole("textbox", { name: "Share (%)" }).nth(1).fill("30");
  await fin.getByRole("textbox", { name: "Due after (days)" }).nth(1).fill("30");
  await expect(fin.getByText("The shares add up to 90. They must add up to 100.")).toBeVisible();
  await fin.getByRole("textbox", { name: "Share (%)" }).nth(1).fill("40");
  await expect(fin.getByText("The shares add up to 100.", { exact: true })).toBeVisible();
  await expect(fin.getByText("Total KES 250.00")).toBeVisible();
  await fin.getByRole("button", { name: "Save draft" }).click();
  await expect(fin.getByText("Draft saved")).toBeVisible();
  await checkPage(fin);
  await fin.getByRole("button", { name: "Void this draft" }).click();
  await fin.getByRole("textbox", { name: "Why is it not needed?" }).fill("Raised by the end-to-end test.");
  await fin.getByRole("button", { name: "Void draft" }).click();
  await expect(fin.getByText("This draft was set aside: Raised by the end-to-end test.")).toBeVisible();
});

test("finance screens read well for each role, at every width", async ({ browser }) => {
  test.setTimeout(180_000);
  const fin = await staff(browser, "finance@example.invalid");
  for (const [link, heading] of [
    ["Finance", "Finance"],
    ["Payments", "Payments"],
    ["Invoices", "Invoices"],
    ["Fee structures", "Fee structures"],
  ] as const) {
    await nav(fin, link);
    await expect(fin.getByRole("heading", { level: 1, name: heading })).toBeVisible();
    await expect(fin.getByText(/^Loading/)).toHaveCount(0);
    await checkPage(fin);
  }
  // The sample structure: lines charged once or every term, and a plan.
  await fin
    .getByRole("link", { name: /Diploma in Example Information Technology/ })
    .first()
    .click();
  await expect(fin.getByRole("heading", { level: 1, name: /Information Technology/ })).toBeVisible();
  await expect(fin.getByLabel("Charged").first()).toBeVisible();
  await checkPage(fin);
  await openAccount(fin);
  await checkPage(fin);

  // The principal may look but not record, confirm, adjust or invoice.
  const principal = await staff(browser, "principal@example.invalid");
  await openAccount(principal);
  await expect(principal.getByRole("heading", { name: "Record a payment" })).toHaveCount(0);
  await expect(principal.getByRole("heading", { name: "Adjust the account" })).toHaveCount(0);
  await expect(principal.getByRole("heading", { name: "New invoice" })).toHaveCount(0);
  await nav(principal, "Payments");
  await expect(principal.getByRole("button", { name: "Confirm", exact: true })).toHaveCount(0);

  // Staff without finance access see no finance links.
  const registrar = await staff(browser, "registrar@example.invalid");
  await expect(
    registrar.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Finance" }),
  ).toHaveCount(0);

  // The student's Fees page.
  const learner = await student(browser, "DTC/SAMPLE/0004");
  await learner.goto("/student/fees");
  await expect(learner.getByRole("heading", { level: 1, name: "Fees" })).toBeVisible();
  await expect(learner.getByRole("heading", { name: "Your programme's fees" })).toBeVisible();
  await checkPage(learner);
});
