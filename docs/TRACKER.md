# 📋 Build Tracker — AI Chief of Staff

> Living progress board. Update this at the end of **every** batch so any new session (you or Claude)
> can instantly see what's done and what's next. Source of truth for status.

**Legend:** ✅ done · 🔄 in progress · ⬜ not started

---

## Current status

- **Last updated:** 2026-06-21
- **Active batch:** ✅ Batches 0–2 COMPLETE & verified
- **Next step:** Batch 3 — AI layer (Claude). API key is set; default model **claude-haiku-4-5** (credit-efficient).

> First delivery (0+1) verified 2026-06-21: vet/gofmt clean, tests green, full curl run.
> Batch 2 verified 2026-06-21: queue + pool unit tests (miniredis) green; live run showed 4 workers
> processing concurrently, retry→dead-letter on failures, and graceful shutdown.
> Note: Postgres host port is **5433** (5432 was already taken by a local Postgres).

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

### ⬜ Batch 3 — AI Layer (Claude)
- [ ] Claude client wrapper (context, timeouts, retry)
- [ ] Planner / Priority / Breakdown agents with strict JSON + schema validation
- [ ] Prompt templates documented in AI_DESIGN.md
- [ ] Mocked unit tests + key-gated live smoke test
- [ ] **Verify (needs key):** sample tasks → valid JSON schedule

### ⬜ Batch 4 — End-to-End Planning Flow
- [ ] `POST /plans/generate` enqueues job (202 + job_id)
- [ ] Worker: tasks → Planner → persist `daily_plans` → status
- [ ] `GET /plans/:date`, `GET /plans/jobs/:id`
- [ ] **Verify:** create → generate → poll → fetch plan

### ⬜ Batch 5 — Next.js Frontend
- [ ] Next.js latest + TS + Tailwind v4 + shadcn/ui scaffold
- [ ] Auth pages + token handling
- [ ] Tasks dashboard (CRUD) + generate plan + schedule view + polling
- [ ] **Verify:** full flow in browser

### ⬜ Batch 6 — V2 (optional)
- [ ] Priority view · adaptive replanning · weekly reports · stress mode · habits · memory

---

## Decision log pointer
Architecture decisions are recorded in [DECISIONS.md](DECISIONS.md).

## How to resume in a new window
1. Read this file (status + next step) and [DECISIONS.md](DECISIONS.md).
2. Skim [ARCHITECTURE.md](ARCHITECTURE.md) for the layer map.
3. Run `make up` then `cd backend && make run` to confirm a green baseline before changing anything.
