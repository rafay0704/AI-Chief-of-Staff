# 📚 Learning ACOS — A Production-Grade Go + Next.js Walkthrough

> **Who this is for:** you — a Node/Next developer who just learned Go. This doc explains
> not just *what* the code does, but *why* it's structured this way, and how each decision
> maps to the production patterns you'll see in real Go shops. Where Go differs from the
> Node mental model, I call it out explicitly: **🟢 Node → Go**.

Read this top-to-bottom once, then keep it open as a map while you click through the files.

---

## 0. The 30-second mental model

```
                      ┌──────────────────────────────────────────────┐
   Browser            │                  Next.js (3000)               │
   (you)  ───────────▶│  React + TanStack Query + Zod                 │
                      │  proxies /api/* ──▶ backend (no CORS)         │
                      └───────────────────────┬──────────────────────┘
                                              │ HTTP + JWT
                      ┌───────────────────────▼──────────────────────┐
                      │              Go API server (8080)             │
                      │  Gin → middleware → handler → service → repo  │
                      └─────┬───────────────────────────────┬────────┘
                            │ SQL (pgx/sqlc)                 │ LPUSH job
                  ┌─────────▼─────────┐            ┌─────────▼─────────┐
                  │  PostgreSQL 17    │            │     Redis 7       │
                  │  (source of truth)│            │  (durable queue)  │
                  └─────────▲─────────┘            └─────────┬─────────┘
                            │ SetPlanDone                    │ BLMOVE (claim)
                      ┌─────┴───────────────────────────────▼─────────┐
                      │            Go worker pool (separate binary)    │
                      │  dispatcher + N goroutines → Claude (Anthropic)│
                      └────────────────────────────────────────────────┘
```

**Two binaries, one codebase.** `cmd/server` serves HTTP; `cmd/worker` drains the queue.
They share everything under `internal/`. This is the single most important architectural
idea in the project — keep it in mind as you read.

---

## 1. Folder structure — and *why* each folder exists

```
backend/
├── cmd/                  ← ENTRYPOINTS. Each subdir = one runnable binary (package main).
│   ├── server/           ← the HTTP API
│   ├── worker/           ← the background job processor
│   ├── migrate/          ← runs DB migrations (golang-migrate as a library)
│   └── enqueue/          ← dev tool to push demo jobs (test the queue by hand)
│
├── internal/            ← ALL real code. "internal" is special: Go forbids
│   │                       importing it from outside this module. Encapsulation
│   │                       enforced by the compiler, not by convention.
│   │
│   ├── config/           ← load + validate env into a typed Config struct
│   ├── domain/           ← pure business types & errors. NO imports of db/http/redis.
│   ├── repository/       ← data access. sqlc-GENERATED code + your .sql files.
│   ├── service/          ← business logic. Orchestrates repo + queue + ai.
│   ├── http/             ← transport layer: router, handlers, middleware.
│   ├── worker/           ← the goroutine pool + job handlers.
│   ├── queue/            ← the Redis-list queue primitive (Enqueue/Claim/Ack…).
│   ├── ai/               ← the Claude client + typed "agents" + prompts.
│   ├── auth/             ← JWT signing/parsing + token manager.
│   └── platform/         ← thin wrappers around external infra (db, redis, logger).
│
├── migrations/          ← versioned SQL schema (000001_init.up/down.sql)
├── sqlc.yaml            ← config: "turn these .sql files into typed Go"
├── Makefile             ← the command palette (make run, make worker, …)
└── .air.toml           ← live-reload config for local dev
```

### 🟢 Node → Go: the folder philosophy
In Express you might have `routes/`, `controllers/`, `models/` and wire them loosely.
Go production apps lean on the **`cmd/` + `internal/`** convention:

- **`cmd/`** = "what can I run?" Each folder is a `main` package = one binary. This is
  why ACOS can ship a server *and* a worker from one repo with zero duplication.
- **`internal/`** = "the library." The `internal` name is a **compiler-enforced** privacy
  boundary — no other module can import your guts. There's no equivalent in Node; it's
  closer to "everything is private unless you publish it."

