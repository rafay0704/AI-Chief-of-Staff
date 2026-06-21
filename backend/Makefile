# ── Backend dev commands ─────────────────────────────────────────────────────
.DEFAULT_GOAL := help

# Load .env from the repo root so DATABASE_URL etc. are available to tools.
ROOT := ..
ifneq (,$(wildcard $(ROOT)/.env))
	include $(ROOT)/.env
	export
endif

MIGRATIONS_DIR := migrations

.PHONY: help tools tidy sqlc migrate-up migrate-down migrate-version run worker build test test-unit lint fmt

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

tools: ## Install optional dev tooling (air, golangci-lint); sqlc is a go tool dep
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tidy: ## Sync go.mod / go.sum
	go mod tidy

sqlc: ## Generate type-safe Go from SQL
	go tool sqlc generate

migrate-up: ## Apply all migrations
	go run ./cmd/migrate up

migrate-down: ## Roll back the last migration
	go run ./cmd/migrate down 1

migrate-version: ## Print current migration version
	go run ./cmd/migrate version

run: ## Run the API server (live reload if air is installed)
	@command -v air >/dev/null 2>&1 && air -c .air.toml || go run ./cmd/server

worker: ## Run the background worker pool
	go run ./cmd/worker

enqueue: ## Enqueue demo jobs: make enqueue n=10 sleep=400 (add fail=1 to test retries)
	go run ./cmd/enqueue -n $(or $(n),5) -sleep $(or $(sleep),300) $(if $(fail),-fail,)

build: ## Build server + worker binaries into ./bin
	go build -o bin/server ./cmd/server
	go build -o bin/worker ./cmd/worker

test: ## Run all tests (unit + integration via testcontainers)
	go test ./... -count=1

test-unit: ## Run only fast unit tests (skip integration)
	go test ./... -short -count=1

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...
