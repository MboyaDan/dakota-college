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
| D-15 | Brief safeguard "a role in use cannot be deleted" (brief 11) is added to Stage 4 alongside the two protections in CLAUDE.md. | Brief wins where it adds to the prompt. | 0 |
| D-16 | `CLAUDE.md` is the canonical master prompt. `docs/claude/MASTER_PROMPT.md` is a copy; if they differ, `CLAUDE.md` wins and I will flag the drift. | Avoids two sources of truth (R-13). | 0 |
| D-17 | Seed and fixture data use names like "Test Student 001", emails at `example.invalid`, and phone numbers in the form `+254 700 000 0NN` flagged as test data in the seed; programmes carry "(Sample)" in the title. | Reserved, obviously fake values (NN-06). Exact phone range confirmed in Stage 3. | 0 |
| D-18 | Password reset is by email in Stage 4. SMS reset (brief 5.1) is added only after DG-03 clears. | SMS provider is a decision gate. | 0 |
| D-19 | Upload scanning sits behind a scanner interface. Stage 9 ships a ClamAV adapter for Compose if it fits; otherwise uploads are marked "not scanned" and the report says so. | Brief 8 requires scanning "if available" (brief 13); we never claim scanning that is not running (R-15). | 0 |
