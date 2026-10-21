.PHONY: help up down run migrate seed seed-admin test test-unit test-e2e test-race lint build

# The api, migrate and seed commands read their settings from ./.env (copy
# .env.example) and the environment; nothing is exported here so .env applies.
TEST_DB_DSN ?= postgres://postgres:postgres@127.0.0.1:5432/staryield_test?sslmode=disable
TEST_REDIS_URL ?= redis://127.0.0.1:6379/15

help: ## Show targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

up: ## Start PostgreSQL and Redis (docker compose)
	docker compose up -d postgres redis

down: ## Stop local infrastructure
	docker compose down

run: ## Run the API
	go run ./cmd/api

migrate: ## Apply migrations
	go run ./cmd/migrate up

seed: ## Seed advisors, packs (and accounts from STARYIELD_SEED_* env)
	go run ./cmd/seed

seed-admin: ## Create the admin, or reset its password, and nothing else (STARYIELD_SEED_ADMIN_* from the shell)
	go run ./cmd/seed -admin-only

test: test-unit test-e2e ## Run all tests

test-unit: ## Unit tests (no infrastructure needed)
	go test ./internal/... -count=1

test-e2e: ## End-to-end tests against PostgreSQL + Redis
	STARYIELD_E2E=1 STARYIELD_TEST_DATABASE_DSN=$(TEST_DB_DSN) STARYIELD_TEST_REDIS_URL=$(TEST_REDIS_URL) go test ./tests/... -count=1

test-race: ## Everything with the race detector
	STARYIELD_E2E=1 STARYIELD_TEST_DATABASE_DSN=$(TEST_DB_DSN) STARYIELD_TEST_REDIS_URL=$(TEST_REDIS_URL) go test ./... -race -count=1

lint: ## gofmt + go vet
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...

build: ## Build binaries into ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/migrate ./cmd/migrate
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/seed ./cmd/seed
