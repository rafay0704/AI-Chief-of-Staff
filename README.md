# 🧠 AI Chief of Staff

An AI-powered "second brain" that ingests your tasks and goals, prioritizes them with Claude, and
generates (and dynamically re-plans) a daily schedule. Built to showcase a **production-grade Go
backend** — clean architecture, real concurrency (goroutines, channels, worker pools), an async
Redis-backed job queue, and a typed Claude AI layer — with a **Next.js** dashboard on top.

> This is a benchmark/portfolio project: every decision favors idiomatic, production-style patterns
> over shortcuts.

## Stack

| Layer | Tech |
|---|---|
| Backend | Go 1.26, Gin, `pgx/v5`, `sqlc`, `golang-migrate` |
| Async | Redis (durable list queue) + custom goroutine worker pool |
| AI | Anthropic Claude (`anthropic-sdk-go`) |
| Auth | JWT (`golang-jwt/v5`) + bcrypt |
| Data | PostgreSQL 17 |
| Frontend | Next.js latest, TypeScript, Tailwind v4, TanStack Query *(Batch 5)* |

## Quickstart

**1. Backend + infra**

```bash
cp .env.example .env          # then edit secrets (note: .env is gitignored)
make up                       # start Postgres + Redis (+ Adminer on :8081)
cd backend
make tidy                     # install Go deps
make migrate-up               # create schema
make sqlc                     # generate typed repository code
make run                      # API on :8080  (live-reload if `air` installed)
make worker                   # second terminal: background job worker
```

**2. Frontend** (needs the backend running)

```bash
cd frontend
pnpm install
pnpm dev                      # dashboard on http://localhost:3000
```

The frontend proxies `/api/*` → the backend (`:8080`), so there's no CORS setup. Open
http://localhost:3000, register, add tasks, and hit **Generate plan**.

Health check:

```bash
curl localhost:8080/healthz   # -> {"status":"ok"}
curl localhost:8080/readyz    # -> checks Postgres + Redis
```

## Documentation

| Doc | Purpose |
|---|---|
| [docs/TRACKER.md](docs/TRACKER.md) | ⭐ Living progress board — what's done & what's next |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design, layers, data flow |
| [docs/SETUP.md](docs/SETUP.md) | Detailed local dev setup |
| [docs/API.md](docs/API.md) | REST endpoint reference |
| [docs/AI_DESIGN.md](docs/AI_DESIGN.md) | Claude agents, prompts, JSON contracts |
| [docs/DECISIONS.md](docs/DECISIONS.md) | Architecture decision records |

## Repo layout

```
backend/    Go API + worker (clean architecture)
frontend/   Next.js dashboard (added in Batch 5)
docs/        living documentation
```
