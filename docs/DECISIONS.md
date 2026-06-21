# 📑 Architecture Decision Records

Short ADRs capturing *why* we chose each significant approach. Newest last.

---

### ADR-001 — Monorepo with separate `backend/` and `frontend/`
**Decision:** One git repo, two top-level apps; backend is its own Go module.
**Why:** Atomic cross-stack changes, one source of truth, simple onboarding. Go module isolation
keeps backend deps clean.

### ADR-002 — `sqlc` + `pgx/v5` instead of an ORM
**Decision:** Write raw SQL in `internal/repository/queries`, generate type-safe Go via `sqlc`.
**Why:** Idiomatic Go, full control over SQL, compile-time safety, no ORM runtime surprises. Best
showcase of "real Go" for a benchmark project. Trade-off: must regenerate on schema change
(`make sqlc`).

### ADR-003 — Custom worker pool over Redis lists (not asynq)
**Decision:** Durable queue = Redis lists (`LPUSH`/`BRPOP`); processing = a hand-built bounded
goroutine pool with channels, graceful shutdown, and panic recovery.
**Why:** The project's purpose is to demonstrate Go concurrency. A library would hide exactly what we
want to show. Trade-off: we implement retry/dead-letter ourselves (acceptable, educational).

### ADR-004 — Full JWT auth from the start
**Decision:** Register/login with bcrypt + JWT (`golang-jwt/v5`), auth middleware on protected routes.
**Why:** Realistic multi-user model; tasks/plans are user-scoped. Chosen over a single seeded user to
mirror production. Trade-off: a little more upfront work before the AI features.

### ADR-005 — Two binaries: `cmd/server` and `cmd/worker`
**Decision:** API and background worker are separate processes sharing `internal/`.
**Why:** API must never block on AI/heavy work; independent scaling and clear separation of concerns.

### ADR-006 — `slog` for structured logging, stdlib config via `caarlos0/env`
**Decision:** Stdlib `slog` (JSON) for logs; typed env struct for config.
**Why:** Zero/minimal deps, modern, structured. Avoids heavier frameworks we don't need.

### ADR-007 — In-repo migration runner (`cmd/migrate`)
**Decision:** Use `golang-migrate` as a **library** in a small `cmd/migrate` rather than the CLI.
**Why:** Avoids build-tag/driver-selection friction with the migrate CLI and global installs; the
pgx/v5 driver is wired in code. `make migrate-up` just runs `go run ./cmd/migrate up`.

### ADR-008 — Reliable Redis-list queue (BLMOVE + processing + dead-letter)
**Decision:** The durable queue uses three Redis lists per namespace: `:jobs` (main), `:jobs:processing`,
`:jobs:dead`. Claim is `BLMOVE main→processing` (atomic, blocking); success acks via `LREM`; failure
re-enqueues with an incremented attempt or, once `MaxAttempts` is reached, moves to dead-letter. On
startup `Recover` moves any stale processing entries back to main.
**Why:** Plain `BRPOP` is at-most-once — a job popped just before a crash is lost. Moving the job to a
processing list keeps it durable until explicitly acked, giving at-least-once delivery with crash
recovery. Trade-off: handlers must be **idempotent** (a job may run more than once); retry/backoff and
dead-lettering are implemented by us rather than a library, which is the point (concurrency showcase).

### ADR-010 — Frontend: Next.js `/api` rewrite proxy; hand-built UI over shadcn
**Decision:** The Next.js app proxies `/api/*` → the Go backend via a `next.config.ts` rewrite, so the
browser is always same-origin (no CORS, no backend change). UI is a small set of hand-built Tailwind v4
components rather than the shadcn CLI; server state is TanStack Query, responses validated with Zod, JWT
kept in `localStorage` behind an `AuthProvider`, route protection done client-side (no middleware).
**Why:** The proxy keeps the backend free of CORS/auth-origin concerns and mirrors a real reverse-proxy
deployment. Hand-built components avoid the interactive shadcn init/codegen and give a distinctive
"command desk" look (warm ink + amber accent) instead of generic defaults. Trade-off: a few primitives
to maintain ourselves; client-side guards mean a brief loader flash before redirect (acceptable for an
SPA-style dashboard). Next 16 makes request APIs async-only and renames `middleware`→`proxy`; we sidestep
both by using client-component pages.

### ADR-009 — Worker pool: dispatcher + unbuffered channel + detached ack context
**Decision:** One dispatcher goroutine does the blocking Claim and hands deliveries to N workers over an
**unbuffered** channel; ack/retry use a context detached from the shutdown context
(`context.WithoutCancel` + timeout).
**Why:** The unbuffered channel gives natural backpressure — the dispatcher can't over-claim beyond
worker capacity, so at most `N` jobs sit outside the durable queue. Detaching the ack context ensures a
job that finishes during shutdown is still acked/retried instead of leaking back as an orphan. Graceful
shutdown drains in-flight work via a `sync.WaitGroup`.
