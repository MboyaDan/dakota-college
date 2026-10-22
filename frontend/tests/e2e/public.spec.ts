import { expect, test } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";

// Public site (Stage 6). These need published content, so they run against
// the real API and the dev seed (E2E_API=1), as in CI.
test.skip(process.env.E2E_API !== "1", "set E2E_API=1 with the backend running and seeded");

const programmePath = "/programmes/example-health-studies-diploma";

const pages = [
  "/",
  "/schools",
  "/schools/ict",
  "/programmes",
  "/programmes?level=diploma&grade=C",
  programmePath,
  "/about",
  "/contact",
  "/apply?programme=example-health-studies-diploma",
  "/admissions",
  "/admissions/requirements",
  "/admissions/fees",
  "/admissions/intakes",
  "/faqs",
  "/faqs?q=fees",
  "/news",
  "/news/sample-news-applications",
  "/events",
  "/events/sample-open-day",
  "/student-life",
  "/alumni",
  "/privacy",
  "/cookies",
  "/terms",
  "/accessibility",
  "/no-such-page-anywhere",
];

for (const path of pages) {
  test(`${path} has no serious axe issues and fits the screen`, async ({ page }) => {
    const res = await page.goto(path);
    expect(res?.status()).toBe(path.startsWith("/no-such") ? 404 : 200);
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    await expectNoSeriousA11yIssues(page);
    expect(await hasHorizontalOverflow(page)).toBe(false);
  });
}

// The page shell streams before the lookup, so Next.js answers 200 with a
// noindex tag and the not-found content (D-77).
for (const path of ["/programmes/no-such-programme", "/schools/no-such-school"]) {
  test(`${path} shows not found and is not indexed`, async ({ page }) => {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("We could not find that page");
    await expect(page.locator('meta[name="robots"][content*="noindex"]').first()).toBeAttached();
  });
}

// AC-03: from the home page, a visitor finds a programme, sees fees,
// requirements and intake, and starts an application in at most four clicks.
test("home to the start of an application in four clicks or fewer", async ({ page }) => {
  let clicks = 0;
  await page.goto("/");

  await page.getByRole("link", { name: "Find a course" }).first().click();
  clicks++;
  await expect(page).toHaveURL(/\/programmes$/);

  await page.getByRole("link", { name: "Diploma in Example Health Studies (Sample)" }).click();
  clicks++;
  await expect(page.getByRole("heading", { level: 2, name: "Fees" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Entry requirements" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Intakes" })).toBeVisible();
  await expect(page.getByRole("term").filter({ hasText: "Next intake" })).toBeVisible();

  await page.getByRole("link", { name: "Apply for this programme" }).first().click();
  clicks++;
  await expect(page).toHaveURL(/\/apply\?programme=example-health-studies-diploma/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Apply to Dakota Training College");
  // Next.js keeps the previous page hidden in the DOM, so match the visible choice.
  await expect(
    page.locator("strong", { hasText: "Diploma in Example Health Studies (Sample)" }),
  ).toBeVisible();
  expect(clicks).toBeLessThanOrEqual(4);
});

test("course finder filters by level and keeps them in the URL", async ({ page }) => {
  await page.goto("/programmes");
  await page.getByLabel("Level").selectOption("certificate");
  await page.getByRole("button", { name: "Show programmes" }).click();
  await expect(page).toHaveURL(/level=certificate/);
  await expect(page.getByRole("heading", { level: 2, name: /match your search/ })).toBeVisible();
  const cards = page
    .getByRole("main")
    .getByRole("listitem")
    .filter({ has: page.getByRole("heading", { level: 3 }) });
  await expect(cards.first()).toBeVisible();
  for (const card of await cards.all())
    await expect(card.getByText("Certificate", { exact: true })).toBeVisible();

  await page.goto("/programmes?q=zzzz-no-such-course");
  await expect(page.getByText("No programmes match")).toBeVisible();
});

test("contact form explains what is missing, then sends", async ({ page }) => {
  await page.goto(`/contact?programme=example-health-studies-diploma`);
  const send = page.getByRole("button", { name: "Send message" });
  await expect(send).toBeEnabled();
  await expect(page.getByLabel("Programme")).toHaveValue("example-health-studies-diploma");

  await send.click();
  await expect(page.getByText("Enter your name.")).toBeVisible();
  await expect(page.getByText("Give an email address or a phone number, so we can reply.")).toBeVisible();
  await expect(page.getByLabel("Your name")).toBeFocused();

  await page.getByLabel("Your name").fill("Test Visitor 001");
  await page.getByLabel("Email address").fill(`visitor-${Date.now()}@example.invalid`);
  await page.getByLabel("Your question", { exact: true }).fill("When does the next intake start?");
  // A person takes more than three seconds; the API drops faster posts (D-70).
  await page.waitForTimeout(3200);
  await send.click();
  await expect(
    page.getByRole("status").filter({ hasText: "Thank you, your message is on its way" }),
  ).toBeVisible();
});

test("sitemap lists public pages; robots keeps crawlers out before launch", async ({ request }) => {
  const sitemap = await (await request.get("/sitemap.xml")).text();
  expect(sitemap).toContain(programmePath);
  expect(sitemap).toContain("/schools/ict");
  expect(sitemap).not.toContain("/staff");
  const robots = await (await request.get("/robots.txt")).text();
  expect(robots).toMatch(/Disallow: \/\s*$/m);
});

test("unknown addresses get the site's 404 page with a way forward", async ({ page }) => {
  const res = await page.goto("/no-such-page-anywhere");
  expect(res?.status()).toBe(404);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("We could not find that page");
  await expect(page.getByRole("search", { name: "Find a course" })).toBeVisible();
  await expect(page.getByRole("contentinfo")).toBeVisible();
});

test("FAQ search narrows the questions and keeps the words in the URL", async ({ page }) => {
  await page.goto("/faqs");
  await expect(page.getByRole("navigation", { name: "Topics" })).toBeVisible();
  await page.getByRole("searchbox", { name: "Search questions" }).fill("instalments");
  await page.getByRole("button", { name: "Search" }).click();
  await expect(page).toHaveURL(/q=instalments/);
  await expect(page.getByRole("status")).toContainText("1 question matches");
  await expect(page.getByText("(Sample) Can I pay in instalments?")).toBeVisible();
  await expect(page.getByText("(Sample) How do I apply?")).toHaveCount(0);
});

test("news list leads to the article, events list to the event", async ({ page }) => {
  await page.goto("/news");
  await page.getByRole("link", { name: "(Sample) Applications news" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("(Sample) Applications news");
  await expect(page.getByRole("heading", { level: 2, name: "A sample heading" })).toBeVisible();
  await expect(page.locator("main li", { hasText: "(Sample) A list item" })).toBeVisible();

  await page.goto("/events");
  await expect(page.getByRole("heading", { level: 2, name: "Coming up" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Recent events" })).toBeVisible();
  await page.getByRole("link", { name: "(Sample) Open day" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("(Sample) Open day");
});

test("text pages show the published words; the alumni button preselects the topic", async ({ page }) => {
  await page.goto("/privacy");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("(Sample) Draft: Privacy notice");
  await expect(page.getByRole("heading", { level: 2, name: "Your rights" })).toBeVisible();

  await page.goto("/alumni");
  await page.getByRole("link", { name: "Request a transcript or certificate" }).click();
  await expect(page.getByLabel("What is your question about?")).toHaveValue("alumni");
});
