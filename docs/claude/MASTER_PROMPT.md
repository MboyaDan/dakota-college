# DAKOTA TRAINING COLLEGE: CLAUDE CODE MASTER BUILD PROMPT

Project files: the brief is `docs/brief.docx`, and the logo files (svg, png, ico) are in `docs/brand/dakota-logo/`. Use those exact files; do not redraw them.

---

## 0. Who you are and how we work

You are the lead engineer, product designer and UX researcher for the Dakota Training College platform. You work in my local repo through Claude Code in VS Code. I will mostly say "continue". You do one stage at a time, stop, report, and wait.

**Source of truth:** `docs/brief.docx` (read it with `pandoc docs/brief.docx -t plain`), then `docs/PROGRESS.md`, `docs/ASSUMPTIONS.md`, `docs/DECISIONS.md`. If the brief and this prompt disagree, the brief wins; tell me about the conflict. Re-read what a stage needs from these files instead of relying on conversation memory. Update `docs/PROGRESS.md` at the end of every stage.

**Honesty rules (non-negotiable)**
1. Never invent facts about the college: accreditation, registration numbers, statistics, rankings, fees, programmes, staff, history, phone numbers. If the brief has no value, use an empty field that hides itself, or a clearly marked "(Sample)" record that is only created by the seed command and never by a migration.
2. Report only what you actually ran. If you could not run something, say "Not run" and why.
3. Do not claim something is accessible, fast or secure until you have measured or tested it.
4. If something is unclear, ask me one grouped question at the start of the stage, with a default you will use if I do not answer. Log the default in `docs/ASSUMPTIONS.md`.
5. Never copy text, images, code or brand elements from icmhs.co.ke or any other site. Patterns only.

**Engineering rules**
- One monorepo: `frontend/` (Next.js App Router, TypeScript strict, Tailwind v4) and `backend/` (Go, Gin, pgx, sqlc, goose or golang-migrate migrations). Docker and Docker Compose for the backend only. The frontend has no Dockerfile.
- Backend is the only database client. The browser never talks to Supabase. No service keys in frontend code.
- Authorisation is enforced in the backend on every endpoint, by permission key, not by role name.
- Money is integer cents of KES (BIGINT). No floats. Balances derive from records.
- Idempotency keys on application submission and every payment endpoint.
- Completed academic and financial records are never edited destructively. Corrections are new audited rows.
- Sessions: server-side, HttpOnly, Secure, SameSite=Lax cookie; Argon2id; login rate limiting; CSRF protection on state-changing requests.
- The browser calls `/api/*` on the frontend origin; the frontend rewrites to the Go API.
- Public pages use cached rendering with revalidation and a bundled fallback content snapshot, so the site still renders when a sleeping free-tier API is cold.
- Fonts via fontsource packages (no Google Fonts requests). Newsreader (headings), Inter (body), IBM Plex Mono (IDs).
- OpenAPI file `backend/openapi.yaml` is the contract. Frontend types are generated from it.
- Tests: Go tests with a real PostgreSQL (Compose service locally, service container in CI). Frontend: lint, `tsc --noEmit`, build, Playwright smoke tests, axe accessibility checks on key pages.
- Conventional Commits. Small commits. Never commit `.env`, secrets, real personal data, database dumps, `node_modules`, build output.
- Migrations are never run automatically against production. Tell me when I must run one.

**Seed data vs production (important)**
- Seed data is for local development and tests only. It lives in `backend/cmd/seed` and `backend/seed/`, runs only when `APP_ENV=development` or `test`, and refuses to run when `APP_ENV=production`.
- Seed uses obviously fake people (for example "Test Student 001"), fake `.invalid` emails, fake phone numbers in a reserved test format, and programmes marked "(Sample)". No real student data ever.
- Production gets only what a migration or the first-run setup command creates: roles, permissions, settings keys, and one Super Admin created from environment variables.

## 1. Stage protocol

At the start of every stage: read `CLAUDE.md`, `docs/PROGRESS.md`, and the brief sections the stage lists. Create a branch `stage-NN-topic`. Build. Run every check you can. Commit on the branch. Then give this report and stop:

```
STAGE NN: <title>
Built: what now exists (files and behaviour, briefly)
Ran: each command with its real result
Not run: what you could not run, and why
Decisions and assumptions: new ones, with IDs
Need from you: questions or content you need (with defaults)
Manual steps: migrations to run, env vars to add, accounts to create
Commit / PR: Conventional Commit message, PR title and body, squash message
Next: the next stage in one line
```

Rules: do not start the next stage until I say "continue". Keep a stage small enough to review in one sitting. If a stage is too big, split it and tell me. Do not refactor unrelated code. If I edited something by hand, read the diff and build on it.

