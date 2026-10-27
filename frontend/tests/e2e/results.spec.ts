import { expect, test, type Browser, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 13b: results from marks entry to the student (AC-09), against the
// real API and the dev seed: the test trainer teaches ICT 101 (four test
// students, the placeholder CAT 30% / exam 70% scheme), the ICT Head of
// School approves, the registrar releases and issues transcripts.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

const students = ["DTC/SAMPLE/0001", "DTC/SAMPLE/0002", "DTC/SAMPLE/0003", "DTC/SAMPLE/0004"];

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

async function openSheet(page: Page, queue: string) {
  await nav(page, queue);
  await page.getByRole("link", { name: /ICT 101/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: /Results: ICT 101/ })).toBeVisible();
}

async function trainerSheet(page: Page) {
  await nav(page, "My teaching");
  await page.getByRole("link", { name: /ICT 101/ }).click();
  await page.getByRole("link", { name: "Enter results" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /Results: ICT 101/ })).toBeVisible();
}

/** The ICT 101 mark the student sees now, or null before any release. */
async function releasedMark(page: Page): Promise<string | null> {
  await page.goto("/student/results");
  await expect(page.getByRole("heading", { level: 1, name: "Results" })).toBeVisible();
  await expect(page.getByText("Loading your results")).toHaveCount(0);
  const card = page.getByRole("listitem").filter({ hasText: "ICT 101" });
  if ((await card.count()) === 0) return null;
  return (await card.locator("dd").first().innerText()).trim();
}

