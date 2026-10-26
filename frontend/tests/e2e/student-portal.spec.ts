import { expect, test, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 12b: the student portal and the staff who answer it, against the
// real API and the dev seed (test students DTC/SAMPLE/0001 to 0004 sign in
// with the seed password; the sample term is open for registration).
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

async function checkPage(page: Page) {
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);
}

async function staffSignIn(page: Page, email: string) {
  await page.goto("/staff/login");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
}

test("a student uses the portal and staff answer them", async ({ browser }, testInfo) => {
  test.setTimeout(120_000);
  // 0002 and 0003 belong to the Stage 11 tests; these two are the portal's.
  const number = testInfo.project.name === "mobile" ? "DTC/SAMPLE/0001" : "DTC/SAMPLE/0004";
  const name = number.endsWith("1") ? "Test Student 001" : "Test Student 004";
  const stamp = `${testInfo.project.name} ${Date.now() % 100000}`;
  const student = await (await browser.newContext()).newPage();
  const registrar = await (await browser.newContext()).newPage();

  // From the public site to the sign-in page.
  await student.goto("/student");
  await expect(student).toHaveURL(/\/student\/login/);
  await expect(student.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  await checkPage(student);
  await student.getByLabel("Student number").fill(number.toLowerCase());
  await student.getByLabel("Password").fill("not the password");
  await student.getByRole("button", { name: "Sign in" }).click();
  await expect(student.getByText("The student number or password is not correct.")).toBeVisible();
  await student.getByLabel("Password").fill(password);
  await student.getByRole("button", { name: "Sign in" }).click();

  // Home.
  await expect(student.getByRole("heading", { level: 1, name: "Hello, Test" })).toBeVisible();
  await expect(student.getByText(number).first()).toBeVisible();
  await expect(student.getByRole("heading", { level: 2, name: "Next classes" })).toBeVisible();
  await checkPage(student);

  // A notice to everyone reaches them; withdrawn, it is gone.
  await staffSignIn(registrar, "registrar@example.invalid");
  await registrar.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Notices" }).click();
  await registrar.getByLabel("Who is it for").selectOption("all");
  await registrar.getByLabel("Title").fill(`E2E notice ${stamp}`);
  await registrar.getByLabel("Notice", { exact: true }).fill("Library closes early on Friday.");
  await registrar.getByRole("button", { name: "Post notice" }).click();
  await expect(registrar.getByText(`E2E notice ${stamp}`).first()).toBeVisible();
  await checkPage(registrar);
  await student
    .getByRole("navigation", { name: "Student portal" })
    .getByRole("link", { name: "Notices" })
    .click();
  await expect(student.getByRole("heading", { level: 3, name: `E2E notice ${stamp}` })).toBeVisible();
  await checkPage(student);
  await registrar.getByRole("button", { name: `Withdraw E2E notice ${stamp}` }).click();
  await expect(registrar.getByText("Notice withdrawn")).toBeVisible();

  // Timetable and units: drop ICT 102, then register again.
  await student.getByRole("link", { name: "Timetable", exact: true }).click();
  await expect(student.getByRole("heading", { level: 2, name: "Monday" })).toBeVisible();
  await checkPage(student);
  await student.getByRole("link", { name: "Units", exact: true }).click();
  await expect(student.getByRole("heading", { level: 2, name: "Open for registration" })).toBeVisible();
  await student.getByRole("button", { name: "Drop ICT 102" }).click();
  await expect(student.getByRole("button", { name: "Register for ICT 102" })).toBeVisible();
  await checkPage(student);
  await student.getByRole("button", { name: "Register for ICT 102" }).click();
  await expect(student.getByRole("button", { name: "Drop ICT 102" })).toBeVisible();

  // A request, answered by the registrar.
  await student.getByRole("link", { name: "Requests", exact: true }).click();
  await student.getByRole("button", { name: "Send request" }).click();
  await expect(student.getByText("Say what you need, and what it is for.")).toBeVisible();
  await student.getByLabel("Details").fill(`E2E ${stamp}: a letter for my bank.`);
  await student.getByRole("button", { name: "Send request" }).click();
  await expect(student.getByText(`E2E ${stamp}: a letter for my bank.`)).toBeVisible();
  await checkPage(student);

  await registrar
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "Student requests" })
    .click();
  const card = registrar.getByRole("article").filter({ hasText: `E2E ${stamp}` });
  await expect(card).toBeVisible();
  await checkPage(registrar);
  await card.getByLabel("Move to").selectOption("done");
  await card.getByLabel("Reply to the student").fill("Collect it from the office.");
  await card.getByRole("button", { name: `Save request from ${name}` }).click();
  await expect(registrar.getByText("Request updated")).toBeVisible();
  await student.reload();
  const mine = student.getByRole("listitem").filter({ hasText: `E2E ${stamp}` });
  await expect(mine.getByText("Collect it from the office.")).toBeVisible();
  await expect(mine.getByText("Done")).toBeVisible();

  // Profile: phone at once; a correction waits for the registrar.
  await student.getByRole("link", { name: "Profile", exact: true }).click();
  await student.getByLabel("Phone number").fill(`+254 7${String(Date.now()).slice(-8)}`);
  await student.getByRole("button", { name: "Save phone" }).click();
  await expect(student.getByText("Phone saved")).toBeVisible();
  await checkPage(student);
  // A run that failed half-way may have left a correction waiting: clear it.
  if (await student.getByText("Waiting for the registrar").isVisible()) {
    await registrar.reload();
    await registrar.getByRole("tab", { name: "Changes of details" }).click();
    const left = registrar.getByRole("article").filter({ hasText: name }).filter({ hasText: "Waiting" });
    await left.getByLabel("Note to the student").fill("Cleared by the end-to-end test.");
    await left.getByRole("button", { name: `Decline change for ${name}` }).click();
    await expect(registrar.getByText("Change declined")).toBeVisible();
    await student.reload();
  }
  await student.getByRole("button", { name: "Ask for a correction" }).click();
  await student.getByLabel("Middle name").fill("Etoo");
  await student.getByLabel("Why").fill(`E2E ${stamp}: my middle name is missing.`);
  await student.getByRole("button", { name: "Send to the registrar" }).click();
  await expect(student.getByText("Waiting for the registrar")).toBeVisible();

  // Declined (so the next run can ask again), with a reason the student sees.
  await registrar.reload();
  await registrar.getByRole("tab", { name: "Changes of details" }).click();
  const change = registrar.getByRole("article").filter({ hasText: `E2E ${stamp}` });
  await change.getByLabel("Note to the student").fill("Please bring your ID card to the office.");
  await change.getByRole("button", { name: `Decline change for ${name}` }).click();
  await expect(registrar.getByText("Change declined")).toBeVisible();
  await student.reload();
  await expect(student.getByText(/declined, Please bring your ID card to the office./).first()).toBeVisible();

  // Students cannot use the staff portal.
  await student.goto("/staff");
  await expect(student).toHaveURL(/\/staff\/login/);
});