### The layering rule (memorize this)
Dependencies point **inward and downward**, never up:

```
http ──▶ service ──▶ repository ──▶ Postgres
            │
            ├──▶ queue ──▶ Redis
            └──▶ ai    ──▶ Claude
         (everyone may import) domain
```

- `domain` imports **nothing** from the app — it's the stable core.
- `http` (handlers) never touches the DB directly; it calls a `service`.
- `service` never speaks HTTP; it takes plain args and returns domain types.

Why this matters: you can test a service with a fake repo, swap Gin for another router,
or reuse the *same* service from both the HTTP server and the worker — which is exactly
what `PlanService` does.

---

## 2. The request lifecycle (trace one call end-to-end)

Let's follow **"Generate plan"** because it touches every layer and the async pipeline.

### Step 1 — Frontend fires the request
`frontend/components/plan-panel.tsx` → `api.generatePlan(token, {...})` →
`POST /api/plans/generate`. Next.js rewrites `/api/*` to `http://localhost:8080/*`
(`next.config.ts`), so there's **no CORS** to configure.

### Step 2 — Router + middleware
`internal/http/router.go` puts this route behind `middleware.Auth(tokens)`. The chain is:
```
RequestID → Logger → Recovery → Auth → handler
```
`Auth` validates the JWT and stuffs the `userID` into the Gin context. If the token is
bad, the handler never runs.

### Step 3 — Handler (transport only)
`internal/http/handler/plan.go` does three things and nothing more:
1. **Decode + validate** the JSON body into a request struct.
2. Pull `userID` from context.
3. Call `h.Plans.Generate(ctx, userID, …)` and **translate the result to HTTP** (status
   code + JSON). Business rules do *not* live here.

### Step 4 — Service (the brain)
`internal/service/plan.go` → `PlanService.Generate`:
1. Parses/validates the date (returns `domain.ErrValidation` on bad input).
2. `repo.UpsertPlanQueued(...)` → writes a `daily_plans` row with `status='queued'`
   (Postgres). **The row's `id` becomes the job id.**
3. Builds a `queue.Job` with the payload and `queue.Enqueue` → `LPUSH` onto Redis.
4. Returns the queued plan **immediately**. The HTTP call finishes in milliseconds;
   the slow Claude work happens elsewhere. This is the async pattern.

### Step 5 — Worker picks it up (different process!)
`cmd/worker` runs a `worker.Pool`. Its dispatcher goroutine is blocked on
`queue.Claim` (a Redis `BLMOVE`). The moment your job lands, it's claimed and handed to
a free worker goroutine, which runs the `plan` handler (`internal/worker/planjob.go`):
1. `MarkRunning` → updates the row to `status='running'`.
2. Loads the user's tasks, calls the Claude **Planner agent** (`internal/ai`).
3. `Complete(planID, scheduleJSON)` → `status='done'` + stores `plan_json`.
   On error → `Fail(planID, reason)` → `status='failed'`.

### Step 6 — Frontend polls to completion
Back in `plan-panel.tsx`, TanStack Query polls `GET /plans/jobs/:id` every 1.5s **only
while** status is `queued|running` (see `refetchInterval`). When it flips to `done`, an
effect invalidates the stored-plan query and the schedule renders. Notice the UI never
holds a connection open — it's stateless polling, which scales trivially.

> **This is the whole point of the project.** A fast, stateless HTTP request that *defers*
> expensive work to a durable queue, processed by a separately-scalable pool, with the
> client polling for the result. That's a textbook production async pattern.

---

## 3. The data layer: sqlc + pgx (no ORM)

ACOS deliberately uses **no ORM**. Instead:

1. You write plain SQL in `internal/repository/queries/*.sql` with magic comments:
   ```sql
   -- name: UpsertPlanQueued :one
   INSERT INTO daily_plans (user_id, plan_date, status)
   VALUES ($1, $2, 'queued')
   ON CONFLICT (user_id, plan_date)
   DO UPDATE SET status = 'queued', plan_json = NULL, error = NULL, updated_at = now()
   RETURNING ...;
   ```
