# Progress

Updated at the end of every stage. Newest stage at the top of the log.

## Current state

- **Current stage:** 10b, Admissions screens. Done, awaiting review.
- **Next stage:** 11, Student records and academics (core).
- **Branch:** `stage-10b-admissions-screens` (from `main` after #27)

## Stage status

| Stage | Title | Status |
|---|---|---|
| 0 | Orientation and plan | Merged (#1) |
| 1 | Repo scaffold and tooling | Merged (#1) |
| 2 | Design system | Merged (#9, #10) |
| 3 | Database and migrations | Merged (#11) |
| 4 | Auth and RBAC | Merged (#12) |
| 5a | Content API | Merged (#14) |
| 5b | Staff content screens | Merged (#15) |
| 5c | Brand refresh | Merged (#16) |
| 6a | Public API | Merged (#17) |
| 6b | Public pages | Merged (#18) |
| 7a | Pages type, news and events API, legal drafts | Merged (#19, tagline artwork #20) |
| 7b | Public site part 2 pages | Merged (#21); CI fix in #22 |
| 8a | Staff enquiry inbox | Merged (#23) |
| 8b | Applicant sign-in and notifications | Merged (#24) |
| 9a | Application form with autosave | Merged (#25) |
| 9b | Documents and submission | Merged (#26) |
| 10a | Admissions workflow API | Merged (#27) |
| 10b | Admissions screens | Done, awaiting review |
| 11 to 21 | See `CLAUDE.md` section 3 | Not started |

## Open questions for the client or owner

Stage 0 questions were answered by the owner on 2026-10-22 (see `ASSUMPTIONS.md`). Still open:

| Topic | IDs | Needed by |
|---|---|---|
| Real contact, registration and programme content | DQ-01, DQ-03, DQ-04, DQ-05 | Stage 20 (dummy values in dev seed until then) |

DQ-02 (Pool Management content is editable by the college) and DQ-13 (English only for now) were closed on 2026-10-22.

## Log

### Stage 10b: Admissions screens (2026-10-26)

- Staff: Applications inbox with queues and filters, review screen with every decision the viewer may take, admissions report, overview counts, nav entry (D-128).
- Applicant: status page with the information request (upload and reply), the offer (letter, accept, decline) and withdraw (D-129).
- Fixed: inbox filters were blocked by the browser when a filter was "Any" (also in the 8a enquiry inbox) (D-130); wide tables widened the staff page on desktop (D-131).
- Verified: 107 unit tests; 128 Playwright tests passed locally, including the whole path from submitted to enrolled across three browsers (officer, applicant, registrar) and declining an offer, with axe and the overflow check on each new screen at 360 px and desktop. Screenshots checked by eye.

### Stage 10a: Admissions workflow API (2026-10-26)

- Stage 10 split into 10a (API) and 10b (screens) by the owner (D-119). Owner chose a 14-day offer window, an empty draft first invoice and a title-only signatory (A-22 to A-24).
- Migration 00013: allowed status changes as a table checked by a trigger (D-120), information requests, staff notes, students, invoices, `admissions.offer_response_days`, four message templates and the offer letter template.
- Staff API: inbox with filters and counts, detail with the submitted copy, documents, history, notes, requests, offer, student and allowed actions; assign (starts review), notes, request information, resume review, reject, offer with a PDF letter, enrol, report (D-121 to D-126).
- Applicant API: reply to a request (with uploads), accept the offer, withdraw or decline, offer letter link; offers lapse after their respond-by date (D-122).
- Enrolment: student number, student record, the applicant account becomes a student account, empty draft invoice, in one transaction (D-124).
- Messages screen handles the offer letter template (D-127).
- Verified: Go tests against PostgreSQL with `-race`, including every illegal status change refused by the database (90 pairs) and by the API (every action from every status), concurrent decisions and enrolments; 103 unit tests; 124 Playwright tests passed locally; a live offer through the site's `/api` rewrite stored its letter in MinIO and served it through a five-minute link.

### Stage 9b: Documents, submission and status tracking (2026-10-24)

- Migration 00012: passport documents, submitted snapshot, short-lived file links.
- Documents checked by content, photos re-encoded without metadata, optional ClamAV scanning (D-110, D-111); private files only through five-minute links bound to one account (D-112).
- Submission once, with idempotency keys, all in one transaction (D-113, D-114); status view with history (D-115).
- Fixed a deadlock in applicant sign-in found by the concurrency test (D-116), and a hang in simultaneous submits found by CI (D-118); Go tests now use a pool of 4 connections.
- Owner confirmed A-18 and A-20 and accepted A-19 (raise in DG-06). New default A-21 (required documents).
- Verified: Go tests against PostgreSQL; 103 unit tests; 124 Playwright tests passed locally, including upload and submission. Not run: a real ClamAV (image pull failed).

### Stage 9a: Application form with autosave (2026-10-23)

- Migration 00011: identity document type, KCSE subject grades, one guardian per application, return path on sign-in links.
- API: start an application (reference emailed, no duplicates), save each step, missing list, guardian consent, draft expiry (D-103 to D-107).
- Site: `/apply` starts the form; `/apply/new`; `/apply/application` with six steps, autosave and check your answers (D-108). Seed intakes dated from today (D-109).
- Owner confirmed A-16 and A-17. New defaults A-18 to A-20; risk R-26 (how parental consent is captured).
- Verified: Go tests against PostgreSQL; 102 unit tests; 124 Playwright tests passed locally, including the whole form from the programme page with axe on every step.

### Stage 8b: Applicant sign-in and message templates (2026-10-23)

- Migration 00010: applicant sign-in links, notification templates (email and SMS), notification log, `notifications.manage` (D-101).
- Applicants sign in by a one-time emailed link, or by reference number plus the email on the application; the account is created on first use (D-97 to D-99). Pages `/apply/start`, `/apply/resume`, `/apply/continue`, `/apply/account`.
- Routes can require an account kind; applicant and staff routes are kept apart (D-100).
- Staff screen "Messages to applicants" with placeholder checks, SMS length and preview. SMS is recorded as skipped until DG-03.
- Dev seed: Test Applicant 001 with draft `DTC-APP-SAMPLE-000001`; CI end-to-end job runs Mailpit (D-102).
- Verified: Go tests against PostgreSQL (sign-in, single use, expiry, staff address refused, rate limit, resume, separation of routes, concurrent first sign-in, templates); 122 Playwright tests passed locally, including the real emails through Mailpit.

### Stage 8a: Staff enquiry inbox (2026-10-23)

- Fixed CI on main (#22): build without the API; `/design` stable after its refresh (D-93).
- Stage 8 split into 8a and 8b (owner). Migration 00009 gives Finance Officers the inbox (D-94).
- Inbox API (list with office, status, owner and search filters; detail with notes and history; assign; status; notes) and staff screens; overview counts (D-95).
- Verified: Go tests for the done-when (a fee enquiry reaches the finance inbox and is assigned), permissions and rules; Playwright flow as the Finance Officer with axe; 111 Playwright tests in all.

### Stage 7b: Public site part 2 pages (2026-10-23)

- Admissions (how to apply), entry requirements, fees, intake calendar, searchable FAQs, news and articles, events, student life with gallery, alumni with transcript requests, privacy, cookies, terms, accessibility statement.
- Branded 404 and error pages (D-87); news and events cached with snapshot fallback (D-88); unpublished text pages say so and are not indexed (D-89).
- Sitemap lists published text pages, all news and events.
- Dev seed: sample news and events (D-91). WhatsApp button icon-only on phones (D-90).
- Verified: 107 Playwright tests (axe and 360 px on every public page, 404 status, FAQ search, news and events, text pages); 28 offline tests with the API stopped.

### Tagline artwork (2026-10-23)

- "Turning Potential into Progress" tagline logos (stacked and horizontal SVG) derived from the supplied artwork by `docs/brand/retagline.py`; the home hero shows the stacked one (D-86).

### Stage 7a: Pages type, news and events API, legal drafts (2026-10-22)

- Stage 7 split into 7a (backend, editor) and 7b (pages), approved by the owner.
- Pages content type (migration 00008) with staff editor; snapshot carries pages and gallery; news list and detail, events list and detail endpoints (D-82, D-83).
- Legal page drafts for review in `docs/legal/` (D-84); alumni enquiry type (D-85).
- Tagline corrected to "Progress" on the home hero, as text under the untagged logo (D-81).
- Owner supplied the phone and WhatsApp number; recorded in `docs/launch-content.md` for go-live.

### Stage 6b: Public pages (2026-10-22)

- Home, schools index and school pages, course finder, programme pages (five questions, Course structured data), about with the campus section, contact page and form, interim apply page, WhatsApp button, footer and announcement bar from site settings, sitemap, robots, per-page metadata with social cards.
- Cached content with background refresh and a bundled snapshot fallback (D-74); indexing off until launch (D-75).
- Fixed: headings on navy bands were invisible (D-79).
- Verified: unit tests; Playwright and axe on every public page at 360 px and desktop; AC-03 path in 3 clicks; AC-15 with the API stopped; build with no API reachable. Lighthouse mobile measured on three pages (AC-02 row).

### Stage 6a: Public API (2026-10-22)

- Stage 6 split into 6a (API) and 6b (pages), approved by the owner.
- `GET /api/v1/public/site`: one snapshot of published content for the public site (D-69).
- `POST /api/v1/public/enquiries`: contact form, saved to `enquiries`, emailed to the right office with Reply-To (D-71); built-in spam protection (D-70).
- Tagline corrected to "Turning Potential into Progress" (D-72, A-09); tagline artwork needs correcting (R-24).
- Verified: Go tests against PostgreSQL (published-only snapshot, hidden unverified claims, routing and fallback, validation, honeypot, fill time, per-contact limit, cross-site refusal); live through the Next.js rewrite with Mailpit.

### Stage 5b: Staff portal screens (2026-10-22)

- `/staff` portal: sign in (split layout), forgot and reset password, overview, approval queue, list and editor screens for all ten content types with preview, image upload, site settings. Navigation and actions follow permissions.
- Typed API client from the OpenAPI types; money and Nairobi date helpers.
- CI: new end-to-end job with the real API and database.
- Verified: editor drafts a programme in the browser, cannot publish; registrar publishes from the approval queue; an edit of the live programme waits for approval; axe clean on each screen; credentials never appear in a URL.
- Revised logo set from the owner: favicons, Apple touch icon, and Open Graph and Twitter image wired in the root metadata (D-68); `SITE_URL` added to `frontend/.env.example`. Header, footer, sign-in and portal shell checked at 360 px and 1280 px.

### Stage 5a: Content API (2026-10-22)

- Staff content API for schools, programmes (with requirements, intakes and fees), news, events, FAQ categories, FAQs, banners, gallery, statistics and accreditations: list, get, create, edit, pending edits of published items, publish, unpublish, archive, delete drafts.
- Site settings API; image upload with validation, rotation, metadata removal and resizing to MinIO or any S3 store.
- Migration 00007 (`content_pending`, image sizes on `files`).
- Verified with real MinIO through the Next.js rewrite: editor uploads an image and drafts a programme; editor cannot publish (403); registrar publishes (200).

### Stage 4: Auth and RBAC (2026-10-22)

- Login, logout, session endpoint, password reset by email, lockout, rate limits, CSRF and origin checks, audit entries for every auth and access change.
- Admin API for staff users and roles with the AC-12 protections; permission checks by key on every route.
- `setup` command for the first Super Admin from environment variables.
- Migration 00006 (index). Verified end to end through the Next.js rewrite with Mailpit.

### Stage 3: Database and migrations (2026-10-22)

- Five goose migrations (embedded): foundation (helpers, counters), identity and access (users, roles, permissions, sessions, password resets, audit log, starter roles and grants), files, consents and idempotency, public content (schools to site settings), enquiries and admissions.
- sqlc queries and generated code in `internal/db/dbq`; `migrate` and `seed` commands; Argon2id password hashing.
- Go moved to 1.26 (D-33).
- Verified: migrations apply from empty, roll back fully and re-apply (test); seed is idempotent and refuses production, staging and hosted hosts.

### Stage 2: Design system (2026-10-22)

- Tokens (Tailwind theme and CSS variables), fontsource fonts via `next/font/local`, supplied logo files copied unchanged to `public/brand/`, favicon and Apple icon.
- Components: Button and LinkButton, Input, Select, Textarea, Checkbox, Card, Tag and StatusTag, Tabs, Accordion, Dialog, Toast, Table, Pagination, Breadcrumb, Alert, Skeleton and Loading, EmptyState. Icon set with one icon per school.
- Shell: skip link, announcement bar (hidden when empty), utility bar, navy header with mega-menus and mobile menu, footer with contact rows that hide until real values exist.
- `/design` page (noindex, dev only) with measured contrast table.
- Verified: axe has no critical or serious issues on `/design`, with the dialog open, and with menus open, at 360 px and on desktop; no horizontal scrolling at 360 px. Home JS is above budget (R-22).

### Stage 1: Repo scaffold and tooling (2026-10-22)

- Backend: Go 1.25 module, Gin router with `/api/v1/health` and `/api/v1/ready`, env config with validation, pgx pool, request ID, JSON access log, panic recovery, one error shape. `openapi.yaml` describes both endpoints.
- Docker: multi-stage distroless Dockerfile with a built-in healthcheck. Compose runs API, PostgreSQL 17.11, MinIO (Chainguard build, D-20) and Mailpit. All ports bound to 127.0.0.1, on 8180 and 5442 (D-21).
- Frontend: Next.js 16.4.2, React 19.3, Tailwind 4.3, TypeScript strict, ESLint plus Prettier, Vitest, Playwright with axe, `/api/*` rewrite to the Go API, OpenAPI-generated types. Brand favicon copied from `docs/brand`. No Google Fonts.
- Tooling: root and backend Makefiles, `.editorconfig`, CI (Go with a Postgres service, frontend checks, Playwright, gitleaks), Dependabot.
- Verified: `docker compose up` serves the API; `npm run dev` serves the site and proxies `/api/v1/ready`. See the Stage 1 report for the commands.

### Stage 0: Orientation and plan (2026-10-22)

- Read `CLAUDE.md`, the brief (via `textutil`, since `pandoc` is not installed), and the logo files.
- Created `docs/PLAN.md`, `docs/PROGRESS.md`, `docs/ASSUMPTIONS.md`, `docs/DECISIONS.md`, `docs/traceability.md`.
- No feature code.
- Owner answered the Stage 0 questions the same day; answers recorded in `ASSUMPTIONS.md` and `DECISIONS.md`.
