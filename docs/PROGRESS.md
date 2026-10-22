# Progress

Updated at the end of every stage. Newest stage at the top of the log.

## Current state

- **Current stage:** 5b, Staff portal screens. Done, awaiting review.
- **Next stage:** 6, Public site part 1 (home, schools, course finder, programme pages, about, contact).
- **Branch:** `stage-05b-staff-portal` (from `main`)

## Stage status

| Stage | Title | Status |
|---|---|---|
| 0 | Orientation and plan | Merged (#1) |
| 1 | Repo scaffold and tooling | Merged (#1) |
| 2 | Design system | Merged (#9, #10) |
| 3 | Database and migrations | Merged (#11) |
| 4 | Auth and RBAC | Merged (#12) |
| 5a | Content API | Merged (#14) |
| 5b | Staff content screens | Done, awaiting review |
| 6 to 21 | See `CLAUDE.md` section 3 | Not started |

## Open questions for the client or owner

Stage 0 questions were answered by the owner on 2026-10-22 (see `ASSUMPTIONS.md`). Still open:

| Topic | IDs | Needed by |
|---|---|---|
| Real contact, registration and programme content | DQ-01, DQ-03, DQ-04, DQ-05 | Stage 20 (dummy values in dev seed until then) |

DQ-02 (Pool Management content is editable by the college) and DQ-13 (English only for now) were closed on 2026-10-22.

## Log

### Stage 5b: Staff portal screens (2026-10-22)

- `/staff` portal: sign in (split layout), forgot and reset password, overview, approval queue, list and editor screens for all ten content types with preview, image upload, site settings. Navigation and actions follow permissions.
- Typed API client from the OpenAPI types; money and Nairobi date helpers.
- CI: new end-to-end job with the real API and database.
- Verified: editor drafts a programme in the browser, cannot publish; registrar publishes from the approval queue; an edit of the live programme waits for approval; axe clean on each screen; credentials never appear in a URL.

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
