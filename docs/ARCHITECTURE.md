# 🏛️ Architecture

## Overview

AI Chief of Staff is a Go backend + Next.js frontend. The backend follows **clean / layered
architecture** so that business logic never depends on transport (HTTP), storage (Postgres), or the
AI provider directly — those are details behind interfaces.

```
            ┌─────────────┐      HTTP/JSON      ┌──────────────────────────────┐
            │  Next.js UI │ ──────────────────▶ │  Go API server (cmd/server)  │
            └─────────────┘ ◀────────────────── │  Gin + middleware + handlers │
                                                 └───────────────┬──────────────┘
                                                                 │ service layer
                                          ┌──────────────────────┼─────────────────────┐
                                          ▼                      ▼                     ▼
                                  ┌──────────────┐      ┌─────────────────┐    ┌──────────────┐
                                  │ repository   │      │  queue (Redis)  │    │  ai (Claude) │
                                  │ (sqlc + pgx) │      │  LPUSH / BRPOP  │    │  agents      │
                                  └──────┬───────┘      └────────┬────────┘    └──────────────┘
                                         ▼                       ▼
                                  ┌──────────────┐      ┌─────────────────────────────┐
                                  │ PostgreSQL   │      │  Worker (cmd/worker)        │
                                  └──────────────┘      │  goroutine pool + channels  │
                                                        └─────────────────────────────┘
```

## Layers (dependency direction points inward)

| Package | Responsibility | Depends on |
|---|---|---|
| `internal/http` | Gin router, handlers, middleware. Maps HTTP ⇄ service. | service, domain |
| `internal/service` | Business logic, orchestration, transactions. | domain, repository, queue, ai |
| `internal/repository` | sqlc-generated typed DB access (`pgx/v5`). | domain (types) |
| `internal/domain` | Core models, enums, sentinel errors. | nothing |
| `internal/ai` | Claude client + Planner/Priority/Breakdown agents. | domain |
| `internal/queue` | Redis-backed durable job queue. | — |
| `internal/worker` | Goroutine worker pool + job handlers. | queue, service, ai |
| `internal/platform/*` | Cross-cutting infra: db pool, redis client, slog. | — |
| `internal/config` | Typed env config loading. | — |

**Rule:** handlers are thin; services hold logic; repositories only do data access. AI and queue are
injected into services as interfaces so they can be mocked in tests.

## Two binaries

- `cmd/server` — the HTTP API. Never blocks on AI; heavy work is enqueued.
- `cmd/worker` — long-running consumer. Pulls jobs from Redis, runs them on a bounded goroutine pool.

## Core data flow (planning)

```
User creates tasks ─▶ POST /plans/generate ─▶ service enqueues PlanJob (Redis)  ─▶ 202 {job_id}
                                                          │
worker pool worker ◀── BRPOP ──────────────────────────┘
        │  load tasks (repository)
        │  call Planner agent (ai → Claude, strict JSON)
        │  persist daily_plans.plan_json (repository)
        ▼  mark job done
Frontend polls GET /plans/jobs/:id ─▶ ready ─▶ GET /plans/:date renders schedule
```

## Concurrency model

- The worker owns a **bounded pool** of N goroutines (`WORKER_CONCURRENCY`).
- A single dispatcher goroutine does blocking `BRPOP` and fans jobs to workers over a channel.
- Each worker recovers from panics, honors `context.Context` cancellation, and retries transient
  failures with backoff (re-enqueue or dead-letter on exhaustion).
- Graceful shutdown: on SIGINT/SIGTERM the dispatcher stops accepting, in-flight jobs finish, then
  the pool drains via `sync.WaitGroup`.

## Error & observability conventions

- Structured `slog` JSON logs; every request carries an `X-Request-ID` propagated via context.
- One JSON error envelope for all API errors: `{"error":{"code","message","details?"}}`.
- Domain sentinel errors (e.g. `ErrNotFound`, `ErrConflict`) are mapped to HTTP status in one place.