test("a trainer posts a notice to their class", async ({ page }, testInfo) => {
  const title = `E2E class notice ${testInfo.project.name} ${Date.now() % 100000}`;
  await staffSignIn(page, "trainer@example.invalid");
  await page.goto("/staff/teaching");
  await page.getByRole("link", { name: /ICT 101/ }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Notices to this class" })).toBeVisible();
  await page.getByLabel("Title").fill(title);
  await page.getByLabel("Notice", { exact: true }).fill("Bring your laptop on Monday.");
  await page.getByRole("button", { name: "Post notice" }).click();
  await expect(page.getByText(title).first()).toBeVisible();
  await checkPage(page);
  await page.getByRole("button", { name: `Withdraw ${title}` }).click();
  await expect(page.getByText("Withdrawn", { exact: true }).first()).toBeVisible();
});

test("the reset page leads students back to their portal", async ({ page }) => {
  await page.goto("/reset-password?portal=student");
  await expect(page.getByRole("link", { name: "ask for a new one" })).toHaveAttribute(
    "href",
    "/student/forgot-password",
  );
  await page.goto("/student/forgot-password");
  await page.getByLabel("Student number").fill("DTC/1999/9999");
  await page.getByRole("button", { name: "Email me a link" }).click();
  await expect(page.getByText("Check your email")).toBeVisible();
  await checkPage(page);
});
