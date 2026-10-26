# Dakota Training College platform: plan

Stage 0 output. Source: `docs/brief.docx` v1.0 (22 October 2026) and `CLAUDE.md`. One page by design; detail lives in the stage list in `CLAUDE.md`.

## What we are building

One monorepo, one modular monolith:

- `frontend/`: Next.js App Router, TypeScript strict, Tailwind v4. Public site (cached rendering with a bundled fallback snapshot), `/student` and `/staff` portals (noindex). The browser only calls `/api/*` on the frontend origin; Next.js rewrites to the Go API.
- `backend/`: Go, Gin, pgx, sqlc, goose migrations, `openapi.yaml` as the contract. The only database client. Server-side sessions, Argon2id, CSRF, permission-key authorisation on every endpoint. Docker Compose locally (API, PostgreSQL, MinIO, Mailpit).
- Production (later, gate DG-04): Supabase Pro (PostgreSQL and storage), Render paid instance, Cloudflare or Vercel Pro for the frontend, Cloudflare DNS.

## Delivery order (milestones from brief section 15)

| Milestone | Stages | Outcome |
|---|---|---|
| M0 Discovery | 0 | Approved plan, open questions logged |
| M1 Public site and applications | 1 to 10 | College can publish content and receive applications |
| M2 Portals | 11 to 13 | Students and trainers use the portals |
| M3 Finance | 14 to 15 | Fees online; M-Pesa only if DG-02 is cleared |
| M4 Hardening and go-live | 16 to 21 | Production launch and handover |

M1 is the launchable slice. Launch is gated on client content (R-01), the privacy notice (AC-13, DG-06) and paid hosting (A-05), not on code alone.

## How each stage runs

Branch `stage-NN-topic`, build, run every check available, commit, report, stop. `docs/PROGRESS.md` updated every stage; `docs/traceability.md` updated whenever an AC gets evidence.

## Stages likely to need splitting

These look bigger than one review sitting. I will propose a split when I reach them:

- Stage 5 (content API and staff content manager): split into 5a API and 5b staff screens.
- Stage 9 (application wizard): split into 9a wizard and autosave, 9b documents and submission.
- Stage 10 (staff admissions): split into 10a API (state machine, decisions, offer PDF, enrolment, report) and 10b screens (owner, 2026-10-26, D-119).
- Stage 11 (student records and academics): split into 11a API and 11b screens (owner, 2026-10-26, D-132).
- Stage 14 (finance core): split into 14a invoices and payments, 14b receipts, adjustments, statements.

## Risks (brief R-01 to R-09 apply; new ones found in Stage 0)

| ID | Risk | Mitigation |
|---|---|---|
| R-10 | GitHub repo `MboyaDan/dakota-college` is **public**. The brief, plans and later code are readable by anyone. | Fine for scaffold work with no secrets. Make it private before any real data, client content or staging credentials exist (brief 12.2, A-05). Secret scanning in CI from Stage 1. |
| R-11 | `pandoc` is not installed. I read the brief with macOS `textutil`, which flattens tables. | I checked the tables by hand. Install with `brew install pandoc` so later stages use the documented command. |
| R-12 | Docker daemon was not running during Stage 0. Stage 1 done-when needs `docker compose up`. | Start Docker Desktop before Stage 1. |
| R-13 | `CLAUDE.md` and `docs/claude/MASTER_PROMPT.md` are identical copies and can drift. | Treat `CLAUDE.md` as canonical (D-16). |
| R-14 | Supabase connection pooler in transaction mode breaks pgx prepared statements. | Use the session pooler or pgx simple protocol for pooled connections; test in Stage 19. |
| R-15 | Upload virus scanning (brief 8, 13) needs ClamAV, which is memory hungry on free tiers. | Brief says "if available". Pluggable scanner interface in Stage 9; decide hosting in DG-04. Until a scanner runs, files stay unscanned and the report says so. |
| R-16 | Gapless student and receipt numbers break under concurrent writes if done naively. | `document_counters` row locked inside the same transaction (Stages 10, 14), with a concurrency test. |
| R-17 | AC-02 asks Lighthouse 90+ in all four categories; free-tier cold starts hurt real-world performance. | Cached pages plus fallback snapshot (ADR-03). Measure in Stage 6, fix in Stage 18, report real numbers. |
| R-18 | Many applicants are minors; seed or test fixtures could leak realistic personal data. | Seed uses "Test Student 001", `.invalid` emails, reserved test phone numbers; seed refuses in production (NN-06). |
| R-19 | Library churn (Next.js, Tailwind v4, Gin, sqlc) between stages. | Pin exact versions, lockfiles committed, Dependabot from Stage 1. |
| R-20 | MinIO no longer publishes official images; the Chainguard build could change terms or disappear. | Digest pin (D-20). Storage code in Stage 9 talks plain S3, so Garage or SeaweedFS can replace MinIO locally without code changes. |
| R-21 | `npm audit` reports 5 high-severity issues in the dev-only lint chain (`eslint-config-next` pulls `fast-glob` and `micromatch`). Production dependencies: 0. | Not shipped to browsers or servers. Dependabot watches for a fixed `eslint-config-next`; recheck in Stage 17. |
| R-22 | JavaScript budget: the placeholder home page ships 182 KB of JS (gzip -9, measured 2026-10-22); about 165 KB of that is the Next.js 16 and React 19 runtime, ours is about 10 KB. Budget is about 150 KB. | Measure with Brotli on staging. Stage 18 evaluates options (for example turning off `partialPrefetching`, or trimming client components). Reported as below target until then. |
| R-24 | The tagline logo files read "Turning Potential into Progression"; the owner's tagline is "Turning Potential into Progress". | Resolved for SVG (D-86): "Progress" tagline SVGs derived from the original outlines and used on the site. PNG exports of the tagline logos still read "Progression"; regenerate them from the new SVGs when needed for print. |
| R-25 | Lighthouse mobile LCP is 2.7 to 3.1 s (simulated slow 4G, local server, 2026-10-22), above the 2.5 s target; almost all of it is render delay, not network or fonts. JS is 147 to 153 KB transferred (R-22). | Stage 18: bundle and hydration audit; measure on staging with a CDN. |
| R-26 | Parental consent for under-18 applicants is the applicant's declaration on the guardian's behalf (A-19); the guardian never confirms it directly. | Raise in the DG-06 review before real applications (AC-13). If a direct confirmation is needed, email the guardian a confirmation link (the notify package can send it); SMS needs DG-03. |
| R-23 | The API sees every request as coming from the frontend server (Next.js rewrites add no client address and pass through a spoofable `X-Forwarded-For`). Per-address rate limits are therefore shared by all visitors. | Per-email limits and the database lockout still work. Stage 19: the frontend passes the platform-provided client IP (Vercel or Cloudflare) in a header the API trusts only with a shared secret. |

