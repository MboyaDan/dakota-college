import { expect, test } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";
import { totp } from "./totp";

// Needs the Go API with seeded test accounts (cd backend && make up && make
// seed). Each Playwright project uses its own seeded account, and seeding
// again turns two-step sign-in off. Enabled with E2E_API=1.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password,
  "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
);

test("two-step sign-in can be turned on, used and turned off", async ({ page }, testInfo) => {
  const email = `twostep.${testInfo.project.name}@example.invalid`;
  const passwordStep = async () => {
    await page.goto("/staff/login");
    await page.getByLabel("Email address").fill(email);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Sign in" }).click();
  };

  await passwordStep();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  // A finance role is asked to turn it on.
  await page.getByRole("link", { name: "Set it up now" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Your account" })).toBeVisible();
  await page.getByRole("button", { name: "Set up two-step sign-in" }).click();
  await page.getByLabel("Your password").fill(password);
  await page.getByRole("button", { name: "Continue" }).click();

  const key = (await page.getByText(/^[A-Z2-7]{4}( [A-Z2-7]{1,4})+$/).textContent()) ?? "";
  expect(key.length).toBeGreaterThan(20);
  await expect(page.getByRole("img", { name: "QR code for your authenticator app" })).toBeVisible();
  await expectNoSeriousA11yIssues(page);
  expect(await hasHorizontalOverflow(page)).toBe(false);
  await page.getByLabel("6-digit code from the app").fill(totp(key));
  await page.getByRole("button", { name: "Turn on" }).click();

  await expect(page.getByText("Save your recovery codes.")).toBeVisible();
  const codes = await page
    .getByRole("listitem")
    .filter({ hasText: /^[a-z2-9]{4}-[a-z2-9]{4}-[a-z2-9]{4}$/ })
    .allTextContents();
  expect(codes).toHaveLength(10);
  await page.getByRole("button", { name: "I have saved my codes" }).click();
  await expect(page.getByText("10 recovery codes left.")).toBeVisible();
  await page.getByRole("button", { name: "Sign out" }).click();

  // The password alone is no longer enough.
  await passwordStep();
  await expect(page.getByLabel("6-digit code")).toBeVisible();
  await expectNoSeriousA11yIssues(page);
  await page.getByLabel("6-digit code").fill("000000");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "Could not sign in" })).toContainText(
    "That code is not right",
  );
  // The next 30-second code: the one used to turn it on cannot be used again.
  await page.getByLabel("6-digit code").fill(totp(key, 30));
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  await expect(page.getByRole("link", { name: "Set it up now" })).toHaveCount(0);

  // A recovery code works too.
  await page.getByRole("button", { name: "Sign out" }).click();
  await passwordStep();
  await page.getByRole("button", { name: "Lost your phone? Use a recovery code" }).click();
  await page.getByLabel("Recovery code").fill(codes[0]!);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();

  // Turned off again with the password, leaving the account as seeded.
  await page.goto("/staff/account");
  await expect(page.getByText("9 recovery codes left.")).toBeVisible();
  await page.getByRole("button", { name: "Turn off" }).click();
  await page.getByLabel("Your password").fill(password);
  await page.getByRole("button", { name: "Turn off two-step sign-in" }).click();
  await expect(page.getByText("Two-step sign-in is off.")).toBeVisible();
});