2. `make sqlc` reads `sqlc.yaml` and **generates type-safe Go** (`*.sql.go`) — a function
   per query, with a typed params struct and a typed return. The generated files are the
   ones like `plans.sql.go`, `tasks.sql.go`.
3. Your `service` calls those generated methods through the `Querier` interface.

### 🟢 Node → Go: why no ORM (e.g. Prisma/TypeORM)?
- **Compile-time safety without runtime magic.** sqlc parses your SQL *and* your schema,
  so a typo in a column name fails `make sqlc`, not production. Prisma gives you types too,
  but via a generated client + query engine; sqlc is just functions over `pgx`.
- **You write real SQL.** No query-builder dialect to learn, no "how do I express this
  join in the ORM" — you already know SQL.
- **`Querier` is an interface**, so services depend on an abstraction. Tests pass a fake
  (`internal/service/fake_test.go`); real runs pass the pgx-backed impl. This is
  dependency inversion, and it's why the service tests don't need a database.

`pgx/v5` is the Postgres driver (faster + more Postgres-native than `database/sql`).
The connection **pool** is created once in `platform/db/db.go` and shared.

---

## 4. Concurrency: the worker pool (the Go showcase)

This is where Go earns its place. Read `internal/worker/pool.go` alongside this section.

### The design (from the file's own doc comment)
- **One dispatcher goroutine** does the blocking `Claim` and sends each job over an
  **unbuffered channel**.
- **N worker goroutines** (`WORKER_CONCURRENCY=4`) receive from that channel and process.

```go
deliveries := make(chan queue.Delivery)   // unbuffered!
for i := 0; i < p.concurrency; i++ {
    go func(id int) { for d := range deliveries { p.process(...) } }(i)
}
p.dispatch(ctx, deliveries)               // dispatcher runs on the main goroutine
```

### 🟢 Node → Go: why this is special
Node is single-threaded with an event loop; "concurrency" means async I/O, and CPU work
blocks everyone. Go gives you **real parallel goroutines** (cheap, ~KB stacks) plus
**channels** for safe communication. Key things to internalize here:

1. **Unbuffered channel = natural backpressure.** The dispatcher *cannot* claim a new job
   until some worker is ready to receive (`out <- d` blocks). So you never pull more work
   off Redis than you can process. You get flow control for free — no semaphore needed.

2. **Graceful shutdown via `context`.** `cmd/worker`/`cmd/server` build a context with
   `signal.NotifyContext(..., SIGINT, SIGTERM)`. On Ctrl-C the context cancels; the
   dispatcher stops claiming, `close(deliveries)` lets workers finish their *in-flight*
   job, and `wg.Wait()` blocks until they're done. No job is dropped mid-flight.
   > 🟢 Node has no built-in equivalent — you'd hand-roll signal handlers and track
   > in-flight work yourself. In Go, `context.Context` is the standard plumbing for
   > cancellation/deadlines and it threads through every layer.

3. **Detached context for acks.** Look at `process`:
   ```go
   ackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.ackTimeout)
   ```
   The job's own context can be cancelled at shutdown, but you still need to Ack/Retry it.
   `WithoutCancel` makes a child that ignores the parent's cancellation — so cleanup
   completes even while shutting down. This is a subtle, genuinely production-grade detail.

4. **Panics don't kill the pool.** `invoke` wraps the handler in `recover()` and converts a
   panic into a `*PanicError`. One bad job can't take down your worker.
   > 🟢 In Node an unhandled throw in an async handler can crash the process; here a panic
   > is contained to one job and turned into a normal retry.

---

## 5. The queue: durability with plain Redis lists

`internal/queue/queue.go` implements an at-least-once queue with **three lists**:
`acos:jobs` (main), `acos:jobs:processing` (in-flight), `acos:jobs:dead` (dead-letter).

