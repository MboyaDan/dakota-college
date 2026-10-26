import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { expectNoSeriousA11yIssues, hasHorizontalOverflow } from "./axe";
import { linkFromEmail, mailpit } from "./mailpit";

// Stage 10b: an application goes from submitted to enrolled through the
// staff screens and the applicant's status page, against the real API, the
// dev seed and Mailpit.
const password = process.env.E2E_STAFF_PASSWORD ?? "";
test.skip(
  process.env.E2E_API !== "1" || !password || !mailpit,
  "set E2E_API=1, E2E_STAFF_PASSWORD and E2E_MAILPIT_URL with the backend running",
);

const pdf = Buffer.from("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n");
// A 1 x 1 pixel PNG.
const png = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "base64",
);

/**
 * Signs an applicant in through the emailed link and submits a complete
 * application through the API, with `api` (a page's request context, so the
 * page shares the session).
 */
async function submitApplication(api: APIRequestContext, email: string, name: string) {
  const since = Date.now() - 2_000;
  expect(
    (await api.post("/api/v1/applicant/sign-in-link", { data: { email, full_name: name } })).status(),
  ).toBe(202);
  const token = new URL(await linkFromEmail(api, email, since), "http://x").searchParams.get("token");
  const signIn = await api.post("/api/v1/applicant/sign-in", { data: { token } });
  expect(signIn.status()).toBe(200);
  const headers = { "X-CSRF-Token": ((await signIn.json()) as { csrf_token: string }).csrf_token };

  const started = await api.post("/api/v1/applicant/applications", {
    headers,
    data: { programme: "example-information-technology-diploma" },
  });
  expect(started.status()).toBe(201);
  const { id } = (await started.json()) as { id: string };
  const put = async (step: string, data: unknown) =>
    expect((await api.put(`/api/v1/applicant/applications/${id}/${step}`, { headers, data })).status()).toBe(
      200,
    );
  const adult = new Date();
  adult.setFullYear(adult.getFullYear() - 20);
  await put("personal", {
    first_name: "Test",
    middle_name: "",
    last_name: name.replace(/^Test /, ""),
    date_of_birth: adult.toISOString().slice(0, 10),
    identity_document_type: "national_id",
    identity_document: "00000099",
    nationality: "Kenyan",
    county: "Kisumu",
    phone: "+254 700 000 099",
  });
  await put("education", {
    items: [
      {
        level: "kcse",
        institution: "Test Secondary School",
        year_completed: 2024,
        qualification: "",
        mean_grade: "B",
        subjects: [{ subject: "Mathematics", grade: "B+" }],
      },
    ],
  });
  await put("sponsor", { type: "self", name: "", phone: "" });
  for (const [kind, file] of [
    ["national_id", { name: "id.pdf", mimeType: "application/pdf", buffer: pdf }],
    ["passport_photo", { name: "photo.png", mimeType: "image/png", buffer: png }],
    ["kcse_certificate", { name: "kcse.pdf", mimeType: "application/pdf", buffer: pdf }],
  ] as const) {
    const up = await api.post(`/api/v1/applicant/applications/${id}/documents`, {
      headers,
      multipart: { kind, file },
    });
    expect(up.status(), await up.text()).toBe(201);
  }
  const submitted = await api.post(`/api/v1/applicant/applications/${id}/submit`, {
    headers: { ...headers, "Idempotency-Key": `e2e-${Date.now()}-${Math.random()}` },
    data: { declaration: true },
  });
  expect(submitted.status()).toBe(200);
  const { reference } = (await submitted.json()) as { reference: string };
  return { id, reference };
}

