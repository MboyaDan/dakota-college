# Root task runner. Backend tasks live in backend/Makefile; frontend tasks are
# npm scripts in frontend/package.json. Run `make help`.

.DEFAULT_GOAL := help
.PHONY: help setup up down api-test web-dev web-check check

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

setup: ## First-time setup: env files and frontend dependencies
	@test -f backend/.env || cp backend/.env.example backend/.env
	@test -f frontend/.env.local || cp frontend/.env.example frontend/.env.local
	cd frontend && npm ci

up: ## Start the backend stack in Docker (API, PostgreSQL, MinIO, Mailpit)
	$(MAKE) -C backend up

down: ## Stop the backend stack
	$(MAKE) -C backend down

api-test: ## Go lint and tests, including database tests (needs `make up`)
	$(MAKE) -C backend lint test-db

web-dev: ## Run the Next.js dev server on http://localhost:3000
	cd frontend && npm run dev

web-check: ## Frontend format, lint, typecheck, unit tests, build
	cd frontend && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build

check: api-test web-check ## Everything CI runs except Playwright and the secret scan
