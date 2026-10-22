# Decisions

Architecture and process decisions. ADR-xx come from the brief (section 18). D-xx are made during the build. A decision is changed only by a new entry that supersedes it.

## From the brief

| ID | Decision | Reason |
|---|---|---|
| ADR-01 | Modular monolith in one monorepo | One developer, simple operations, atomic cross-stack changes. |
| ADR-02 | Go API is the only database client | Keeps rules and credentials in one place. |
| ADR-03 | Public pages use cached rendering with fallback content | The site survives a sleeping free-tier API. |
| ADR-04 | Server-side sessions in HttpOnly cookies and a frontend rewrite to the API | Immediate revocation, no cross-site cookie problems. |
| ADR-05 | Money as integer cents of KES | Exact arithmetic. |
| ADR-06 | Content managed in the staff portal, not in code | The college can run its own site. |
| ADR-07 | Idempotency keys on submissions and payments | Safe retries on poor connections. |
| ADR-08 | Launch public site and applications first, portals later | Earliest value to the college. |
| ADR-09 | School-level icons live in the UI icon set, not in the crest | Schools may change; the crest should not. |

## Made in Stage 0

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-10 | Migrations use **goose** (SQL files, up and down in one file, embedded in the binary for the `migrate` command). Never run automatically against production. | Single-file migrations are easier to review; goose embeds cleanly with `embed.FS`; supports Go migrations if ever needed. | 0 |
| D-11 | API paths are `/api/v1/...` on the Go server. Next.js rewrites `/api/:path*` to `${API_ORIGIN}/api/:path*`, so the browser sees the same path on the frontend origin. | Brief 12.1 and CLAUDE.md both satisfied with one rule; no path translation to debug. | 0 |
| D-12 | Frontend API types are generated from `backend/openapi.yaml` with `openapi-typescript`, and calls go through a small typed fetch wrapper. Generated file is committed and checked for drift in CI. | Contract-first, no runtime codegen dependency on the backend. | 0 (implemented 1 to 5) |
| D-13 | `docker-compose.yml` and `Dockerfile` live in `backend/`, as in the CLAUDE.md repo layout. Root `Makefile` delegates to `backend/Makefile` and `frontend` npm scripts. | Follows the prescribed layout; the frontend has no Dockerfile. | 0 |
| D-14 | npm is the frontend package manager, with a committed `package-lock.json`. | Ships with Node; one fewer tool for the client's future maintainer. | 0 |
| D-15 | (Confirmed by owner 2026-10-22.) Brief safeguard "a role in use cannot be deleted" (brief 11) is added to Stage 4 alongside the two protections in CLAUDE.md. | Brief wins where it adds to the prompt. | 0 |
| D-16 | `CLAUDE.md` is the canonical master prompt. `docs/claude/MASTER_PROMPT.md` is a copy; if they differ, `CLAUDE.md` wins and I will flag the drift. | Avoids two sources of truth (R-13). | 0 |
| D-17 | Seed and fixture data use names like "Test Student 001", emails at `example.invalid`, and phone numbers in the form `+254 000 000 0NN` (not a routable Kenyan number); programmes carry "(Sample)" in the title. Updated in Stage 3: the earlier `+254 700 …` form could belong to a real subscriber. | Reserved, obviously fake values (NN-06). | 0 |
| D-18 | (Confirmed by owner 2026-10-22.) Password reset is by email in Stage 4. SMS reset (brief 5.1) is added only after DG-03 clears. | SMS provider is a decision gate. | 0 |
| D-19 | (Confirmed by owner 2026-10-22.) Upload scanning sits behind a scanner interface. Stage 9 ships a ClamAV adapter for Compose if it fits; otherwise uploads are marked "not scanned" and the report says so. | Brief 8 requires scanning "if available" (brief 13); we never claim scanning that is not running (R-15). | 0 |

## Made in Stage 1

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-20 | Local S3 storage uses `cgr.dev/chainguard/minio` (MinIO built from source by Chainguard), pinned by digest. | MinIO stopped publishing official images: `minio/minio` is gone from Docker Hub and Quay (checked 2026-10-22). Chainguard's free tier only offers `:latest`, so a digest pin keeps builds reproducible. MinIO is AGPLv3; we run it unmodified as a local dev tool and never ship it. Production storage stays Supabase Storage. | 1 |
| D-21 | Default local host ports are API 8180 and PostgreSQL 5442 (not 8080 and 5432). Inside Compose the API still listens on 8080. | Other projects on the developer machine already hold 8080 and 5432. A unique port also stops the frontend proxying to someone else's API by accident. | 1 |
| D-22 | `/api/v1/health` is liveness (process up, no dependency checks). `/api/v1/ready` is readiness (database ping, 2 s timeout, 503 with no error detail when down; detail goes to logs). | A slow or sleeping database must not get the API restarted, and probe responses must not leak infrastructure detail. | 1 |
| D-23 | Every API error uses one JSON shape: `{"error": {"code", "message", "request_id"}}`. Every response carries `X-Request-ID` (a well-formed caller value is reused, otherwise generated). | Plain-language messages for users, stable codes for the frontend, and a correlation ID for support. | 1 |
| D-24 | Go integration tests use `TEST_DATABASE_URL` (a separate `dakota_test` database). They skip locally without it but fail when `REQUIRE_DB_TESTS=1`, which CI and `make test-db` set. | Real PostgreSQL in tests, and a missing database is never reported as a pass. | 1 |
| D-25 | The scaffold home page is marked noindex. Stage 6 replaces it with real SEO metadata. | A placeholder must never be indexed. | 1 |

