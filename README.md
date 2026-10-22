# Dakota Training College platform

Public website, online applications, student portal and staff portal for Dakota Training College, Makueni.

- `frontend/`: Next.js (App Router, TypeScript, Tailwind v4). Public site plus `/student` and `/staff` portals.
- `backend/`: Go API (Gin, pgx, sqlc, goose). The only database client. Contract: `backend/openapi.yaml`.
- `docs/`: brief, plan, progress, assumptions, decisions, traceability, brand assets.

Build instructions for contributors (and Claude Code) are in `CLAUDE.md`. Current status is in `docs/PROGRESS.md`.

## Requirements

Go 1.25, Node 24, Docker Desktop (or Docker Engine with Compose v2).

## Run it locally

```sh
make setup        # copies .env.example files, installs frontend packages
make up           # API, PostgreSQL, MinIO and Mailpit in Docker
make web-dev      # Next.js on http://localhost:3000
```

| Service | URL |
|---|---|
| Website | http://localhost:3000 |
| API (direct) | http://localhost:8180/api/v1/health |
| API via the website | http://localhost:3000/api/v1/health |
| Mailpit (caught email) | http://localhost:8025 |
| MinIO console | http://localhost:9001 |
| PostgreSQL | localhost:5442 (user `dakota`, see `backend/.env.example`) |

Ports avoid the usual 8080 and 5432 so the stack can run beside other projects. Change them in `backend/.env` and `frontend/.env.local`.

## Checks

```sh
make check                              # Go lint + tests (with DB), frontend lint, types, unit tests, build
cd frontend && npm run test:e2e         # Playwright smoke tests and axe (after npm run build)
```

CI runs the same checks on every pull request, plus a gitleaks secret scan.

## Rules that matter

- Never commit `.env` files, secrets, real personal data or database dumps.
- Seed data is for development and tests only and refuses to run in production.
- Migrations are never run automatically against production.