| Operation | Redis command | Why |
|---|---|---|
| `Enqueue` | `LPUSH main` | add to head |
| `Claim` | `BLMOVE main→processing` (blocking) | **atomically** move; if the worker crashes the job is still in `processing`, not lost |
| `Ack` | `LREM processing` | success → forget it |
| `Retry` | `LREM processing` + `LPUSH main` (attempt++), or → `dead` once exhausted | exponential backoff, capped attempts |
| `Recover` | `LMOVE processing→main` on startup | re-queue jobs orphaned by a crash |

### Why this matters (production reasoning)
- **At-least-once delivery**: `BLMOVE` is atomic, so a crash between "pop" and "process"
  doesn't vaporize the job — `Recover()` puts it back on the next boot.
- **Because delivery is at-least-once, handlers must be idempotent.** The plan handler
  upserts by `(user_id, plan_date)`, so re-processing the same job is harmless. **This is
  a rule you must hold in your head whenever you add a new job type.**
- **Dead-letter list** = jobs that failed `maxAttempts` times or have an unknown type. They
  don't loop forever; they're parked for inspection.
- The decision to hand-roll this instead of using `asynq` was deliberate (see
  `docs/DECISIONS.md`) — the *point* of the project is to show you understand the
  primitives, not to hide them behind a library.

---

## 6. The AI layer: typed agents, not raw prompts

`internal/ai/` keeps Claude well-contained:
- `client.go` — wraps the Anthropic SDK; one place that knows the API.
- `prompts.go` — the prompt templates (system + user) as Go strings/builders.
- `schema.go` — the **JSON schema** the model must return.
- `agents.go` — typed methods like "plan the day", "prioritize", "break down a task". Each
  returns a Go struct, not a blob of text.

The production lesson: **treat the LLM like a typed function.** You send structured input,
demand structured JSON output, parse it into a domain type, and validate. The frontend's
Zod schemas (`frontend/lib/schemas.ts`) mirror this on the client — and note
`scheduleItemSchema` uses `.catch("focus")` so an unexpected block type from the model
degrades gracefully instead of crashing the UI. Defensive parsing on both ends.

Model choice is config (`ANTHROPIC_MODEL=claude-haiku-4-5-...`) — a credit-efficient model
for a high-volume background job, swappable without code changes.

---

## 7. Config, errors, and other production habits worth copying

### Typed config (`internal/config`)
Env vars are loaded **once** into a typed `Config` struct and validated at boot. If a
required var is missing, the process exits immediately with a clear error — **fail fast**,
never discover a missing secret three requests deep.
> 🟢 Node habit of reading `process.env.FOO` scattered across files → centralized, typed,
> validated once. Much easier to reason about.

### Sentinel errors + wrapping (`internal/domain/errors.go`)
The codebase defines errors like `domain.ErrValidation`, `domain.ErrNotFound`. Services
return these; handlers map them to HTTP status codes with `errors.Is(...)`. Lower layers
*wrap* with context: `fmt.Errorf("upsert plan: %w", err)`. The `%w` verb preserves the
chain so `errors.Is` still works at the top.
> 🟢 Node → Go: instead of throwing/catching, Go returns `error` as the last value and you
> check it explicitly (`if err != nil`). Verbose, but the control flow is always visible —
> no invisible exception bubbling.

### Structured logging (`log/slog`)
`platform/logger` builds an `slog.Logger`. Logs are key/value (`"job_id", id`), not string
soup — greppable and machine-parseable, which is what real log pipelines want.

### Middleware you should recognize
`RequestID` (trace one request across logs), `Logger` (access logs), `Recovery` (turn a
panic into a 500 instead of crashing the server). Standard production hygiene.

### Graceful HTTP shutdown (`cmd/server/main.go`)
On signal: `srv.Shutdown(ctx)` with a 10s timeout — stops accepting new connections, lets
in-flight requests finish. Paired with the worker's graceful drain, the whole system stops
cleanly. This is what lets you deploy/restart without dropping user requests.

---

## 8. The frontend, briefly (where the production thinking is)

