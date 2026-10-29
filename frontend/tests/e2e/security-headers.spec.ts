import { expect, test, type Page } from "@playwright/test";

// Stage 17a: the Content-Security-Policy and other headers on real pages
// (D-191, D-192). Public pages keep a fixed policy so they stay cached;
// signed-in pages get a fresh nonce, and nothing the site does may be blocked.

/** Records CSP violations the page reports, from before any script runs. */
async function watchViolations(page: Page): Promise<() => Promise<string[]>> {
  const consoleErrors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error" && /Content Security Policy/i.test(m.text())) consoleErrors.push(m.text());
  });
  await page.addInitScript(() => {
    const w = window as unknown as { __csp: string[] };
    w.__csp = [];
    document.addEventListener("securitypolicyviolation", (e) => {
      w.__csp.push(`${e.violatedDirective} ${e.blockedURI}`);
    });
  });
  return async () => [
    ...consoleErrors,
    ...(await page.evaluate(() => (window as unknown as { __csp?: string[] }).__csp ?? [])),
  ];
}

function scriptSrc(policy: string | null) {
  return (
    policy
      ?.split(";")
      .find((d) => d.trim().startsWith("script-src"))
      ?.trim() ?? ""
  );
}

test("public pages carry the fixed policy and the standard headers", async ({ page }) => {
  const violations = await watchViolations(page);
  for (const path of ["/", "/programmes", "/apply"]) {
    const res = await page.goto(path);
    const h = res!.headers();
    expect(scriptSrc(h["content-security-policy"]), path).toBe("script-src 'self' 'unsafe-inline'");
    expect(h["content-security-policy"]).toContain("frame-ancestors 'none'");
    expect(h["x-frame-options"]).toBe("DENY");
    expect(h["x-content-type-options"]).toBe("nosniff");
    expect(h["referrer-policy"]).toBe("strict-origin-when-cross-origin");
    expect(h["strict-transport-security"]).toBe("max-age=31536000");
    expect(h["permissions-policy"]).toContain("camera=()");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  }
  expect(await violations()).toEqual([]);
});

test("signed-in pages get a fresh nonce and still work", async ({ page }) => {
  const violations = await watchViolations(page);
  const nonces = new Set<string>();
  for (const path of ["/staff/login", "/student/login", "/apply/start"]) {
    const res = await page.goto(path);
    const src = scriptSrc(res!.headers()["content-security-policy"]);
    expect(src, path).toMatch(/^script-src 'self' 'nonce-[A-Za-z0-9+/=]+' 'strict-dynamic'$/);
    nonces.add(src);
    // Sending the empty form shows field messages, which needs the page's
    // scripts to have run under the policy.
    await page
      .getByRole("button", { name: /^(Sign in|Email me a link)$/ })
      .first()
      .click();
    await expect(page.getByRole("alert").or(page.locator("[aria-invalid=true]")).first()).toBeVisible();
  }
  expect(nonces.size).toBe(3);
  expect(await violations()).toEqual([]);

  // Control: injected markup with an inline handler, the usual shape of an
  // XSS payload, is blocked and reported, so the checks above are not
  // passing vacuously.
  const ran = await page.evaluate(async () => {
    const w = window as unknown as { __injected?: boolean };
    const div = document.createElement("div");
    div.innerHTML = '<img src="/__missing__.png" alt="" onerror="window.__injected = true">';
    document.body.append(div);
    await new Promise((r) => setTimeout(r, 500));
    return w.__injected === true;
  });
  expect(ran).toBe(false);
  expect((await violations()).join()).toContain("script-src");
});

test("a portal link on the public site loads the strict page fresh", async ({ page }) => {
  await page.goto("/");
  const [doc] = await Promise.all([
    page.waitForResponse(
      (r) => r.request().resourceType() === "document" && new URL(r.url()).pathname === "/staff",
    ),
    page.getByRole("contentinfo").getByRole("link", { name: "Staff Portal" }).click(),
  ]);
  expect(scriptSrc(doc.headers()["content-security-policy"])).toContain("'nonce-");
});

const password = process.env.E2E_STAFF_PASSWORD ?? "";

test("staff and student portals run with no policy violations", async ({ browser }, testInfo) => {
  test.skip(
    process.env.E2E_API !== "1" || !password,
    "set E2E_API=1 and E2E_STAFF_PASSWORD with the backend running",
  );
  test.setTimeout(90_000);

  const staff = await (await browser.newContext()).newPage();
  const staffViolations = await watchViolations(staff);
  await staff.goto("/staff/login");
  await staff.getByLabel("Email address").fill("principal@example.invalid");
  await staff.getByLabel("Password").fill(password);
  await staff.getByRole("button", { name: "Sign in" }).click();
  await expect(staff.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  const nav = staff.getByRole("navigation", { name: "Staff" });
  for (const [link, heading] of [
    ["Applications", "Applications"],
    ["Students", "Students"],
    ["Finance", "Finance"],
    ["Reports", "Reports"],
  ]) {
    await nav.getByRole("link", { name: link, exact: true }).click();
    await expect(staff.getByRole("heading", { level: 1, name: heading })).toBeVisible();
  }
  expect(await staffViolations()).toEqual([]);

  const student = await (await browser.newContext()).newPage();
  const studentViolations = await watchViolations(student);
  await student.goto("/student/login");
  await student
    .getByLabel("Student number")
    .fill(testInfo.project.name === "mobile" ? "DTC/SAMPLE/0001" : "DTC/SAMPLE/0004");
  await student.getByLabel("Password").fill(password);
  await student.getByRole("button", { name: "Sign in" }).click();
  await expect(student.getByRole("heading", { level: 1, name: /^Hello/ })).toBeVisible();
  for (const name of ["Timetable", "Fees", "Profile"]) {
    await student
      .getByRole("navigation", { name: "Student" })
      .getByRole("link", { name, exact: true })
      .click();
    await expect(student.getByRole("heading", { level: 1, name })).toBeVisible();
  }
  expect(await studentViolations()).toEqual([]);
});