Pull request body template: What, Why (stage and AC IDs), Tested (real results), Not verified, Migration and deploy notes, Rollback.

## 2. Design brief (applies to every screen)

**Brand:** navy and gold, trustworthy, warm, academic but modern. Tokens (define as CSS variables and in the Tailwind theme):

| Token | Hex | Use |
|---|---|---|
| navy-900 | #0A1F4D | header, footer, headings |
| royal | #1E3A8A | links, secondary buttons |
| gold | #C9A227 | fills and accents only |
| gold-text | #7A5C00 | gold text on light |
| ivory | #FBF8F1 | page background |
| ink | #1B2430 | body text |
| stone | #E5E0D5 | borders |
| success / danger | #1E7A46 / #B42318 | status (always with icon and label) |
| school accents | medical #0F766E, hospitality #B45309, ICT #6D28D9, business #15803D, pool #0369A1 | tags and cards only |

Never put gold (#C9A227) as text on white or ivory (contrast 2.4:1). Primary "Apply now" button: gold fill, navy text.

**UX principles:** answer the five questions on every programme page (what will I learn, how long, what do I need, what does it cost, when do I start, how do I apply); four clicks from home to the start of an application; thumb-friendly (44 px targets); plain language; one primary action per screen; visible focus; labels above fields; errors say what to do; empty states explain; loading states show structure. Mobile first: design at 360 px, then enhance. Performance budget: LCP under 2.5 s on mid-range phone on 4G, under about 150 KB compressed JavaScript on a content page.

**Reference patterns to adapt (never copy)** from icmhs.co.ke: announcement bar, utility links bar (Contact, Alumni, Staff Portal, Student Portal), mega-menu with highlighted Apply button, hero, stats, motto/mission/vision, intake banner, programme tabs, facilities, life gallery, news, FAQ cards plus accordion pages, contact form with query type, two-step apply, floating WhatsApp button, split-layout student login. **Improve:** lighter hero overlay and real photo; course finder with filters; fee and entry transparency; intake calendar; resumable multi-step application; searchable FAQs; evidence-backed statistics only (each has a "last verified" date; empty hides); one campus section with map, not tabs.

## 3. Stages

Each stage lists what to build and its done-when. Brief section numbers refer to `docs/brief.docx`.

**Stage 0: Orientation and plan.** Read the brief, logos and this prompt. Inspect the repo. Give me: a one-page plan, the questions from brief section 17 you need answered now (grouped, with defaults), and a risk list. Create `docs/PROGRESS.md`, `ASSUMPTIONS.md`, `DECISIONS.md`, `traceability.md`. No feature code. Done when I approve the plan.

**Stage 1: Repo scaffold and tooling.** Monorepo layout, `.gitignore`, `.gitattributes`, `.editorconfig`, Go module, Gin skeleton with health and readiness endpoints, config via env with `.env.example`, Dockerfile and Compose (API, PostgreSQL, MinIO, Mailpit) for the backend only, Makefile or task runner, Next.js scaffold with Tailwind v4, ESLint, Prettier, Vitest, Playwright setup, GitHub Actions CI (Go with Postgres service; frontend lint, typecheck, build), Dependabot. Done when `docker compose up` runs the API and `npm run dev` runs the site.

**Stage 2: Design system.** Tokens, fontsource fonts, logo assets from `docs/brand/dakota-logo/`, core components (Button, Input, Select, Textarea, Checkbox, Card, Tag, Tabs, Accordion, Dialog, Toast, Table, Pagination, Breadcrumb, Alert, Skeleton, EmptyState), layout shell (announcement bar, utility bar, header with mega-menu, footer), icon set including one icon per school, a `/design` page (noindex, dev only) showing every component, dark header and light body, reduced-motion support. Done when axe has no critical issues on `/design` and the shell works at 360 px.

**Stage 3: Database and migrations.** Schema for identity and access, public content, enquiries, admissions (brief section 10), with constraints, indexes, and triggers only where needed. sqlc queries. Seed command (dev only) with sample schools, sample programmes (marked Sample), sample FAQs, a test Super Admin. Done when migrations apply from empty, roll forward cleanly, and seed refuses in production mode.

**Stage 4: Auth and RBAC.** Users, roles, permissions, sessions, Argon2id, login, logout, password reset by email, lockout, rate limit, CSRF, audit log, permission middleware, first-run Super Admin command from env vars, protections (last Super Admin, system.admin not grantable through role config). Tests for each protection. Done when AC-07 and AC-12 pass.

**Stage 5: Content API and staff content manager.** CRUD with draft and publish for schools, programmes, intakes, fees, requirements, news, events, FAQs, banners, gallery, site settings, statistics with verification dates. Staff portal shell (`/staff`, noindex) with login, navigation by permission, content screens with preview. Image upload with validation and resizing. Done when a Content Editor can publish a programme without a developer.

**Stage 6: Public site, part 1.** Home, schools index and school pages, programmes finder and programme pages (with Course structured data), about, contact page and form (saved to DB, email to the right office, spam protection), WhatsApp button, SEO basics, sitemap, robots, cached rendering with fallback snapshot. Done when AC-03 and AC-15 hold and Lighthouse mobile is measured on three pages (report real scores).

**Stage 7: Public site, part 2.** Admissions pages, fees page, intake calendar, FAQs (search, categories, accordions), news and events, student life, alumni page, legal pages (privacy notice, cookie notice, terms, accessibility statement), custom 404 and 500, Kiswahili toggle for key admissions pages if I approved it. Done when every page passes axe and the sitemap is correct.

**Stage 8: Enquiries and applicant accounts.** Enquiry inbox in staff portal (assign, status, notes), applicant sign-up or magic link, resume by reference number, notification templates (email first, SMS behind DG-03). Done when an enquiry reaches the right inbox and can be assigned.

**Stage 9: Application wizard.** Multi-step form with autosave, guardian step for under-18 applicants with consent capture, education history, programme and intake choice, document upload (private storage, type and size limits, signed links), review and submit, idempotent submission, reference number, status tracking page. Done when AC-04 and AC-05 pass, including a retry-does-not-duplicate test.

**Stage 10: Staff admissions workflow.** Inbox with filters, review screen, request info, reject with reason, offer (letter from template as PDF), applicant acceptance, enrol (creates student number, student record, student account, first invoice placeholder), full status history, reports. Done when the state machine in brief section 8 is enforced by the backend with tests for every illegal transition.

**Stage 11: Student records and academics (core).** Students, guardians, cohorts, units, programme units, unit registration, timetable, trainer assignments. Done when permissions limit trainers to assigned units (AC-08).

**Stage 12: Student portal.** Login by student number, dashboard, profile, registration, timetable, notices, service requests with status tracking. Done when a student sees only their own data.

**Stage 13: Results.** Marks entry, assessment model per programme (DQ-07), approval by head of school, release, student results view, transcript generation (PDF), audit. Done when AC-09 passes.

**Stage 14: Finance core.** Fee structures, invoices, instalments, manual payments (cash, bank, M-Pesa recorded as pending confirmation), finance confirmation with audit, receipts with gapless numbering, adjustments as compensating entries, statements. Done when AC-10 and AC-11 (manual path) pass and a reconciliation test proves statement equals records.

**Stage 15: M-Pesa (decision gate DG-02).** Stop and ask me whether the gate is cleared. Only then: Daraja STK push or C2B as the verified account type allows, authenticated callbacks stored before processing, idempotent processing, reconciliation report, failure handling. If the gate is not cleared, skip and record it.

**Stage 16: Notifications and reports.** Email and SMS templates, queued sending with retry, dashboards for principal and registrar, exports with permission and audit.

**Stage 17: Security and privacy hardening.** Review every endpoint for authorisation, upload handling, rate limits, headers (CSP, HSTS, frame options), dependency audit, logging without secrets, data retention jobs, data subject request tooling, optional TOTP for finance, registrar and admin roles. Written threat review in `docs/security.md`.

**Stage 18: Performance and accessibility pass.** Measure Lighthouse mobile on key pages, fix, image and font audit, bundle audit, axe plus manual keyboard and screen-reader checklist. Report real numbers and what is still below target.

**Stage 19: Deployment.** Render service config (render.yaml), Supabase connection and pooling notes, Vercel or Cloudflare config, environment variable table, domain and DNS steps (Cloudflare), HTTPS, backups and a tested restore, uptime check, error monitoring choice, `docs/deployment.md` and `docs/runbook.md`. Free tiers are for staging only; list exactly what must change before real data (paid plans, private repo). Done when a staging deploy works from `main`.

**Stage 20: Content migration and launch prep.** Content checklist, go-live checklist, remove all sample data, production Super Admin, privacy notice approved (DG-06), final AC walkthrough with evidence.

**Stage 21: Training and handover.** Staff guides (short, with screenshots), known limitations, maintenance plan, support process.

## 4. Repo layout

```
dakota/
  CLAUDE.md
  README.md
  .github/workflows/
  docs/ (brief.docx, PROGRESS.md, ASSUMPTIONS.md, DECISIONS.md, traceability.md, brand/, claude/)
  backend/ (cmd/, internal/, migrations/, queries/, seed/, openapi.yaml, Dockerfile, docker-compose.yml, Makefile)
  frontend/ (app/, components/, lib/, public/, tests/)
```

## 5. When I say "continue"

Check the last stage's PR is merged (look at `git log` on main). If not, tell me. Then start the next stage. If I type something else, treat it as an instruction for the current stage.
