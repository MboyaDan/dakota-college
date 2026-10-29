# Progress

Updated at the end of every stage. Newest stage at the top of the log.

## Current state

- **Current stage:** 17d, Home page photos and founders page (owner request). Done, awaiting review.
- **Next stage:** 17b, optional two-step sign-in (TOTP) for finance, registrar and admin staff (KCB signature check slots in when KCB sends the key).
- **Branch:** `stage-17d-home-photos-founders` (from `main` after #44)

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
| 10b | Admissions screens | Merged (#28) |
| 11a | Student records and academics API | Merged (#29) |
| 11b | Student records and academics screens | Merged (#30) |
| 12a | Student portal API | Merged (#31) |
| 12b | Student portal screens | Merged (#32) |
| 13a | Results API | Merged (#33) |
| 13b | Results screens | Merged (#34) |
| 14a | Finance API | Merged (#35) |
| 14b | Finance screens | Merged (#36) |
| 15a | M-Pesa C2B | Merged (#37) |
| 15a.2 | M-Pesa hardening and sandbox tunnel | Merged (#38) |
| 15b | KCB Buni notifications | Merged (#39, gitleaks fix #40); signature check waits for KCB |
| 16a | Message queue with retries | Merged (#41) |
| 16b | Reports API: dashboard data and count exports | Merged (#42) |
| 16c | Dashboard pages and export buttons | Merged (#43) |
| 17a | Security review, headers, rate limits, dependency audit | Done (#44) |
| 17b | Two-step sign-in (TOTP) | Not started |
| 17c | Retention jobs, data subject requests, breach procedure | Not started |
| 17d | Home page photos and founders page (owner request) | Done, awaiting review |
| 18 to 21 | See `CLAUDE.md` section 3 | Not started |

## Open questions for the client or owner

Stage 0 questions were answered by the owner on 2026-10-22 (see `ASSUMPTIONS.md`). Still open:

| Topic | IDs | Needed by |
|---|---|---|
| Real contact, registration and programme content | DQ-01, DQ-03, DQ-04, DQ-05 | Stage 20 (dummy values in dev seed until then) |

DQ-02 (Pool Management content is editable by the college) and DQ-13 (English only for now) were closed on 2026-10-22.

## Log

### Stage 17d: Home page photos and founders page (2026-10-29)

- Owner asked for photo backgrounds on the school tiles, a richer home page and a founders page. Photos are staff uploads with consent only; until then, branded designs, never stock or generated people (D-200).
- Migration 00022: school photos (description, consent to publish), gallery "home page photo" tick, `founders` content type (D-201 to D-203). Staff content manager: photo fields on schools, the tick on gallery photos, a Founders screen.
- Site: school tiles with photo and navy fade or plain navy with the school's colour; hero photo with a gold frame (logo artwork until there is one); navy values band; founder teaser; "Life at Dakota" strip from the gallery; CSS texture backgrounds (D-205). New `/about/founders`, founders on About, in the About menu and the sitemap.
- Site tolerates snapshots from an older API (D-204).
- Verified: Go `httpapi`, `db`, `seed` with PostgreSQL and `-race` (new `TestPublicPhotosAndFounders`; founders added to `TestContentKinds`); migration up, down, up on the dev database; lint, typecheck, build; 137 unit tests; Playwright public and security specs 86 of 86 (new founders and school tile tests, axe on `/about/founders`); staff spec 7 passed, 1 skipped; offline spec 30 of 30 with the API stopped. Contrast over photos checked by eye with plain test images (one near white) uploaded only to the local database, then removed.

### Stage 17a: Security review, headers and rate limits (2026-10-29)

- Stage 17 split into 17a, 17b (TOTP) and 17c (retention, data subject requests); owner chose nonces on signed-in pages, CI failing on high or critical advisories, and ClamAV as an opt-in service tested locally and in CI (D-191, D-193, D-194, D-199).
- Written review `docs/security.md` (assets, attackers, boundaries, controls with evidence, findings F-01 to F-06, open items before real data) and a generated route list `docs/endpoints.md` (208 routes) kept current by a test (D-198).
- Fixed: anyone could lock every account out of sign-in, because all visitors shared one address behind the rewrite (R-23). The frontend proxy now passes the platform's client address with a shared secret and the API believes it only with that secret (D-195). New env vars: `CLIENT_IP_SECRET` (API, required in staging and production), `CLIENT_IP_HEADER`, `CLIENT_IP_SECRET` and `CONTENT_IMAGE_ORIGIN` (frontend).
- Site: Content-Security-Policy from `proxy.ts`, with a nonce on the staff portal, student portal and application form (now rendered per request; form pages moved into an `apply/(form)` route group) and a fixed policy on cached public pages; HSTS, frame, referrer and permissions headers; links into the signed-in areas load fresh (D-191, D-192). API: security headers on every response, `no-store` by default (D-192).
- Permission routes need a staff account and roles are for staff only (D-196). Sign-in forms keep text typed before the scripts load (D-197).
- Dependencies: quic-go raised to v0.59.1 (GO-2026-5676, reachable); CI runs govulncheck and npm audit. ClamAV: Compose and CI use the Debian image, which runs on arm64; new CI job scans EICAR against a real clamd.
- No migrations.
- Verified: all Go packages with PostgreSQL and `-race` (new: `TestSecurityHeaders`, `TestTrustedClientIP`, `TestVerifiedVisitorAddressReachesAuditAndLimits`, `TestPermissionRoutesNeedAStaffAccount`, `TestApplicantRoutesRefuseOtherApplicants`, `TestEndpointInventoryIsCurrent`, `TestClientIPSecret`); `TestClamAVLive` against a local ClamAV (EICAR found, clean file passed); govulncheck 0 reachable; npm audit 0; 137 unit tests; typecheck, lint, Prettier, build; Playwright 170 passed, 0 failed, 42 skipped by design (offline, M-Pesa, KCB, one-width tests), plus the offline spec with the API stopped (28 passed); every script on 8 signed-in pages carried the response's nonce (curl on the built site); live check through the rewrite: a verified visitor audited under their own address, a forged header ignored.

### Stage 16c: Reports page and export buttons (2026-10-28)

- `/staff/reports` (nav "Reports", `reports.view`) with a period form; shows the sections the API returns: admissions, students, results, finance, enquiries, each with stat tiles and a table with totals (D-188, D-189). Empty state for roles that may open reports but see no data (ICT Admin).
- CSV buttons (admissions, enrolments, pass rates, fee collection) for viewers with `reports.export`, downloading the period on screen; refusals show a message (D-190). Overview card links to the page.
- No backend or migration changes.
- Verified: typecheck, lint, Prettier, build; 126 unit tests (5 new for percentages, file names, months); Playwright `reports.spec.ts` 10 of 10 at 360 px and desktop (Principal sees five sections and four buttons and downloads admissions: file name, BOM, header, Total row; Finance sees no admissions and downloads fee collection; Head of School sees admissions without buttons; ICT Admin sees the empty state; a reversed period shows the field message; axe and no overflow). Full suite: 161 passed, 7 timed out on a slowed run (display asleep), all 7 passed on rerun. Screenshots checked by eye at 360 px and desktop.

### Stage 16b: Reports API (2026-10-28)

- Stage 16b split into 16b (API) and 16c (screens); only count exports for now, so no approval yet; Registrar and Finance Officer may export what they can see (owner, D-183, D-184).
- Migration 00021: `reports.export` for Registrar and Finance Officer.
- `internal/reports`: one dashboard by permission (admissions, students, results, finance, enquiries; null when not allowed) for a period of Nairobi dates (D-185, D-186); four CSV exports (admissions, enrolments, fee collection, results) with formula-safe cells, integer money and audit (D-187).
- Verified: Go tests against PostgreSQL with `-race` (`TestDashboardAndExports`: every figure from known data, period filtering, each role's sections, data permission needed per export, unknown export, bad periods, CSV bytes, BOM, Total rows, formula guard, audit count; unit tests for shillings, percentages and the formula guard; route walk and OpenAPI match); migration 00021 up, down and up on a scratch database; live check against the migrated dev API (Registrar: no finance section; enrolments CSV downloads).

### Stage 16a: Message queue with retries (2026-10-28)

- Stage 16 split into 16a (queue) and 16b (dashboards and exports); exports with personal data will need a second person's approval (brief 7.2); DG-03 not cleared (owner, D-177).
- Migration 00020: `notifications` becomes the queue (encrypted text while waiting, attempts, next try, expiry, claim), `system_key` for staff-facing emails, `sending` status; rows queued before the migration are marked failed.
- `internal/outbox`: worker in the API process with retries after 1, 5, 15, 60 and 240 minutes; refused addresses fail at once; text encrypted with `MESSAGE_KEY` and bound to the recipient, erased once sent or expired; links expire with their message (D-178 to D-180).
- All email now goes through it: applicant, student and admissions templates, plus password resets and enquiry routing and assignment, which used to be fire-and-forget.
- Staff: "Sent messages" with filters and "Send again" for failed messages, a link from the templates page, an overview card when messages failed in the last week (D-181). SMS stays skipped behind an `SMSSender` interface until DG-03 (D-182).
- New env var `MESSAGE_KEY` (required in staging and production).
- Verified: Go tests against PostgreSQL with `-race` (outbox: retry timing, give up and resend, refused address, expiry with the link, text erased, abandoned claim, changed recipient unreadable, four workers sharing 35 messages; API: log, filters, resend, audit, permissions, an applicant email waiting out an outage); migration 00020 up, down and up on a scratch database; live: a reset email sent through the queue to Mailpit, and with Mailpit stopped the email waited and was delivered on the one-minute retry; 121 unit tests; 151 Playwright tests passed locally including the new sent-messages test with axe and the overflow check at 360 px and desktop. Screenshots checked by eye.

### Stage 15b: KCB Buni notifications (2026-10-27)

- Built from KCB's IPN specification (kept locally in `docs/kcb/`, not committed). The owner's Buni sandbox keys obtain a UAT token.
- Owner chose to build everything except the signature check: KCB notifications create pending bank payments that Finance confirms; unmatched ones wait to be assigned or returned (D-174 to D-176; A-42, A-43).
- Migration 00019: stored notifications with their Signature header (append-only), one row per KCB transaction (guarded), payment `source`.
- API: secret-path notification endpoint with KCB's acknowledgement; status, transactions, assign, refund, day check, reprocess. Shared callback guard with M-Pesa. Screens: KCB bank page; M-Pesa and KCB share the assign and return form.
- Verified: all Go packages with PostgreSQL and `-race`; 121 unit tests; 156 Playwright tests passed locally with M-Pesa and KCB enabled; a KCB-shaped notification sent through the ngrok tunnel and the callback proxy reached the API and became a pending payment. Screenshot checked at 360 px.
- Waiting for KCB: the key and method to verify `Signature`, and the meaning of their 13-digit timestamp sample. Then send the endpoint to buni@kcbgroup.com for review.

### Stage 15a.2: M-Pesa hardening and sandbox tunnel (2026-10-27)

- Owner supplied Daraja sandbox keys, shortcode and passkey, and Buni sandbox keys (kept only in the git-ignored `.env`).
- Hardening (D-173): secrets kept out of access logs; refused callbacks throttled per address; IP allow list required in production; optional ceiling on automatic credit; callback-only proxy for tunnels.
- Live sandbox: OAuth token from Daraja works with the owner's keys. Registering C2B URLs fails on Safaricom's side with 500.003.1001 "Service is currently unreachable" for every shortcode tried (174379, 600999, 600000) on both v1 and v2, twice; not yet retried after that. A Safaricom-shaped confirmation sent through the public ngrok URL reached the API through the proxy and was credited and receipted; the proxy refused every other path.
- Verified: all Go packages with PostgreSQL and `-race`, including tests for throttling, the credit ceiling and log redaction.

### Stage 15a: M-Pesa C2B (2026-10-27)

- DG-02 cleared by the owner: paybill C2B; KCB Buni (bank) is 15b. Owner chose count matched payments at once, an unmatched queue, and the student number as the account number (D-169 to D-172; A-40, A-41; DQ-09 confirmed, values pending).
- Configuration only through `MPESA_*` environment variables (see `backend/.env.example`), checked at start-up.
- Migration 00018: stored callbacks (append-only), one row per M-Pesa transaction (unique code, guarded), payments without a recorder when confirmed by the provider, permission `finance.mpesa.setup`.
- API: secret-path callbacks (validation, confirmation); status, transactions, assign, refund, daily reconciliation; register URLs, sandbox simulation, reprocess. Students' Fees shows the paybill and their account number.
- Screens: M-Pesa page (setup status, waiting payments with assign or return, day reconciliation, payments received, setup for Super Admin).
- Verified: Go tests with PostgreSQL and `-race`, all packages, including a fake Daraja server for registration and simulation; 121 unit tests; 153 Playwright tests passed locally with M-Pesa enabled using fake keys. Screenshots checked at 360 px and 1280 px.
- Not verified: anything against the real Daraja sandbox or production (no credentials here).

### Stage 14b: Finance screens (2026-10-27)

- Staff: Finance (summary, find a student, who owes), Payments (queue to confirm or reject), Invoices (bill a class, drafts, issue several), Fee structures, and each student's fee account (statement, record a payment, adjust, new invoice, PDFs). Overview card and a link from the student record (D-165, D-166, D-168).
- Students: Fees page and a fees card on Home with balance and overdue amount (D-167).
- API: the dashboard now carries the fee balance and overdue amount; `GET /staff/finance/programmes`.
- Verified: Go tests with PostgreSQL and `-race` (all packages); 121 unit tests; 150 Playwright tests passed locally, including the payment flow at 360 px (M-Pesa recorded, not counted, confirmed, balance falls by the amount; rejected transfer; draft editing and voiding) and axe and overflow checks on every finance screen at both widths. Screenshots checked by eye at 360 px and 1280 px; fixed doubled "(optional)" labels, a missing space in a download link, the arrears and issued-invoice tables cut off on phones.
- Tests now use role queries and visible-only lookups: Next.js keeps the previously visited page in the DOM, hidden. Two older specs were made independent of data left by other specs (a second "Finance" link; an extra student enrolled in ICT 101).

### Stage 14a: Finance API (2026-10-27)

- Stage 14 split into 14a (API) and 14b (screens). Owner chose Finance-owned fee structures, any confirmer may confirm, cash receipted at once, and billing a whole class (D-157 to D-164; defaults A-37 to A-39).
- Migration 00017: fee structures with lines and instalment plans; invoice lines and instalments; payments, receipts and adjustments; permission `finance.payment.record`; setting `finance.payment_instructions` (empty). Triggers refuse changes to issued invoices, decided payments, receipts and adjustments, and any deletion.
- API: fee structures (optionally from the published fees); drafts, fill from structure, issue one or many (gapless `DTC/INV/YYYY/NNNNN`), void drafts; bill a class for a term; record, confirm and reject payments with Idempotency-Key (gapless receipts `DTC/RCT/YYYY/NNNNN`); adjustments; account with statement and balance; arrears; summary; receipt, invoice and statement PDFs; the student's own fees and PDFs. Enrolment now fills the first invoice from the structure when one exists.
- Dev seed: a sample fee structure for the sample ICT diploma, the test students' first invoices filled (still drafts), and one pending sample M-Pesa payment.
- Verified: Go tests against PostgreSQL with `-race`, all packages (AC-10 reconciliation over 60 random operations; AC-11 manual path; database guards; permissions; concurrency; enrolment fill; OpenAPI matches routes; migrations up, down, up). Live against the migrated dev API: issued an invoice, confirmed the sample payment, downloaded the three PDFs and checked them by eye.

### Stage 13b: Results screens (2026-10-27)

- Trainer: "Enter results" on each class; marks or outcomes per student, save, send for approval, correct with a reason after approval (D-153, D-154).
- Head of School: "Results to approve" queue and review, approve or return with a reason. Registrar: "Results to release" queue, release with confirmation; overview cards count both.
- Assessment tab in Units and terms: marks (parts, weights with a running total, pass mark, grades) or competency.
- Student record: released results, official transcripts (audited links), issue a transcript; Student requests: issue a requested transcript and close the request.
- Student portal: Results page (cards by term), provisional slip PDF, official transcripts.
- Dev seed: ICT Head of School (D-155).
- Verified: Go tests with PostgreSQL and `-race` (all packages, seed counts updated); 117 unit tests; 147 Playwright tests passed locally, including the full results flow at 360 px (AC-09 in the browser, D-156) and axe on every new screen. Screenshots checked by eye at 360 px and 1280 px; fixed a list made invalid by `role="group"`, misaligned mark fields, and a results table too wide for phones.
- Note: the sign-in limit (in memory) blocks a second full e2e run within 15 minutes from one machine; restart the local API between runs.

### Stage 13a: Results API (2026-10-26)

- Stage 13 split into 13a (API) and 13b (screens); owner chose assessment per programme with marks by default, Trainer → Head → Registrar, and a results slip plus official transcripts on request (D-149 to D-152; DQ-07 confirmed; defaults A-35, A-36).
- Migration 00016: assessment schemes, result sheets with events, marks, releases (all append-only where they record history), transcripts.
- API: programme assessment; trainer's sheet, marks entry and submit; approval queue, approve, return; release queue and release; a student's released results, provisional slip PDF and own transcripts; staff transcript issue (optionally closing a transcript request) and links.
- Verified: Go tests against PostgreSQL with `-race` (AC-09: nothing visible before release, each role only its step, a correction invisible until re-released, history cannot be edited; scope; competency and returns; slip and transcripts, including another student's request refused); integer rounding unit tests; 142 Playwright tests still pass; live sheet against the migrated dev API; PDFs rendered and checked by eye.

### Stage 12b: Student portal screens (2026-10-26)

- Student portal at `/student` (fixing the public "Student Portal" link): sign-in by number, forgot password, Home, Timetable, Units, Notices, Requests, Profile (D-146).
- Staff: Notices, Student requests (services and changes of details), class notices for trainers (D-147).
- Dev seed: registration open around today, a sample notice, passwords for all test students (D-148).
- Verified: 111 unit tests; 142 Playwright tests passed locally (twice for the new portal spec), including a student signing in by number, seeing a notice, dropping and re-registering a unit, sending a request the registrar answers, changing their phone and asking for a correction the registrar declines, a trainer's class notice, and the reset page leading back to the portal, with axe and the overflow check at 360 px and desktop. Screenshots checked by eye.

### Stage 12a: Student portal API (2026-10-26)

- Stage 12 split into 12a (API) and 12b (screens); owner chose an emailed set-up link, contacts changed directly with identity changes approved by the registrar, and notices by managers and trainers (D-140 to D-144, A-31 to A-33).
- Migration 00015: notices, service requests, profile change requests; the enrolment email carries the set-up link; two new email templates.
- API: sign-in and password reset by student number; the student's dashboard, profile, phone, contacts, change request, self-registration within the window, timetable, notices and service requests; staff notices (managers and trainers), service requests and change requests (D-141 to D-145).
- Verified: Go tests against PostgreSQL with `-race` (set-up link to sign-in by number, unknown numbers look like wrong passwords, AC-08 with two students, registration rules, change approval, notices by audience, service request rules and email); 136 Playwright tests still pass; live sign-in by student number against the migrated dev API.

### Stage 11b: Student records and academics screens (2026-10-26)

- Staff screens: Students, My teaching, Classes, Units and terms, Staff schools; overview card for trainers (D-137).
- Dev seed: sample term, ICT units, two classes, four enrolled test students, Head of School assignment; the go-live sample check covers terms and units (D-138).
- Fixed: tables with screen-reader-only text widened the page on phones (D-139).
- Verified: 109 unit tests; 136 Playwright tests passed locally, including the trainer seeing only ICT 101 and no contact details (AC-08), the Head of School seeing no students outside Medical, the registrar's record, status, contact, timetable and registration flow, and the Staff schools page, with axe and the overflow check at 360 px and desktop; Go seed and backend tests pass. Screenshots checked by eye.

### Stage 11a: Student records and academics API (2026-10-26)

- Stage 11 split into 11a (API) and 11b (screens); terms set by the registrar; Heads of School scoped to their schools; trainers get a teaching permission instead of `students.view` (owner, D-132, D-133).
- Migration 00014: terms, cohorts, units, programme units, offerings, trainer assignments, unit registrations, timetable, student guardians, student status history, school assignments; three permissions and new grants; backfill of students enrolled in Stage 10 (D-134).
- API: students (list, record, status with reason, cohort, guardians), cohorts, terms, unit catalogue, programme units, offerings with trainers, timetable and registration (by student or cohort, drops kept), a trainer's own teaching and class lists, school assignments for staff (D-135, D-136). Enrolment now files students into their intake's cohort.
- Verified: Go tests against PostgreSQL with `-race` (AC-08 for trainers, Head of School scope including no assignment, records, offerings and registration rules, enrolment cohorts, migration backfill, route walk); 128 Playwright tests still pass; live check through the site's `/api` rewrite after migrating the dev database (seeded trainer refused student records, registrar sees all).

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
