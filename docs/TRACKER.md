# 📋 Build Tracker — AI Chief of Staff

> Living progress board. Update this at the end of **every** batch so any new session (you or Claude)
> can instantly see what's done and what's next. Source of truth for status.

**Legend:** ✅ done · 🔄 in progress · ⬜ not started

---

## Current status

- **Last updated:** 2026-06-24
- **Active batch:** ✅ Batches 0–7 COMPLETE & verified — full MVP + 6 V2 features
- **Next step:** (optional) adaptive replanning, preference memory; or deploy/CI hardening

> Batch 7 (showcase features) verified 2026-06-24 — all live against `rafay@gmail.com`:
> **Analytics** (`GET /stats`: completion rate, focus minutes, priority mix, 7-day trend),
> **Focus modes** (balanced/deep_focus/stress_relief/light on the planner),
> **Weekly AI report** (4th Claude agent — narrative wins/watch-outs/suggestions),
> **Habit tracking** (habits + check-ins tables, streaks 5/3/4, contribution grid UI).
> **45 backend tests pass**; frontend build + lint clean. App runs: backend :8080, web :3000.

> Batch 4 verified: full chain live — generate → queue → worker → Claude Planner → Postgres → fetch.
> Batch 5 verified 2026-06-22: Next.js dashboard builds clean (5 routes); ran the full flow **through
> the frontend `/api` proxy** (register → tasks → generate → poll running→done → 4-block plan rendered).
> Frontend runs on **:3000**, proxies `/api/*` → backend `:8080` (no CORS, no backend change).
> Default model **claude-haiku-4-5-20251001**. Postgres host port **5433** (5432 taken locally).

---

## Batches

### ✅ Batch 0 — Foundation & Docs
- [x] git init + monorepo scaffold
- [x] `docker-compose.yml` (Postgres 17, Redis 7, Adminer) — Postgres on host **5433**
- [x] `.env.example`, root + backend `Makefile`, `.gitignore`
- [x] docs: README, ARCHITECTURE, SETUP, API, AI_DESIGN, DECISIONS, TRACKER
- [x] `sqlc.yaml`, `.air.toml`, `.golangci.yml`
- [x] `config` (typed env), `platform/logger` (slog), `platform/db` (pgxpool), `platform/redis`
- [x] Gin server: `/healthz` + `/readyz`, middleware (request-id, recovery, logging)
- [x] **Verified:** `make up && make run`; `/healthz` 200, `/readyz` OK

### ✅ Batch 1 — Auth + Users + Task CRUD
- [x] Migrations: `users`, `tasks`, `daily_plans` (enums, FKs, indexes)
- [x] sqlc queries → generated repository (`internal/repository`)
- [x] Auth: register / login (bcrypt + JWT), JWT middleware, `GET /me`
- [x] Task CRUD scoped to user (`internal/service`, `internal/http/handler`)
- [x] Validation (gin binding) + JSON error envelope
- [x] Unit (fake repo) + integration (testcontainers Postgres) tests
- [x] **Verified:** register → login → task CRUD via curl; `make test` green

### ✅ Batch 2 — Redis Queue + Worker Pool
- [x] `queue` package: reliable Redis-list queue (BLMOVE main→processing, ack/retry, dead-letter, recover)
- [x] `worker` pool (dispatcher + goroutines + unbuffered channel), graceful shutdown, panic recovery, backoff retry
- [x] `cmd/worker` (pool + demo handler) and `cmd/enqueue` dev tool (`make enqueue n=10 sleep=400 [fail=1]`)
- [x] Unit tests with miniredis (queue + pool incl. retry/dead-letter/unknown-type)
- [x] **Verified:** 4 workers processing concurrently, retry→dead-letter, clean SIGINT shutdown

### ✅ Batch 3 — AI Layer (Claude)
- [x] Claude client wrapper (`Completer` interface, context timeouts, SDK auto-retry) — `internal/ai/client.go`
- [x] Planner / Priority / Breakdown agents with strict JSON + schema validation + 1 repair round-trip
- [x] Versioned prompt templates (`planner-v1`, `priority-v1`, `breakdown-v1`) — `internal/ai/prompts.go`
- [x] Mocked unit tests (fake Completer) + key-gated live smoke test (`live_test.go`)
- [x] **Verified:** live Claude call returned a valid schedule; agents testable without a key

### ✅ Batch 4 — End-to-End Planning Flow
- [x] `POST /plans/generate` enqueues job (202 + job_id); plan id == job id (`daily_plans` upsert)
- [x] Worker plan handler: pending tasks → Planner agent → persist `daily_plans` → status transitions
- [x] `GET /plans/jobs/:id` (poll) + `GET /plans?date=YYYY-MM-DD` (fetch)
- [x] PlanService unit tests (fake enqueuer + querier): enqueue, idempotent re-gen, lifecycle, not-found
- [x] **Verified:** create → generate → poll (queued→running→done) → fetch full schedule, live

### ✅ Batch 5 — Next.js Frontend
- [x] Next.js 16 + TS + Tailwind v4 scaffold; `/api/*` rewrite proxy → backend (no CORS)
- [x] Typed API client (`lib/api.ts`) + Zod schemas + auth context (JWT in localStorage)
- [x] Login + register pages with validation + error handling
- [x] Dashboard: task CRUD UI + generate-plan controls + schedule timeline + job-status polling
- [x] Custom dark "command desk" theme (warm ink + amber accent), hand-built UI primitives
- [x] **Verified:** `pnpm build` clean (5 routes); full flow driven through the `/api` proxy

### ✅ Batch 6 — AI task tools
- [x] **Priority engine** — `POST /ai/prioritize` ranks pending tasks (rank/urgent/reason + drop suggestions); UI: Prioritize button, rank badges, drop list
- [x] **Task breakdown** — `POST /ai/breakdown/:id` splits a task into ordered steps; UI: per-task expander

### ✅ Batch 7 — Showcase features
- [x] **Productivity Analytics** — `GET /stats` (SQL aggregation: completion rate, focus minutes, priority mix, 7-day trend); Insights panel with completion ring, stat tiles, trend bars, priority mix
- [x] **Focus / Stress modes** — `mode` on plan generate (balanced/deep_focus/stress_relief/light) reshapes the planner prompt; mode selector in the plan panel
- [x] **Weekly AI Report** — Reporter agent (`reporter-v1`) + `POST /ai/weekly-report`; narrative headline/summary/wins/watch-outs/suggestions in the Insights panel
- [x] **Habit tracking** — `habits` + `habit_checkins` tables; `GET/POST/DELETE /habits`, check-in toggle; streak computation; 4-week contribution-grid UI
- [ ] (still open) adaptive auto-replanning · preference memory

---

## Decision log pointer
Architecture decisions are recorded in [DECISIONS.md](DECISIONS.md).

## How to resume in a new window
1. Read this file (status + next step) and [DECISIONS.md](DECISIONS.md).
2. Skim [ARCHITECTURE.md](ARCHITECTURE.md) for the layer map.
3. Run `make up` then `cd backend && make run` to confirm a green baseline before changing anything.
