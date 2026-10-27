import { expect, test, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 11b: student records and academics against the real API and the
// dev seed (sample term, ICT units, four test students, the test trainer on
// ICT 101, the medical Head of School).
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

async function checkPage(page: Page) {
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);
}

test("a trainer sees only their own class (AC-08)", async ({ page }) => {
  await signIn(page, "trainer@example.invalid");
  const nav = page.getByRole("navigation", { name: "Staff" });
  await expect(nav.getByRole("link", { name: "Students" })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Classes" })).toHaveCount(0);
  await page.getByRole("link", { name: "See my classes" }).click();

  await expect(page.getByRole("heading", { level: 1, name: "My teaching" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Monday" })).toBeVisible();
  await expect(page.getByRole("link", { name: /ICT 101/ })).toBeVisible();
  await expect(page.getByRole("link", { name: /ICT 102/ })).toHaveCount(0);
  await checkPage(page);

  await page.getByRole("link", { name: /ICT 101/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: /ICT 101/ })).toBeVisible();
  await expect(page.getByRole("cell", { name: "DTC/SAMPLE/0001" })).toBeVisible();
  // No contact details on a class list.
  await expect(page.getByText("@example.invalid")).toHaveCount(0);
  await checkPage(page);

  // Student records are not theirs to open, even by address.
  await page.goto("/staff/students");
  await expect(page.getByText("Could not load students")).toBeVisible();
});

test("a Head of School sees only their own school", async ({ page }) => {
  await signIn(page, "head.medical@example.invalid");
  await page.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Students" }).click();
  // The sample students study ICT; this head works in Medical.
  await expect(page.getByText("No students here")).toBeVisible();
  await checkPage(page);
});

test("the registrar keeps a student record and runs a class", async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  // Each project edits its own sample student, so parallel runs never collide.
  const number = testInfo.project.name === "mobile" ? "DTC/SAMPLE/0002" : "DTC/SAMPLE/0003";
  await signIn(page, "registrar@example.invalid");
  await page.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Students" }).click();
  await page.getByLabel("Search").fill(number);
  await page.getByRole("button", { name: "Show" }).click();
  await expect(page.getByRole("status")).toHaveText("1 student");
  await checkPage(page);
  await page.getByRole("link", { name: /Test Student/ }).click();

  await expect(page.getByRole("heading", { level: 1, name: /Test Student/ })).toBeVisible();
  await expect(page.getByText(number).first()).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Units registered" }).getByRole("cell", { name: /ICT 101/ }),
  ).toBeVisible();
  await checkPage(page);

  // A contact, with a mistake first.
  await page.getByRole("button", { name: "Add a contact" }).click();
  await page.getByLabel("Full name").fill("Test Parent E2E");
  await page.getByRole("button", { name: "Add contact" }).click();
  await expect(page.getByText("Give a phone number or an email address.")).toBeVisible();
  await page.getByLabel("Phone").fill("+254 700 000 900");
  await page.getByRole("button", { name: "Add contact" }).click();
  await expect(page.getByText("Test Parent E2E").first()).toBeVisible();

  // Defer, then reinstate: both kept in the history with reasons.
  await page.getByLabel("New status").selectOption("deferred");
  await page.getByRole("button", { name: "Change status" }).click();
  await expect(page.getByText("Write why the status is changing.")).toBeVisible();
  await page.getByLabel("Reason").fill("E2E: deferred for one term.");
  await page.getByRole("button", { name: "Change status" }).click();
  await expect(page.getByText("E2E: deferred for one term.").first()).toBeVisible();
  await page.getByLabel("New status").selectOption("active");
  await page.getByLabel("Reason").fill("E2E: back from deferment.");
  await page.getByRole("button", { name: "Change status" }).click();
  await expect(page.getByText("E2E: back from deferment.").first()).toBeVisible();

  // A class: timetable and registration of the cohort.
  await page.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Classes" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Classes" })).toBeVisible();
  await checkPage(page);
  await page.getByRole("link", { name: /ICT 102/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: /ICT 102/ })).toBeVisible();
  const room = `E2E ${testInfo.project.name} ${Date.now() % 100000}`;
  await page.getByLabel("Day").selectOption("5");
  await page.getByLabel("Starts").fill("14:00");
  await page.getByLabel("Ends").fill("13:00");
  await page.getByLabel("Room").fill(room);
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByText("It must end after it starts.")).toBeVisible();
  await page.getByLabel("Ends").fill("15:30");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const entry = page.getByRole("listitem").filter({ hasText: `Friday 14:00 to 15:30, ${room}` });
  await expect(entry).toHaveCount(1);
  await page.getByRole("button", { name: `Remove Friday 14:00 to 15:30, ${room}` }).click();
  await expect(entry).toHaveCount(0);

  await page.getByLabel("Register a cohort").selectOption({ index: 1 });
  await page.getByRole("button", { name: "Register", exact: true }).click();
  await expect(page.getByText(/registered, \d+ already on the list/)).toBeVisible();
  await checkPage(page);

  // The catalogue tabs.
  await page
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "Units and terms" })
    .click();
  await expect(page.getByText("Current term (Sample)").first()).toBeVisible();
  await page.getByRole("tab", { name: "Units", exact: true }).click();
  await expect(page.getByRole("cell", { name: "ICT 103" }).first()).toBeVisible();
  await checkPage(page);
  await page.getByRole("tab", { name: "Cohorts" }).click();
  await expect(page.getByRole("cell", { name: /Example Information Technology/ }).first()).toBeVisible();
});

test("the ICT administrator sees which schools staff work in", async ({ page }) => {
  await signIn(page, "ict.admin@example.invalid");
  await page.getByRole("navigation", { name: "Staff" }).getByRole("link", { name: "Staff schools" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Staff schools" })).toBeVisible();
  const head = page.getByRole("group", { name: /Test Head of School 001/ });
  await expect(head.getByLabel("School of Medical and Health Sciences")).toBeChecked();
  await expect(head.getByLabel("School of ICT")).not.toBeChecked();
  await checkPage(page);
});
