import { expect, test, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";
import { linkFromEmail, mailpit } from "./mailpit";

// Stage 9a: from a programme page to a filled-in application, against the
// real API, signing in through the emailed link (Mailpit).
test.skip(
  process.env.E2E_API !== "1" || !mailpit,
  "set E2E_API=1 and E2E_MAILPIT_URL with the backend running",
);

const programme = "Diploma in Example Information Technology (Sample)";

/** Checks the step on screen, then saves it and moves on. */
async function saveAndContinue(page: Page, nextTitle: string) {
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);
  await page.getByRole("button", { name: "Save and continue" }).click();
  await expect(page.getByRole("heading", { level: 2, name: new RegExp(nextTitle) })).toBeVisible();
}

test("an under-18 applicant starts and fills in an application", async ({ page, request }, testInfo) => {
  const email = `e2e-form-${testInfo.project.name}-${Date.now()}@example.invalid`;

  // Programme page, Apply, Start: asked to sign in, keeping the programme.
  await page.goto("/programmes/example-information-technology-diploma");
  await page.getByRole("link", { name: "Apply for this programme" }).first().click();
  await page.getByRole("link", { name: "Start your application" }).click();
  await expect(
    page.getByRole("heading", { level: 1, name: "Start or continue your application" }),
  ).toBeVisible();
  const since = Date.now() - 2_000;
  await page.getByLabel("Your name").fill("Test Applicant E2E");
  await page.getByLabel("Email address").fill(email);
  await page.getByRole("button", { name: "Email me a link" }).click();
  await expect(page.getByText("Check your email")).toBeVisible();

  // The link brings them back to start the programme they chose.
  await page.goto(await linkFromEmail(request, email, since));
  await page.getByRole("button", { name: "Continue to my application" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Start an application" })).toBeVisible();
  await expect(page.getByText(`You are applying for ${programme}.`)).toBeVisible();
  await expectNoSeriousA11yIssues(page);
  await page.getByRole("button", { name: "Start my application" }).click();

  // About you. The name typed at sign-in is already there.
  await expect(page.getByRole("heading", { level: 2, name: /About you/ })).toBeVisible();
  const reference = (await page.locator(".font-mono").first().textContent()) ?? "";
  expect(reference).toMatch(/^DTC-APP-\d{4}-\d{6}$/);
  await expect(page.getByLabel("First name")).toHaveValue("Test");

  // A wrong value is explained next to its field and at the top.
  await page.getByLabel("Phone number").fill("call me");
  await page.getByRole("button", { name: "Save and continue" }).click();
  await expect(page.getByRole("link", { name: /Phone number: Enter a phone number/ })).toBeVisible();
  await expect(page.getByLabel("Phone number")).toHaveAttribute("aria-invalid", "true");

  const sixteen = new Date();
  sixteen.setFullYear(sixteen.getFullYear() - 16);
  await page.getByLabel("Last name").fill("Applicant E2E");
  await page.getByLabel("Date of birth").fill(sixteen.toISOString().slice(0, 10));
  await page.getByLabel("Identity document").selectOption("birth_certificate");
  await page.getByLabel("Document number").fill("E2E 0001");
  await page.getByLabel("Phone number").fill("+254 700 000 001");
  await page.getByLabel("Nationality").fill("Kenyan");
  await page.getByLabel("County where you live").selectOption("Nairobi");

  // Autosave: wait for it, reload, and the answers are still there.
  await expect(page.getByRole("status").filter({ hasText: "All changes saved" })).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Nationality")).toHaveValue("Kenyan");
  await expect(page.getByLabel("Document number")).toHaveValue("E2E 0001");

  // Under 18, so the guardian step comes next.
  await saveAndContinue(page, "Parent or guardian");
  await page.getByLabel("Their full name").fill("Test Guardian E2E");
  await page.getByLabel("How are they related to you?").selectOption("mother");
  await page.getByLabel("Their phone number").fill("+254 700 000 002");
  await page.getByLabel(/has read the privacy notice and agrees/).check();
  await saveAndContinue(page, "Education");

  await page.getByLabel("Name of the school or college").fill("Test Secondary School");
  await page.getByLabel("Year you finished").fill(String(new Date().getFullYear() - 1));
  await page.getByLabel("KCSE mean grade").selectOption("C+");
  await page.getByRole("button", { name: "Add a subject" }).click();
  await page.getByLabel("Subject 1").fill("English");
  await page.getByLabel(/^Grade/).selectOption("B-");
  await saveAndContinue(page, "Fees and sponsor");

  await page.getByLabel("My parent or guardian").check();
  await page.getByLabel("Their name").fill("Test Guardian E2E");
  await page.getByLabel("Their phone number").fill("+254 700 000 002");
  await saveAndContinue(page, "Check your answers");

  await expect(page.getByText("All the answers we need are here")).toBeVisible();
  await expect(page.getByText("Test Secondary School, 20")).toBeVisible();
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);

  // The account page lists it with its reference, to continue later.
  await page.getByRole("link", { name: "Back to your applications" }).click();
  const card = page.getByRole("listitem").filter({ hasText: reference });
  await expect(card.getByText("Not submitted")).toBeVisible();
  await card.getByRole("link", { name: /Continue your application/ }).click();
  await expect(page.getByRole("heading", { level: 2, name: /Programme and intake/ })).toBeVisible();
  await expect(page.getByRole("radio", { checked: true })).toHaveCount(1);
});
