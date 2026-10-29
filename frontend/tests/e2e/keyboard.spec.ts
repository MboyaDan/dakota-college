import { expect, test, type Page } from "@playwright/test";

// Stage 18: keyboard use of the real public pages (the design page has its
// own checks). The skip link comes first, every stop shows a focus ring and
// is on screen, menus open with Enter and close with Escape, and forms can be
// sent and fixed without a mouse.

const pages = [
  "/",
  "/programmes",
  "/admissions",
  "/admissions/fees",
  "/faqs",
  "/contact",
  "/apply",
  "/about",
];

test("the skip link is the first stop and moves focus to the content", async ({ page }) => {
  for (const path of pages) {
    await page.goto(path);
    await page.keyboard.press("Tab");
    const skip = page.getByRole("link", { name: "Skip to main content" });
    await expect(skip, path).toBeFocused();
    await expect(skip, path).toBeInViewport();
    await page.keyboard.press("Enter");
    await expect(page.locator("main#main"), path).toBeFocused();
  }
});

type Stop = { label: string; visible: boolean; ring: boolean };

/** Tabs through the page and describes each stop. */
async function tabStops(page: Page, max: number): Promise<Stop[]> {
  const stops: Stop[] = [];
  // The page has finished loading fonts and streamed content first.
  await page.waitForLoadState("networkidle");
  for (let i = 0; i < max; i++) {
    await page.keyboard.press("Tab");
    let stop = await describeFocus(page);
    // A late layout change can leave the stop briefly out of view; measure
    // again before calling it off screen.
    if (stop && !stop.visible) {
      await page.waitForTimeout(150);
      stop = await describeFocus(page);
    }
    if (!stop) break;
    stops.push(stop);
  }
  return stops;
}

function describeFocus(page: Page): Promise<Stop | null> {
  return page.evaluate(async () => {
    // Let the browser finish scrolling the new stop into view.
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    const el = document.activeElement as HTMLElement | null;
    if (!el || el === document.body) return null;
    const r = el.getBoundingClientRect();
    const cs = getComputedStyle(el);
    const outline = cs.outlineStyle !== "none" && parseFloat(cs.outlineWidth) >= 2;
    return {
      label: `${el.tagName.toLowerCase()} "${(el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40)}"`,
      visible:
        r.width > 0 && r.height > 0 && r.bottom > 0 && r.top < innerHeight && cs.visibility !== "hidden",
      ring: outline || cs.boxShadow !== "none",
    };
  });
}

test("every keyboard stop is on screen and shows a focus ring", async ({ page }) => {
  for (const path of ["/", "/programmes", "/contact", "/faqs"]) {
    await page.goto(path);
    const stops = await tabStops(page, 80);
    expect(stops.length, path).toBeGreaterThan(5);
    const hidden = stops.filter((s) => !s.visible).map((s) => s.label);
    const noRing = stops.filter((s) => !s.ring).map((s) => s.label);
    expect(hidden, `${path}: focus on something off screen`).toEqual([]);
    expect(noRing, `${path}: no visible focus`).toEqual([]);
  }
});

test("menus open with Enter and close with Escape, returning focus", async ({ page }, info) => {
  await page.goto("/");
  const toggle =
    info.project.name === "mobile"
      ? page.getByRole("button", { name: "Menu" })
      : page.getByRole("navigation", { name: "Main" }).getByRole("button", { name: "Programmes" });
  await toggle.focus();
  await page.keyboard.press("Enter");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(toggle).toBeFocused();
});

test("an FAQ opens and closes from the keyboard", async ({ page }, info) => {
  // Chromium's phone emulation leaves <summary> out of the tab order (even
  // with tabindex); desktop keyboards are tested here, phones in the manual
  // checklist (docs/accessibility.md).
  test.skip(info.project.name === "mobile", "summary is not tabbable under phone emulation");
  await page.goto("/faqs");
  await page.waitForLoadState("networkidle");
  const summary = page.locator("details summary").first();
  test.skip((await summary.count()) === 0, "no FAQs published");
  const details = page.locator("details").first();
  await summary.focus();
  const before = await details.evaluate((d: HTMLDetailsElement) => d.open);
  await page.keyboard.press("Enter");
  await expect.poll(() => details.evaluate((d: HTMLDetailsElement) => d.open)).toBe(!before);
  await page.keyboard.press("Space");
  await expect.poll(() => details.evaluate((d: HTMLDetailsElement) => d.open)).toBe(before);
});

test("the contact form can be sent and fixed with the keyboard alone", async ({ page }) => {
  await page.goto("/contact");
  const send = page.getByRole("button", { name: "Send message" });
  await expect(send).toBeEnabled();
  await send.focus();
  await page.keyboard.press("Enter");
  // Focus moves to the first field to fix, which says what to do.
  const name = page.getByLabel("Your name");
  await expect(name).toBeFocused();
  await expect(name).toHaveAttribute("aria-invalid", "true");
  await expect(name).toHaveAccessibleDescription(/\S/);
});