- **App Router + `"use client"`** islands. Server components by default; interactive panels
  opt into the client.
- **TanStack Query** owns *server state* — caching, polling, invalidation. You saw it drive
  the plan-status poll declaratively (poll *because* status is in-progress, not via a
  manual `setInterval` flag). Study `plan-panel.tsx` for this pattern.
- **Zod** validates every API response at the boundary, so bad data fails loudly at one
  spot instead of producing `undefined` deep in a component.
- **The `/api/*` rewrite** (`next.config.ts`) means the browser only ever talks to its own
  origin → no CORS, and the backend URL is a deploy-time concern, not baked into the bundle.
- **JWT in localStorage** (`lib/auth.tsx`) — simple for a portfolio app. (In a hardened
  prod app you'd weigh httpOnly cookies for XSS resistance — a good thing to know the
  tradeoff of.)

---

## 9. Testing strategy (look at the `_test.go` files)

Go keeps tests **next to the code** (`pool_test.go` beside `pool.go`). ACOS has three tiers:
- **Unit** (fast, no I/O): services tested against fakes (`service/fake_test.go`,
  `service/*_test.go`). Run with `make test-unit` (`-short`).
- **Integration**: `repository_test.go`, `queue_test.go` spin up real Postgres/Redis via
  **testcontainers** (needs Docker). This tests the actual SQL and Redis behavior, not a
  mock of it.
- **Live**: `ai/live_test.go` hits the real Claude API (gated, opt-in — costs credits).

> 🟢 Node → Go: no Jest/Mocha — testing is in the standard library (`testing` package +
> `go test`). Table-driven tests are the idiom. `-short` is the convention for skipping
> slow/integration tests.

---

## 10. A study plan (do this, in order)

1. **Run it and break it.** You already have it running. In Adminer, watch
   `daily_plans.status` flip queued→running→done as you click Generate.
2. **Trace one request** with the file map in §2 open. Put a `log.Info` in the handler,
   service, and worker handler; watch the order they fire (and that the last two are in a
   *different process*).
3. **Read `pool.go` line by line.** It's the densest, most valuable file. Re-read §4 after.
4. **Add a feature** to cement it. Suggested: a `weekly report` job — new migration, new
   `.sql` query (`make sqlc`), a service method, an HTTP route, a new worker handler, and a
   panel. You'll touch every layer exactly once. (This is literally Batch 6 in the tracker.)
5. **Deliberately fail a job.** `make enqueue n=3 fail=1` and watch the retry → backoff →
   dead-letter path in the worker logs and the `acos:jobs:dead` Redis list.

### Companion docs already in this repo
| Doc | Read it for |
|---|---|
| `docs/ARCHITECTURE.md` | the system design in more detail |
| `docs/DECISIONS.md` | *why* sqlc/custom-queue/JWT were chosen (the tradeoffs) |
| `docs/AI_DESIGN.md` | the Claude agents, prompts, and JSON contracts |
| `docs/API.md` | every REST endpoint |
| `docs/SETUP.md` | environment setup details |
| `docs/TRACKER.md` | what's done and what's next |

---

## 11. The ten ideas to walk away with

1. **`cmd/` + `internal/`** = many binaries, one shared private library.
2. **Layered, inward-pointing dependencies**; `domain` depends on nothing.
3. **Interfaces at the seams** (`Querier`, `Enqueuer`) make everything testable.
4. **Fast HTTP + deferred work via a durable queue** is the core scaling pattern.
5. **Goroutines + an unbuffered channel** give you a worker pool with built-in backpressure.
6. **`context.Context`** threads cancellation/timeouts through every layer; it's how Go does
   graceful shutdown.
7. **At-least-once delivery ⟹ idempotent handlers.** Always.
8. **Explicit errors with `%w` wrapping + sentinel `errors.Is`** beats invisible exceptions.
9. **sqlc**: real SQL, compile-time-checked, no ORM magic.
10. **Validate at every boundary** (Zod on the client, typed structs + schema on the server,
    parsed LLM output) so bad data dies early and loudly.
```
