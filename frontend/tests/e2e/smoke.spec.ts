import { expect, test } from "@playwright/test";
import { expectNoSeriousA11yIssues } from "./axe";

test("home page renders", async ({ page }) => {
  const response = await page.goto("/");
  expect(response?.status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Dakota Training College");
});

test("home page has no critical or serious axe violations", async ({ page }) => {
  await page.goto("/");
  await expectNoSeriousA11yIssues(page);
});

// Needs the Go API running (cd backend && make up). Enabled with E2E_API=1.
test("/api/* is proxied to the Go API", async ({ request }) => {
  test.skip(process.env.E2E_API !== "1", "set E2E_API=1 with the backend running");
  const res = await request.get("/api/v1/health");
  expect(res.status()).toBe(200);
  expect(await res.json()).toEqual({ status: "ok" });
  expect(res.headers()["x-request-id"]).toMatch(/^[0-9a-f]{32}$/);
});
