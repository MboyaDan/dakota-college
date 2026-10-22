# Progress

Updated at the end of every stage. Newest stage at the top of the log.

## Current state

- **Current stage:** 2, Design system. Done, awaiting review.
- **Next stage:** 3, Database and migrations.
- **Branch:** `stage-02-design-system` (stacked on `stage-01-scaffold`, PR #1 not merged yet)

## Stage status

| Stage | Title | Status |
|---|---|---|
| 0 | Orientation and plan | Done, approved by owner 2026-10-22; PR not merged |
| 1 | Repo scaffold and tooling | Done, PR #1 open |
| 2 | Design system | Done, awaiting review |
| 3 to 21 | See `CLAUDE.md` section 3 | Not started |

## Open questions for the client or owner

Stage 0 questions were answered by the owner on 2026-10-22 (see `ASSUMPTIONS.md`). Still open:

| Topic | IDs | Needed by |
|---|---|---|
| Real contact, registration and programme content | DQ-01, DQ-03, DQ-04, DQ-05 | Stage 20 (dummy values in dev seed until then) |

DQ-02 (Pool Management content is editable by the college) and DQ-13 (English only for now) were closed on 2026-10-22.

## Log

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