async function staffSignIn(page: Page, email: string) {
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

test("an application is reviewed, offered, accepted and enrolled", async ({ browser }, testInfo) => {
  test.setTimeout(120_000);
  const stamp = `${testInfo.project.name}-${Date.now()}`;
  const email = `e2e-admit-${stamp}@example.invalid`;
  const name = `Test Admit ${stamp}`;

  const applicant = await (await browser.newContext()).newPage();
  const officer = await (await browser.newContext()).newPage();
  const registrar = await (await browser.newContext()).newPage();
  const { id, reference } = await submitApplication(applicant.request, email, name);
  const applicationPath = `/apply/application?id=${id}`;

  // The officer finds it from the overview and the inbox.
  await staffSignIn(officer, "admissions@example.invalid");
  await expect(officer.getByRole("heading", { level: 2, name: "Applications" })).toBeVisible();
  await officer
    .getByRole("navigation", { name: "Staff" })
    .getByRole("link", { name: "Applications" })
    .click();
  await expect(officer.getByRole("heading", { level: 1, name: "Applications" })).toBeVisible();
  await officer.getByLabel("Search").fill(reference);
  await officer.getByRole("button", { name: "Show" }).click();
  await expect(officer.getByRole("status")).toHaveText("1 application");
  await checkPage(officer);
  await officer.getByRole("link", { name: new RegExp(name) }).click();

  await expect(officer.getByRole("heading", { level: 1, name })).toBeVisible();
  await expect(officer.getByText("Not reviewed yet")).toBeVisible();
  await expect(officer.getByText("Mathematics B+")).toBeVisible();
  await checkPage(officer);

  // Assign starts the review; then ask for more.
  await officer.getByRole("button", { name: "Assign to me" }).click();
  await expect(officer.getByText("In review", { exact: true }).first()).toBeVisible();
  await officer
    .getByLabel("What do you need from the applicant?")
    .fill("Please upload a clearer KCSE result slip.");
  await officer.getByRole("button", { name: "Send request" }).click();
  await expect(officer.getByText("Waiting on the applicant", { exact: true })).toBeVisible();

  // The applicant uploads and replies.
  await applicant.goto(applicationPath);
  await expect(
    applicant.getByRole("heading", { level: 2, name: "The admissions office needs more from you" }),
  ).toBeVisible();
  await expect(applicant.getByText("Please upload a clearer KCSE result slip.")).toBeVisible();
  await checkPage(applicant);
  await applicant.getByLabel("What is it?").selectOption("kcse_certificate");
  await applicant
    .getByLabel("Upload a document")
    .setInputFiles({ name: "kcse-clear.pdf", mimeType: "application/pdf", buffer: pdf });
  await expect(applicant.getByText("KCSE certificate or result slip uploaded.")).toBeVisible();
  await applicant.getByRole("button", { name: "Send reply" }).click();
  await expect(applicant.getByText("Write your reply before sending it.")).toBeVisible();
  await applicant.getByLabel("Your reply").fill("I have uploaded a clearer copy.");
  await applicant.getByRole("button", { name: "Send reply" }).click();
  await expect(applicant.getByText("Being reviewed", { exact: true }).first()).toBeVisible();

  // The officer sees the reply and the new document, and offers a place.
  await officer.reload();
  await expect(officer.getByText("I have uploaded a clearer copy.")).toBeVisible();
  await expect(officer.getByText(/sent later/)).toBeVisible();
  await officer.getByLabel("Conditions of the offer").fill("Bring your original KCSE certificate.");
  await officer.getByRole("button", { name: "Offer a place" }).click();
  await expect(officer.getByRole("heading", { level: 2, name: "Offer" })).toBeVisible();
  await expect(officer.getByRole("button", { name: "Open the offer letter" })).toBeVisible();
  await checkPage(officer);

  // The letter is a PDF behind a short-lived link.
  const session = (await (await officer.request.get("/api/v1/auth/session")).json()) as {
    csrf_token: string;
  };
  const link = await officer.request.post(`/api/v1/staff/applications/${id}/offer-letter/link`, {
    headers: { "X-CSRF-Token": session.csrf_token },
  });
  const letter = await officer.request.get(((await link.json()) as { url: string }).url);
  expect(letter.headers()["content-type"]).toBe("application/pdf");
  expect((await letter.body()).subarray(0, 5).toString()).toBe("%PDF-");

  // The applicant accepts.
  await applicant.reload();
  await expect(
    applicant.getByRole("heading", { level: 2, name: "You have been offered a place" }),
  ).toBeVisible();
  await expect(applicant.getByText("Bring your original KCSE certificate.")).toBeVisible();
  await checkPage(applicant);
  await applicant.getByRole("button", { name: "Accept offer" }).click();
  await applicant.getByRole("button", { name: "Yes, accept" }).click();
  await expect(applicant.getByText("Offer accepted", { exact: true }).first()).toBeVisible();

  // Only the registrar can enrol.
  await officer.reload();
  await expect(officer.getByText("Accepted, to enrol", { exact: true }).first()).toBeVisible();
  await expect(officer.getByRole("button", { name: "Enrol as a student" })).toHaveCount(0);
  await staffSignIn(registrar, "registrar@example.invalid");
  await registrar.goto(`/staff/applications/${id}`);
  await registrar.getByRole("button", { name: "Enrol as a student" }).click();
  await registrar.getByRole("button", { name: "Enrol now" }).click();
  await expect(registrar.getByRole("heading", { level: 2, name: "Student" })).toBeVisible();
  await expect(registrar.locator(".font-mono", { hasText: /^DTC\/\d{4}\/\d{4}$/ })).toBeVisible();
  await expect(registrar.getByText("Enrolled", { exact: true }).first()).toBeVisible();
  await checkPage(registrar);

  // The report counts it.
  await registrar.getByRole("link", { name: "Applications", exact: true }).first().click();
  await registrar.getByRole("link", { name: "Admissions report" }).click();
  await expect(registrar.getByRole("heading", { level: 1, name: "Admissions report" })).toBeVisible();
  await expect(registrar.getByRole("table")).toBeVisible();
  await checkPage(registrar);
});

test("an applicant can decline an offer", async ({ browser }, testInfo) => {
  test.setTimeout(90_000);
  const stamp = `${testInfo.project.name}-${Date.now()}`;
  const name = `Test Decline ${stamp}`;
  const applicant = await (await browser.newContext()).newPage();
  const { id } = await submitApplication(applicant.request, `e2e-decline-${stamp}@example.invalid`, name);

  const officer = await (await browser.newContext()).newPage();
  await staffSignIn(officer, "admissions@example.invalid");
  await officer.goto(`/staff/applications/${id}`);
  await officer.getByRole("button", { name: "Assign to me" }).click();
  await expect(officer.getByRole("button", { name: "Offer a place" })).toBeVisible();
  await officer.getByRole("button", { name: "Offer a place" }).click();
  await expect(officer.getByRole("heading", { level: 2, name: "Offer" })).toBeVisible();

  await applicant.goto(`/apply/application?id=${id}`);
  await applicant.getByRole("button", { name: "Decline offer" }).click();
  await applicant.getByLabel("Why are you declining?").fill("I am moving to another town.");
  await applicant.getByRole("dialog").getByRole("button", { name: "Decline offer" }).click();
  await expect(applicant.getByText("Withdrawn", { exact: true }).first()).toBeVisible();
  await expect(
    applicant.getByText("Applicant declined the offer: I am moving to another town."),
  ).toBeVisible();
  await checkPage(applicant);
});
