# Backend tasks. Run `make help` for the list.

# Load .env if present so `make run` and `make test` see local settings.
-include .env
export

.DEFAULT_GOAL := help

.PHONY: help up down logs ps run build test test-db lint fmt docker-build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

up: ## Start API, PostgreSQL, MinIO and Mailpit in Docker
	docker compose up --build -d

down: ## Stop the Docker stack (keeps data volumes)
	docker compose down

logs: ## Follow API logs
	docker compose logs -f api

ps: ## Show stack status
	docker compose ps

run: ## Run the API on the host against the Compose database
	go run ./cmd/api

build: ## Build the API binary into bin/
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api

test: ## Run Go tests (database tests skip unless TEST_DATABASE_URL is set)
	go test -race ./...

test-db: ## Run Go tests and require the database tests to run
	REQUIRE_DB_TESTS=1 go test -race ./...

lint: ## Check formatting and run go vet
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...

fmt: ## Format Go code
	gofmt -w .

docker-build: ## Build the production API image
	docker build -t dakota-api:local .