test("results go from the trainer to the student only once released", async ({ browser }, testInfo) => {
  // One sheet per class, so the flow runs once, at phone width.
  test.skip(testInfo.project.name !== "mobile", "runs once, at phone width");
  test.setTimeout(180_000);
  const stamp = Date.now() % 100000;

  const learner = await student(browser, students[0]);
  const before = await releasedMark(learner);
  await checkPage(learner);

  // A CAT mark whose total differs from what the student sees now.
  const total = (cat: number) => Math.floor((cat * 30 + 50 * 70 + 50) / 100);
  let cat = 60 + (stamp % 40);
  if (String(total(cat)) === before) cat = cat > 80 ? cat - 10 : cat + 10;
  const expected = String(total(cat));

  const trainer = await staff(browser, "trainer@example.invalid");
  const head = await staff(browser, "head.ict@example.invalid");
  const registrar = await staff(browser, "registrar@example.invalid");

  // An earlier run that stopped half way may have left the sheet waiting.
  await trainerSheet(trainer);
  if (await trainer.getByText("Waiting for the Head of School").isVisible()) {
    await openSheet(head, "Results to approve");
    await head.getByLabel("Or return them to the trainer").fill("E2E: reset after an earlier run.");
    await head.getByRole("button", { name: "Return with this reason" }).click();
    await expect(head.getByText("Returned to the trainer").first()).toBeVisible();
    await trainer.reload();
  }
  const correcting = await trainer.getByRole("button", { name: "Correct marks" }).isVisible();
  if (correcting) await trainer.getByRole("button", { name: "Correct marks" }).click();
  await checkPage(trainer);

  // Marks entry: a mistake first, then every mark.
  const field = (number: string, part: string) =>
    trainer.getByRole("group", { name: new RegExp(number) }).getByLabel(part);
  await field(students[0], "Continuous assessment (30%)").fill("101");
  await trainer.getByRole("button", { name: correcting ? "Save the correction" : "Save marks" }).click();
  await expect(trainer.getByText("Enter a whole number from 0 to 100.")).toBeVisible();
  for (const [i, n] of students.entries()) {
    await field(n, "Continuous assessment (30%)").fill(String(i === 0 ? cat : 55));
    await field(n, "Final exam (70%)").fill("50");
  }
  // Students other specs enrolled into this class need marks too before it can be sent.
  for (const box of await trainer.getByRole("group").getByRole("textbox").all()) {
    if ((await box.inputValue()) === "") await box.fill("50");
  }
  if (correcting)
    await trainer.getByLabel("Why are you correcting these results?").fill(`E2E ${stamp}: re-marked.`);
  await trainer.getByRole("button", { name: correcting ? "Save the correction" : "Save marks" }).click();
  await expect(trainer.getByText(/^(Marks saved|Correction saved)/)).toBeVisible();
  await expect(
    trainer.getByRole("group", { name: new RegExp(students[0]) }).getByText(`Total ${expected}, grade`),
  ).toBeVisible();
  await checkPage(trainer);
  await trainer.getByRole("button", { name: "Send for approval" }).click();
  await expect(trainer.getByText("Sent to the Head of School")).toBeVisible();
  await expect(trainer.getByText("Waiting for the Head of School")).toBeVisible();

  // Waiting for approval: the student still sees what they saw.
  expect(await releasedMark(learner)).toBe(before);

  // The Head of School returns them once, with a reason.
  await nav(head, "Overview");
  await expect(head.getByRole("heading", { level: 2, name: "Results to approve" })).toBeVisible();
  await head.getByRole("link", { name: "Review results" }).click();
  await head.getByRole("link", { name: /ICT 101/ }).click();
  await expect(head.getByRole("heading", { level: 1, name: /Results: ICT 101/ })).toBeVisible();
  await expect(head.getByRole("row").filter({ hasText: students[0] })).toContainText(expected);
  await checkPage(head);
  await head.getByRole("button", { name: "Return with this reason" }).click();
  await expect(head.getByText("Say what the trainer needs to change.")).toBeVisible();
  await head.getByLabel("Or return them to the trainer").fill(`E2E ${stamp}: check the exam marks.`);
  await head.getByRole("button", { name: "Return with this reason" }).click();
  await expect(head.getByText("Returned to the trainer").first()).toBeVisible();

  // The trainer sees why, and sends them again unchanged.
  await trainer.reload();
  await expect(trainer.getByText(`E2E ${stamp}: check the exam marks.`)).toBeVisible();
  await trainer.getByRole("button", { name: "Send for approval" }).click();
  await expect(trainer.getByText("Sent to the Head of School")).toBeVisible();

  await head.reload();
  await head.getByRole("button", { name: "Approve results" }).click();
  await expect(head.getByText("Results approved")).toBeVisible();
  await expect(head.getByText("You have nothing to decide on these results.")).toBeVisible();

  // Approved but not released: still nothing new for the student.
  expect(await releasedMark(learner)).toBe(before);

  // The registrar releases them.
  await openSheet(registrar, "Results to release");
  await checkPage(registrar);
  await registrar
    .getByRole("button", { name: before === null ? "Release results" : "Release the correction" })
    .click();
  await registrar.getByRole("dialog").getByRole("button", { name: "Release", exact: true }).click();
  await expect(registrar.getByText("Results released to students")).toBeVisible();
  await expect(registrar.getByText("Released", { exact: true }).first()).toBeVisible();

  // Now the student sees the new mark, and can download the slip.
  expect(await releasedMark(learner)).toBe(expected);
  await checkPage(learner);
  const slip = await learner.request.get("/api/v1/student/results/slip");
  expect(slip.status()).toBe(200);
  expect(slip.headers()["content-type"]).toContain("application/pdf");

  // A transcript request, answered by issuing the official transcript.
  await learner
    .getByRole("link", { name: "send a request for a transcript" })
    .or(learner.getByRole("navigation", { name: "Student portal" }).getByRole("link", { name: "Requests" }))
    .first()
    .click();
  await learner.getByLabel("What do you need?").selectOption("transcript");
  await learner.getByLabel("Details").fill(`E2E ${stamp}: transcript for an employer.`);
  await learner.getByRole("button", { name: "Send request" }).click();
  await expect(learner.getByText(`E2E ${stamp}: transcript for an employer.`)).toBeVisible();

  await nav(registrar, "Student requests");
  const card = registrar.getByRole("article").filter({ hasText: `E2E ${stamp}` });
  await card.getByRole("button", { name: "Issue the transcript for Test Student 001" }).click();
  await expect(registrar.getByText("Transcript issued and request done")).toBeVisible();
  await checkPage(registrar);

  await learner.goto("/student/results");
  const transcript = learner
    .getByRole("listitem")
    .filter({ hasText: /DTC\/TR\/\d{4}\/\d{4}/ })
    .first();
  await expect(transcript).toBeVisible();
  // The new tab follows a five-minute link to the private file (a
  // download in headless Chromium), which serves the PDF.
  const file = learner.context().waitForEvent("request", (r) => r.url().includes("/api/v1/files/"));
  await transcript.getByRole("button", { name: /^Download/ }).click();
  const pdf = await learner.request.get((await file).url());
  expect(pdf.status()).toBe(200);
  expect(pdf.headers()["content-type"]).toContain("application/pdf");
  await checkPage(learner);
});

