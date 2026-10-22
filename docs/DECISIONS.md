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

## Made in Stage 4

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-43 | Sessions: opaque 256-bit random token in an HttpOnly, SameSite=Lax cookie (`__Host-dtc_session`, Secure, when `SESSION_COOKIE_SECURE=true`; `dtc_session` in local http development). Only its SHA-256 is stored. Absolute lifetime 12 h, idle timeout 30 min, `last_seen_at` refreshed at most once a minute. Permissions are read from the database on every request. | Immediate revocation (ADR-04), role changes apply at once (brief 12.1), a database leak does not expose live sessions. | 4 |
| D-44 | CSRF: every state-changing request with a session needs `X-CSRF-Token`, an HMAC derived from the session token (nothing extra stored; the session endpoint returns it after a reload). In addition, any unsafe request whose `Origin` is not in `ALLOWED_ORIGINS`, or whose `Sec-Fetch-Site` is `cross-site`, is refused, which also covers login CSRF. | Defence in depth on top of SameSite=Lax. | 4 |
| D-45 | Lockout and rate limits: 5 wrong passwords in a row lock the account for 15 min in the database. In memory: 5 sign-in attempts per email and 100 per client address per 15 min; 3 reset requests per email and 30 per address per hour. Unknown emails get the same answer, the same limits and the same Argon2id time (a dummy hash). | Brute-force protection without revealing which emails have accounts. | 4 |
| D-46 | One API instance is assumed (in-memory limits are per instance; the database lockout is durable). `X-Forwarded-For` is not trusted: tested 2026-10-22, the Next.js rewrite passes a visitor-supplied value through unchanged and adds none of its own. The API therefore sees the frontend server's address, which is why the per-address limit is generous (R-23). | Spoofed headers must never bypass limits. | 4 |
| D-47 | Password reset: single-use link valid 1 hour; completing a reset spends every outstanding link and signs the user out everywhere. The email is sent in the background so response time does not reveal whether the account exists. New staff accounts have no password and get this email to set one. | Brief 5.1 and 13; no admin ever knows a staff password. | 4 |
| D-48 | Access protections (AC-12, brief 11): every user and role change runs under a row lock on the super_admin role and is refused if it takes the active Super Admin count from one or more to zero. Only a holder of system.admin can give, remove or disable someone with a role carrying it. system.admin can never be put into a role through the API, even by a Super Admin, and the Super Admin role's permissions cannot be edited. Starter roles and roles in use cannot be deleted. Nobody can disable their own account. | Prevents lock-out and privilege escalation, including two admins removing each other at once (tested). | 4 |
| D-49 | Every route is registered with its protection (public, or session plus optional permission). Tests walk the route table (AC-07) and compare it with `openapi.yaml`, so a new route cannot ship unprotected or undocumented. | AC-07 holds automatically as routes are added. | 4 |
| D-50 | `SMTP_FROM` is a bare address; the display name "Dakota Training College" is added in code. | Values with spaces or angle brackets break `.env` parsing in some tools. | 4 |
| D-51 | Two-step login (TOTP) for Super Admin, finance and registrar accounts stays in Stage 17, as the brief places it. | Default for the Stage 4 question (A-15). | 4 |

