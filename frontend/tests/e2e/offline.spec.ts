import { expect, test } from "@playwright/test";

// AC-15: public pages keep serving when the API is unavailable. Run after
// stopping the API (CI does this after the main run): E2E_API_DOWN=1.
test.skip(process.env.E2E_API_DOWN !== "1", "set E2E_API_DOWN=1 with the API stopped");

test("API is really down", async ({ request }) => {
  const res = await request.get("/api/v1/health");
  expect(res.status()).toBeGreaterThanOrEqual(500);
});

for (const path of [
  "/",
  "/schools/ict",
  "/programmes/example-health-studies-diploma",
  "/about",
  "/about/founders",
  "/contact",
  "/admissions/fees",
  "/admissions/intakes",
  "/alumni",
  "/apply/track",
  "/privacy",
  "/news",
  "/news/sample-news-applications",
  "/events",
]) {
  test(`${path} still renders its content`, async ({ page }) => {
    const res = await page.goto(path);
    expect(res?.status()).toBe(200);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("contentinfo")).toBeVisible();
  });
}

test("filtered course finder falls back to the bundled content", async ({ page }) => {
  const res = await page.goto("/programmes?level=diploma");
  expect(res?.status()).toBe(200);
  await expect(page.getByRole("link", { name: "Diploma in Example Health Studies (Sample)" })).toBeVisible();
});

test("FAQ search falls back to the bundled content", async ({ page }) => {
  const res = await page.goto("/faqs?q=instalments");
  expect(res?.status()).toBe(200);
  await expect(page.getByText("(Sample) Can I pay in instalments?")).toBeVisible();
});