test("each role finds their results screens", async ({ browser }, testInfo) => {
  test.setTimeout(90_000);
  // Read only, so it can run beside the flow above.
  const trainer = await staff(browser, "trainer@example.invalid");
  await trainerSheet(trainer);
  await expect(trainer.getByText("Continuous assessment 30%, Final exam 70%. Pass mark 40.")).toBeVisible();
  await checkPage(trainer);

  // The medical Head of School has no ICT results to approve.
  const medical = await staff(browser, "head.medical@example.invalid");
  await nav(medical, "Results to approve");
  await medical.getByLabel("Show").selectOption("released");
  await expect(medical.getByText("Nothing here")).toBeVisible();
  await expect(medical.getByRole("link", { name: /ICT 101/ })).toHaveCount(0);
  await checkPage(medical);

  // The registrar's queue and a student's record.
  const registrar = await staff(browser, "registrar@example.invalid");
  await expect(registrar.getByRole("heading", { level: 2, name: "Results to release" })).toBeVisible();
  await nav(registrar, "Results to release");
  await expect(registrar.getByRole("heading", { level: 1, name: "Results to release" })).toBeVisible();
  await checkPage(registrar);
  await nav(registrar, "Students");
  await registrar.getByLabel("Search").fill(testInfo.project.name === "mobile" ? students[1] : students[2]);
  await registrar.getByRole("button", { name: "Show" }).click();
  await expect(registrar.getByRole("status")).toHaveText("1 student");
  await registrar.getByRole("link", { name: /Test Student/ }).click();
  await expect(registrar.getByRole("heading", { level: 2, name: "Results" })).toBeVisible();
  await expect(registrar.getByRole("heading", { level: 3, name: "Official transcripts" })).toBeVisible();
  await checkPage(registrar);

  // A trainer has none of these screens.
  await expect(
    trainer.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: /Results to/ }),
  ).toHaveCount(0);

  // The student's page, whatever has been released so far.
  const learner = await student(browser, testInfo.project.name === "mobile" ? students[1] : students[2]);
  await learner
    .getByRole("navigation", { name: "Student portal" })
    .getByRole("link", { name: "Results" })
    .click();
  await expect(learner.getByRole("heading", { level: 1, name: "Results" })).toBeVisible();
  await expect(learner.getByRole("heading", { level: 2, name: "Official transcripts" })).toBeVisible();
  await checkPage(learner);
});

test("the registrar sets how a programme is assessed", async ({ page }) => {
  await page.goto("/staff/login");
  await page.getByLabel("Email address").fill("registrar@example.invalid");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  const panel = page.getByRole("tabpanel", { name: "Assessment" });
  await nav(page, "Units and terms");
  await page.getByRole("tab", { name: "Assessment" }).click();
  await panel.getByLabel("Programme").selectOption({ label: "Certificate in Example Care Skills (Sample)" });
  await expect(page.getByLabel("How students are assessed")).toHaveValue(/marks|competency/);
  await page.getByLabel("How students are assessed").selectOption("marks");

  await page.getByLabel("Part").first().fill("Practical work");
  await page.getByLabel("Weight (%)").first().fill("40");
  await page.getByLabel("Part").nth(1).fill("Theory");
  await page.getByLabel("Weight (%)").nth(1).fill("50");
  await expect(page.getByText("The weights add up to 90. They must add up to 100.")).toBeVisible();
  await page.getByRole("button", { name: "Save assessment" }).click();
  await expect(page.getByText(/The weights add up to 90\. They must add up to 100\./).nth(1)).toBeVisible();
  await page.getByLabel("Weight (%)").nth(1).fill("60");
  await page.getByRole("button", { name: "Save assessment" }).click();
  await expect(page.getByText("Assessment saved")).toBeVisible();
  await expect(page.getByText("Not set yet")).toHaveCount(0);
  await checkPage(page);

  // Kept when the programme is opened again.
  await page.reload();
  await page.getByRole("tab", { name: "Assessment" }).click();
  await panel.getByLabel("Programme").selectOption({ label: "Certificate in Example Care Skills (Sample)" });
  await expect(page.getByLabel("Part").first()).toHaveValue("Practical work");
  await expect(page.getByLabel("Weight (%)").nth(1)).toHaveValue("60");
});