## Made in Stage 5a

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-52 | Stage 5 is split: 5a content API (this stage), 5b staff portal screens. | Each half fits one review sitting (PLAN.md). | 5a |
| D-53 | The Stage 5 done-when ("a Content Editor can publish a programme") is read with DQ-14: the Content Editor drafts, a Registrar or Principal publishes, no developer involved. Starter grants are unchanged (editor: content.read and content.write; registrar and principal: content.publish). | The brief and the owner's DQ-14 answer win over the prompt's wording. | 5a |
| D-54 | Editing a published item saves a **pending version** (`content_pending`, one JSON document per item); the live tables keep what the site shows until someone with content.publish publishes again. Drafts are edited in place. Unpublish and archive apply any pending edit first, so no work is lost. Only drafts that were never published can be deleted; published content is archived. | Approval cannot be bypassed by editing something already live (DQ-14), and history is kept. | 5a |
| D-55 | A programme is one document: programme fields plus entry requirements, intakes and fees. Fee or intake edits on a published programme therefore also wait for approval. Intakes keep their IDs across edits; removing an intake that applications use is refused. | Fees are the most sensitive public claim; applications must not lose their intake. | 5a |
| D-56 | Publishing checks completeness: a programme needs a summary, what you will learn, a duration, at least one entry requirement and at least one fee line, and a published school; statistics and accreditations need a verification date (not in the future) and evidence; gallery photos need confirmed consent; news needs a summary and body. Problems come back as a list of fields with plain instructions (422). | The five questions on every programme page (CLAUDE.md), NN-01 and NN-08 enforced before anything goes live. | 5a |
| D-57 | Saves and publishes carry `base_version`; a mismatch returns 409 instead of overwriting someone else's edit. Documents with unknown fields are refused. | Two editors cannot silently overwrite each other; typos in a client cannot drop data. | 5a |
| D-58 | FAQ categories and site settings have no draft stage; changing them needs content.publish. | They are structure and site-wide facts (contact details), not drafts. | 5a |
| D-59 | Image uploads: JPG, PNG or WebP up to 10 MB and 10,000 px a side (40 MP), checked by content not file name; JPEG orientation applied; re-encoded (JPEG quality 82, PNG kept as PNG) at widths 480, 960 and 1600, never wider than the original; all metadata, including GPS, removed. Stored in the public bucket under `images/<uuid>/<width>.<ext>` with a one-year immutable cache header. WebP output is not produced (Go has no pure WebP encoder); revisit in Stage 18. | Privacy (no location leaks), safety (no decompression bombs), and light pages on mobile data. | 5a |
| D-60 | Storage settings `S3_*`; with no `S3_ENDPOINT` the API runs and uploads answer 503. In development and test the API creates the buckets and makes the public one readable; hosted buckets are set up by hand (Stage 19). | Works locally with no manual steps; never changes hosted storage automatically. | 5a |

## Made in Stage 5b

| ID | Decision | Reason | Stage |
|---|---|---|---|
| D-61 | The staff portal (`/staff`, noindex) renders in the browser after a session check, inside one Suspense boundary. The browser calls the API through a typed client (`openapi-fetch` on the generated types) that adds the CSRF header on every state-changing call. Signed-out visitors go to `/staff/login?next=…`; only `/staff` paths are followed after sign-in. | Session-dependent screens gain nothing from prerendering; the API enforces permissions regardless (AC-07). Typed calls catch contract drift at build time. | 5b |
| D-62 | One description per content type (`lib/staff/kinds.ts`) drives the list, the form and the preview for all ten types. The preview of a programme shows the five answers applicants look for and flags anything missing. | One set of screens to maintain; editors see gaps before asking for approval. | 5b |
| D-63 | Navigation and buttons follow permissions: editors do not see Publish or the approval queue; publishers do; people without content.write see read-only forms. | Plain screens per role; the API is still the guard. | 5b |
| D-64 | Money is typed in shillings ("45,000" or "1,250.50") and converted to cents with string arithmetic, never floating point; date-times are entered and shown in Nairobi time (UTC+3). | ADR-05, A-01. | 5b |
| D-65 | Sign-in forms use `method="post"` and keep the submit button disabled until the page is interactive. | Found in testing: a fast click before hydration sent the form as GET, which would put the password in the URL. Covered by an end-to-end test. | 5b |
| D-66 | In OpenAPI, the CSRF token is a security scheme (`csrf`, header `X-CSRF-Token`) required with the session on the 17 state-changing operations, not a request parameter. | Matches how it works and keeps generated client types simple. | 5b |
| D-67 | CI runs Playwright in a separate job against the real stack: PostgreSQL, migrations, seed (APP_ENV=test), the API binary, and the production build of the site. MinIO is not started in CI, so the image upload is covered by Go tests and local runs. | The staff flows only mean something against the real API. | 5b |

