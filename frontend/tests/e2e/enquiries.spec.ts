import { expect, test, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Stage 8a: the staff enquiry inbox, against the real API and the dev seed
// (E2E_API=1, E2E_STAFF_PASSWORD).
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

test("a fee enquiry reaches the finance inbox and is handled there", async ({ page, request }, testInfo) => {
  const name = `E2E Parent ${testInfo.project.name} ${Date.now()}`;
  // The public form drops posts made in under 3 seconds (D-70), so say it took longer.
  const sent = await request.post("/api/v1/public/enquiries", {
    data: {
      full_name: name,
      email: `parent-${Date.now()}-${testInfo.project.name}@example.invalid`,
      query_type: "fees",
      message: "How much is the diploma per term?",
      website: "",
      elapsed_ms: 8000,
    },
  });
  expect(sent.status()).toBe(202);

  await signIn(page, "finance@example.invalid");
  await expect(page.getByRole("heading", { level: 2, name: "Enquiries" })).toBeVisible();
  await page.getByRole("link", { name: "Enquiries", exact: true }).first().click();

  // Finance staff open on their own office.
  await expect(page.getByRole("main").getByRole("link", { name: "Finance", exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByRole("link", { name })).toBeVisible();
  // Filters with "Any status" still search (a required empty select once blocked this).
  await page.getByLabel("Search").fill(name);
  await page.getByRole("button", { name: "Show" }).click();
  await expect(page.getByRole("status")).toHaveText("1 enquiry");
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await expectNoSeriousA11yIssues(page);

  await page.getByRole("link", { name }).click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
  await expect(page.getByText("How much is the diploma per term?")).toBeVisible();

  await page.getByRole("button", { name: "Assign to me" }).click();
  await expect(page.getByText("Assigned", { exact: true })).toBeVisible();
  const owner = page.getByRole("heading", { level: 2, name: "Owner" }).locator("..");
  await expect(owner.locator("p", { hasText: "Test Finance Officer 001" })).toBeVisible();

  await page.getByLabel("Add a note").fill("Sent the fee sheet by email.");
  await page.getByRole("button", { name: "Save note" }).click();
  await expect(page.getByText("Sent the fee sheet by email.")).toBeVisible();

  await page.getByRole("button", { name: "Mark waiting for reply" }).click();
  await expect(page.getByText("Waiting for reply").first()).toBeVisible();
  await expect(page.getByText(/added a note/)).toBeVisible();
  await expectNoSeriousA11yIssues(page);
});

test("people without enquiry access do not see the inbox", async ({ page }) => {
  await signIn(page, "editor@example.invalid");
  await expect(page.getByRole("link", { name: "Enquiries", exact: true })).toHaveCount(0);
  await page.goto("/staff/enquiries");
  await expect(page.getByText("You do not have permission to do this.")).toBeVisible();
});
