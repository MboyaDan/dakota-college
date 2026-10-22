# Traceability

Maps each acceptance criterion (brief 16) and non-negotiable rule (brief 21) to the stage that builds it, how it is verified, and the evidence. Status changes only when a stage report contains real evidence (a test name, a command output, a measured score).

Status values: **Planned**, **In progress**, **Pass** (with evidence), **Fail**, **Gated** (waiting on a decision gate).

## Acceptance criteria

| ID | Criterion (short) | Stage(s) | Verification | Status | Evidence |
|---|---|---|---|---|---|
| AC-01 | Every public page passes axe and keyboard navigation, no critical issues | 2, 6, 7, 18 | Playwright plus axe on every public route; manual keyboard checklist | In progress | Stage 2: `tests/e2e/design-system.spec.ts`: axe clean (critical and serious) on `/` and `/design`, with dialog and menus open, at 360 px and desktop; keyboard tests for skip link, tabs, dialog, menus (2026-10-22). |
| AC-02 | Lighthouse mobile 90+ on home, programme, admissions (all four categories) | 6, 18 | Lighthouse CLI mobile preset, scores recorded with date | Planned | |
| AC-03 | Find programme, see fees, requirements, intake, start application in at most 4 clicks | 6, 9 | Playwright test counting navigations from home | Planned | |
| AC-04 | Application saves, resumes, submits once (retry never duplicates), tracked by reference | 9 | Go tests (idempotent submit with repeated key), Playwright resume flow | Planned | |
| AC-05 | Documents private, type and size checked, staff-only short-lived links | 9, 17 | Go tests on upload validation and signed link expiry; unauthenticated fetch denied | Planned | |
| AC-06 | Staff edit programmes, intakes, fees, news, events, FAQs; changes live within cache period | 5, 6 | Playwright: editor publishes, page shows change after revalidation | In progress | Stage 5a API: `TestProgrammeEditorialWorkflow` (editor drafts, registrar publishes, edits of live content wait for approval, fees change only on approval), `TestEveryContentKindWorkflow` (all ten types). Site and cache half: Stage 6. |
| AC-07 | Backend denies unauthorised API calls even if frontend bypassed | 4, 17 | Go tests calling every protected route without and with wrong permissions | Pass (Stage 4 routes) | `TestEveryRouteIsProtected` walks the route table: every non-public route gives 401 without a session, and 403 to a user holding every other permission; a student gets 403 on every admin route. New routes are covered automatically; `TestOpenAPIMatchesRoutes` keeps the contract in step (2026-10-22). |
| AC-08 | Student sees only own data; trainer only assigned units | 11, 12 | Go tests with two students and two trainers | Planned | |
| AC-09 | Results visible only after approval and release; every change audited | 13 | Go tests on result visibility and audit rows | Planned | |
| AC-10 | Statements reconcile from records; no floating point money | 3, 14 | Reconciliation test; schema check that money columns are BIGINT | In progress | Stage 3: `TestMoneyIsIntegerCents` (every `*_cents` column bigint, no float or numeric columns, negative and non-KES amounts rejected). Reconciliation: Stage 14. |
| AC-11 | M-Pesa paid only through verified flow or finance confirmation | 14, 15 | Go tests: typed reference alone never marks paid | Planned (manual path); Gated (DG-02) | |
| AC-12 | Last active Super Admin cannot be removed; system.admin not grantable via roles | 3, 4 | Go tests for each protection | Pass | `TestSuperAdminAndRoleProtections` (last Super Admin cannot lose the role or be disabled; ICT Admin cannot grant, remove or disable Super Admins; system.admin refused in role create and edit for everyone; Super Admin role not editable), `TestConcurrentSuperAdminRemovalLeavesOne` (5 rounds of simultaneous mutual removal leave exactly one), `TestRolesInUseAndStarterRolesCannotBeDeleted`, `TestRolesAndPermissions` (2026-10-22). |
| AC-13 | Privacy notice, consent, parental consent live before real applications | 7, 9, 20 | Page present; consent rows stored; under-18 path test | Planned (gated by DG-06) | |
| AC-14 | Backups run, restore tested, HTTPS only, no secrets committed | 1, 19 | Restore drill log; secret scan in CI; HSTS check | In progress | Stage 1: gitleaks in CI; local `gitleaks git` scan of history found no leaks (2026-10-22). Backups, restore, HTTPS: Stage 19. |
| AC-15 | Public pages serve from cache when API is unavailable | 6 | Playwright with API stopped | Planned | |
| AC-16 | No unverified statistic, ranking or accreditation claim on public pages | 3, 5, 6, 20 | Statistics require verification date (DB constraint); empty hides; launch content review | In progress | Stage 3: `TestUnverifiedClaimsCannotBePublished` (DB refuses to publish statistics or accreditations without verification date and evidence). |
| AC-17 | Test data and test users absent from production | 3, 20 | Seed refuses in production (test); go-live query check | In progress | Stage 3: `TestGuardRefuses*` and manual run (seed exits 1 for production, staging and a Supabase host); `is_sample` on all seeded content with `CountSampleRows` for the go-live check; test users all `@example.invalid`. |
| AC-18 | Training, runbooks, known limitations, handover complete | 19, 21 | Documents present and reviewed by client | Planned | |

## Non-negotiable rules

| ID | Rule (short) | Enforced in | How |
|---|---|---|---|
| NN-01 | No unverified claims | 3, 5, 6 | Verification date and evidence required to publish a statistic or accreditation (DB constraint, Stage 3; API check with field messages, Stage 5a `TestContentRules`) |
| NN-02 | Backend enforces authorisation | 4 onward | Permission middleware on every route; route table test |
| NN-03 | Money integer cents; balances derived | 14 | BIGINT columns, no stored balances |
| NN-04 | Idempotent submissions and payments | 9, 14, 15 | `idempotency_records` table and middleware |
| NN-05 | No destructive edits to completed records | 3, 10, 13, 14 | Append-only triggers on audit log, status history, consents (Stage 3, `TestAppendOnlyTables`); compensating entries later |
| NN-06 | No real data in seed or tests | 3 onward | Seed conventions (D-17), production refusal |
| NN-07 | Production DB is managed or operated, never a throwaway container | 19 | Deployment docs, DG-04 |
| NN-08 | Personal data and photos used only with consent | 5a, 7, 9 | `consents` table; gallery photos need confirmed consent to publish (DB and API, `TestContentRules`); uploads stripped of metadata including GPS (`TestProcessAppliesOrientationAndStripsMetadata`) |
| NN-09 | No copying from reference site | All | Patterns only; original copy |
| NN-10 | M-Pesa paid needs verified path or finance confirmation | 14, 15 | State machine on payments |
