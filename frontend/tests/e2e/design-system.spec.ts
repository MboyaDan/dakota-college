import { expect, test } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// /design is only built when ENABLE_DESIGN_PAGE=1 (CI and local e2e builds).

test.describe("design page", () => {
  test("renders every component section and is noindex", async ({ page }) => {
    await page.goto("/design");
    await expect(page.getByRole("heading", { level: 1, name: "Design system" })).toBeVisible();
    for (const name of [
      "Colours and measured contrast",
      "Typography",
      "Logo files (as supplied)",
      "Icons",
      "Buttons",
      "Form fields",
      "Cards and tags",
      "Alerts",
      "Tabs",
      "Accordion",
      "Dialog and toast",
      "Table",
      "Pagination and breadcrumb",
      "Loading and empty states",
    ]) {
      await expect(page.getByRole("heading", { level: 2, name })).toBeVisible();
    }
    await expect(page.locator('meta[name="robots"]')).toHaveAttribute("content", /noindex/);
  });

  test("has no critical or serious axe violations", async ({ page }) => {
    await page.goto("/design");
    await expectNoSeriousA11yIssues(page);
  });

  test("fits the viewport without horizontal scrolling", async ({ page }) => {
    await page.goto("/design");
    expect(await hasHorizontalOverflow(page)).toBe(false);
  });

  test("tabs follow the arrow-key pattern", async ({ page }) => {
    await page.goto("/design");
    const diploma = page.getByRole("tab", { name: "Diploma" });
    await diploma.focus();
    await page.keyboard.press("ArrowRight");
    await expect(page.getByRole("tab", { name: "Certificate" })).toBeFocused();
    await expect(page.getByRole("tab", { name: "Certificate" })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("tabpanel")).toHaveText("Sample certificate panel.");
    await page.keyboard.press("End");
    await expect(page.getByRole("tab", { name: "Short courses" })).toBeFocused();
    await page.keyboard.press("ArrowRight");
    await expect(diploma).toBeFocused();
  });

  test("dialog opens, closes on Escape and returns focus", async ({ page }) => {
    await page.goto("/design");
    const trigger = page.getByRole("button", { name: "Open dialog" });
    await trigger.click();
    const dialog = page.getByRole("dialog", { name: "Withdraw application?" });
    await expect(dialog).toBeVisible();
    await expectNoSeriousA11yIssues(page);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
  });

  test("toasts are announced in live regions", async ({ page }) => {
    await page.goto("/design");
    await page.getByRole("button", { name: "Show error toast" }).click();
    await expect(page.getByRole("alert").filter({ hasText: "Could not save" })).toBeVisible();
    await page.getByRole("button", { name: "Show success toast" }).click();
    await expect(page.getByRole("status").filter({ hasText: "Changes saved" })).toBeVisible();
  });

  test("respects reduced motion", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/design");
    const duration = await page
      .locator(".animate-pulse")
      .first()
      .evaluate((el) => getComputedStyle(el).animationDuration);
    expect(parseFloat(duration)).toBeLessThan(0.001);
  });
});

test.describe("site shell", () => {
  test("never requests Google Fonts", async ({ page }) => {
    const external: string[] = [];
    page.on("request", (r) => {
      if (/fonts\.(googleapis|gstatic)\.com/.test(r.url())) external.push(r.url());
    });
    await page.goto("/design");
    await page.waitForLoadState("networkidle");
    expect(external).toEqual([]);
  });

  test("skip link moves focus to main content", async ({ page }) => {
    await page.goto("/");
    await page.keyboard.press("Tab");
    const skip = page.getByRole("link", { name: "Skip to main content" });
    await expect(skip).toBeFocused();
    await expect(skip).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(page.locator("main#main")).toBeFocused();
  });

  test("touch targets in the header are at least 44 px tall", async ({ page }) => {
    await page.goto("/");
    const targets = page.locator("header a:visible, header button:visible");
    const count = await targets.count();
    expect(count).toBeGreaterThan(0);
    for (let i = 0; i < count; i++) {
      const box = await targets.nth(i).boundingBox();
      expect(box?.height, await targets.nth(i).innerText()).toBeGreaterThanOrEqual(44);
    }
  });

  test("mobile menu opens, shows links, and closes on Escape", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "mobile", "mobile layout only");
    await page.goto("/");
    expect(await hasHorizontalOverflow(page)).toBe(false);
    await expect(page.getByRole("link", { name: "Apply now" })).toBeVisible();

    const toggle = page.getByRole("button", { name: "Menu" });
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await page.getByRole("button", { name: "Programmes" }).click();
    await expect(page.getByRole("link", { name: "Pool Management" })).toBeVisible();
    const menu = page.getByRole("navigation", { name: "Main" });
    await expect(menu.getByRole("link", { name: "Student Portal" })).toBeVisible();
    expect(await hasHorizontalOverflow(page)).toBe(false);
    await expectNoSeriousA11yIssues(page);

    await page.keyboard.press("Escape");
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(toggle).toBeFocused();
  });

  test("desktop mega-menu opens, closes on Escape and outside click", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop", "desktop layout only");
    await page.goto("/");
    await expect(page.getByRole("navigation", { name: "Utility" })).toBeVisible();

    const programmes = page.getByRole("button", { name: "Programmes" });
    await programmes.click();
    await expect(programmes).toHaveAttribute("aria-expanded", "true");
    await expect(page.getByRole("link", { name: "Medical and Health Sciences" })).toBeVisible();
    await expectNoSeriousA11yIssues(page);

    await page.keyboard.press("Escape");
    await expect(programmes).toHaveAttribute("aria-expanded", "false");
    await expect(programmes).toBeFocused();

    await page.getByRole("button", { name: "Admissions" }).click();
    await expect(page.getByRole("link", { name: "Intake calendar" })).toBeVisible();
    // Click outside the menu: the bottom corner of the viewport.
    const { width, height } = page.viewportSize()!;
    await page.mouse.click(width - 5, height - 5);
    await expect(page.getByRole("link", { name: "Intake calendar" })).toBeHidden();
  });
});