## Made in Stage 2

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-26 | Icons come from `lucide-react` (ISC), re-exported through `components/icons.tsx` only. School icons: stethoscope (medical), chef hat (hospitality), monitor (ICT), briefcase (business), waves (pool). | One consistent, tree-shaken set; school icons live in the UI set, not the crest (ADR-09). | 2 |
| D-27 | Fonts load with `next/font/local` from the fontsource packages' latin woff2 files (Newsreader and Inter variable, IBM Plex Mono 400/500). Plex Mono is not preloaded. | Self-hosted, preloaded and metric-matched fallbacks without any Google Fonts request. | 2 |
| D-28 | Tokens live in `app/globals.css` (`@theme`) and in `lib/contrast.ts`. A unit test checks every text/background pair the UI uses against WCAG AA and checks the two files agree. Added tokens: `ink-muted` #4A5565, `warning` #8A4B00, gold hover #B8931F, danger hover #8F1C13 (all measured). | Contrast is measured, not assumed; tokens cannot drift silently. | 2 |
| D-29 | Focus ring is 3 px royal on light surfaces and gold on navy (`.on-dark`). | Gold on ivory is 2.4:1 and fails the 3:1 non-text minimum. | 2 |
| D-30 | Accordion uses native `<details>`; Dialog uses native `<dialog>`; Tabs, Toast and the header menu are the only client components. | Accessible by default and the least JavaScript. | 2 |
| D-31 | `/design` is served in development, and in production builds only when `ENABLE_DESIGN_PAGE=1` (CI sets it for axe). Otherwise it returns 404. Always noindex. | Dev-only page that CI can still test. | 2 |
| D-32 | Public pages live in the `app/(site)` route group, which renders the shell (skip link, announcement bar, utility bar, header, footer). Portals will get their own layouts. | Keeps portal layouts separate from the marketing shell. | 2 |

## Made in Stage 3

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-33 | Go moves from 1.25 to **1.26** (`go.mod`, Dockerfile `golang:1.26.10-alpine`, CI `1.26.x`). | goose 3.28+ and `x/crypto` 0.56+ need Go 1.26, and Go 1.25 is no longer supported (Go supports the two newest releases: 1.27 and 1.26). Pinning old dependencies to stay on an unsupported Go would trade security fixes for nothing. | 3 |
| D-34 | An `accreditations` table (body, reference, evidence note, verification date) is added beyond brief section 10. | The About page needs "accreditation and registration" (brief 6.1) with the same evidence rule as statistics (NN-01). | 3 |
| D-35 | Two permission keys are added beyond brief 11: `enquiries.view` and `enquiries.manage`. | The enquiry inbox (Stage 8) needs its own permissions; reusing admissions keys would over-grant. | 3 |
| D-36 | Every content table has `is_sample`. The seed sets it on everything it creates; `CountSampleRows` is the launch check (AC-17). | Sample data can be found and removed reliably before launch. | 3 |
| D-37 | Honesty and integrity rules are also database constraints: statistics and accreditations need a verification date and evidence to be published; gallery items need consent; audit log, status history and consents are append-only (trigger); one live application per applicant, programme and intake; rejection needs a reason; money columns are `bigint` cents with KES only. | NN-01, NN-03, NN-05, NN-08 and AC-04 hold even if application code has a bug. | 3 |
| D-38 | Primary keys are UUIDs (`gen_random_uuid()`), except append-only logs (`bigint` identity). Human references (application, student, receipt numbers) come from `document_counters`, gapless per year. | IDs in URLs do not reveal counts; references stay readable and gapless (A-06). | 3 |
| D-39 | The seed refuses unless `APP_ENV` is development or test, and also refuses database hosts on Supabase, Render, Neon and AWS RDS. Test accounts use `SEED_PASSWORD` (dev only, at least 12 characters). | Two independent guards against sample data reaching a real database (NN-06, AC-17). | 3 |
| D-40 | `migrate` changes a production database only with `-confirm-production`, and never runs `reset` there. The API never migrates on start-up. | CLAUDE.md: migrations are never run automatically against production. | 3 |
| D-41 | Argon2id parameters: 19 MiB, 2 passes, 1 lane, 16-byte salt, 32-byte key, PHC string format. Minimum password length 12. | OWASP minimum, sized for small hosting instances; parameters are stored in each hash so they can be raised later. | 3 |
| D-42 | Kiswahili: no language columns now (DQ-13). Adding it later means a translations table keyed by entity and field. | Avoids empty columns while keeping the option open. | 3 |

