# ── AI Chief of Staff — top-level dev commands ───────────────────────────────
.DEFAULT_GOAL := help

.PHONY: help up down logs ps backend-% migrate-up migrate-down sqlc run worker test lint tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_%-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Start Postgres + Redis + Adminer
	docker compose up -d

down: ## Stop infra (keep volumes)
	docker compose down

logs: ## Tail infra logs
	docker compose logs -f

ps: ## Show infra status
	docker compose ps

# Delegate the common backend targets so you can run them from the repo root.
migrate-up migrate-down sqlc run worker enqueue test lint tidy: ## Run the matching backend target
	$(MAKE) -C backend $@
