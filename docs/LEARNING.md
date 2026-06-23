# 📚 Learning ACOS — A Beginner's Full Walkthrough (Go + Next.js + Claude)

> **Who this is for:** you — a Node/Next developer who just learned Go and wants to *understand*
> this project deeply, not just run it. This doc explains **what** each part does, **why** it's
> built that way, and **how the pieces talk to each other**. Where Go differs from the Node mental
> model, it's flagged: **🟢 Node → Go**.
>
> **How to read it:** skim §0–§2 for the big picture, then read §7 (AI) and §8 (UI↔backend) slowly —
> those are the two things you asked about. §13 is a plain-English glossary; jump there any time a
> word is unfamiliar.

---

## 0. The 30-second mental model

```
                      ┌──────────────────────────────────────────────┐
   Browser            │                  Next.js (port 3000)          │
   (you click)  ─────▶│  React + TanStack Query + Zod                 │
                      │  proxies /api/* ──▶ backend (so: no CORS)     │
                      └───────────────────────┬──────────────────────┘
                                              │ HTTP request + JWT token
                      ┌───────────────────────▼──────────────────────┐
                      │           Go API server (port 8080)           │
                      │  Gin → middleware → handler → service → repo  │
                      └─────┬─────────────────────┬───────────────────┘
                            │ SQL (pgx/sqlc)      │ LPUSH job (heavy AI work)
                  ┌─────────▼────────┐   ┌────────▼────────┐
                  │  PostgreSQL 17   │   │     Redis 7     │
                  │ (source of truth)│   │ (durable queue) │
                  └─────────▲────────┘   └────────┬────────┘
                            │ save result          │ BLMOVE (claim job)
                      ┌─────┴──────────────────────▼─────────────────┐
                      │      Go worker pool (a SEPARATE binary)       │
                      │  dispatcher + N goroutines → Claude (AI)      │
                      └───────────────────────────────────────────────┘
```

There are **two running Go programs** that share one codebase:

- **`cmd/server`** — answers HTTP requests fast. Never does slow work itself.
- **`cmd/worker`** — sits in a loop pulling "jobs" off Redis and doing the slow AI work.

Plus **Postgres** (permanent data) and **Redis** (a to-do list of background jobs). The Next.js app
is the UI. Hold this picture in your head; everything below is a zoom-in on one box.

---

## 1. Why two binaries? (the single most important idea)

Generating a daily plan calls Claude, which takes ~2–5 seconds. If the HTTP request did that itself,
the user's browser would hang for 5 seconds and your server could only handle a handful of people at
once.

So instead:

1. The **server** writes a "please generate a plan" note onto a Redis list and **immediately** replies
   "OK, it's queued" (milliseconds).
2. The **worker** — a different process — picks up that note, calls Claude, and saves the result to
   Postgres.
3. The browser **polls** ("is it done yet?") every 1.5s until the plan appears.

This is the classic **"fast request, deferred work"** pattern. It's the spine of the whole project.
Both programs `import` the same `internal/` packages, so there's zero code duplication —
`cmd/server/main.go` and `cmd/worker/main.go` are just two different "wirings" of the same Lego bricks.

> 🟢 **Node → Go:** in Node you'd reach for BullMQ + a separate worker script. Same idea here, but the
> queue is hand-built on Redis (to *show* the mechanics) and the worker is a compiled Go binary.

---

## 2. The backend, part by part

Everything real lives in `backend/internal/`. `internal/` is a **magic folder name** in Go: the
compiler forbids any code outside this module from importing it. It's privacy enforced by the
compiler, not by politeness.

Here's each package, what it's responsible for, and the one concept it teaches.

```
backend/
├── cmd/                ENTRYPOINTS — each subfolder is one runnable program (`package main`)
│   ├── server/         the HTTP API           → go run ./cmd/server
│   ├── worker/         the background processor→ go run ./cmd/worker
│   ├── migrate/        applies DB schema changes
│   └── enqueue/        a dev toy to push fake jobs and watch the queue
│
└── internal/
    ├── config/         reads .env → one typed Config struct, validated at boot
    ├── domain/         the pure "nouns" of the app (Task, Plan, Habit, Stats) + error values.
    │                   Imports NOTHING from the app — it's the stable core everyone depends on.
    ├── platform/       thin wrappers around infrastructure:
    │   ├── db/         creates the Postgres connection pool (pgxpool)
    │   ├── redis/      creates the Redis client
    │   └── logger/     creates the structured logger (slog) + request-id helpers
    ├── repository/     DATA ACCESS. Your .sql files + the Go that sqlc GENERATES from them.
    ├── service/        BUSINESS LOGIC. The "brain" — orchestrates repo + queue + ai.
    ├── http/           TRANSPORT. Gin router, handlers (HTTP↔service), middleware.
    ├── queue/          the Redis-list job queue primitive (Enqueue/Claim/Ack/Retry…)
    ├── worker/         the goroutine worker pool + the job handlers
    ├── ai/             the Claude client + the 4 "agents" + prompts + JSON schemas
    └── auth/           JWT token signing/parsing
```

### The layering rule (memorize this one picture)

Dependencies only point **inward / downward**, never back up:

```
http (handlers) ──▶ service ──▶ repository ──▶ Postgres
                       │
                       ├──▶ queue ──▶ Redis
                       └──▶ ai    ──▶ Claude
                    everyone may import ▶ domain  (domain imports nobody)
```

What this buys you, concretely:
- A **handler** never runs SQL. It calls a **service**. So you could swap Gin for another router and
  the business logic wouldn't notice.
- A **service** never speaks HTTP (no status codes, no JSON). It takes plain Go arguments and returns
  `domain` types or an `error`. So the *same* `PlanService` is used by both the server (to enqueue)
  and the worker (to save results).
- `domain` depends on nothing, so it can never cause an import cycle and is trivial to test.

> 🟢 **Node → Go:** Express apps often blur routes/controllers/models. Go shops lean hard on this
> layered, one-direction dependency rule because it makes large codebases stay testable and swappable.

### A note on each layer's "shape"

- **handler** (e.g. `internal/http/handler/task.go`): *thin*. Decode JSON → call service → translate
  the result to an HTTP status + JSON. ~3 steps, no logic.
- **service** (e.g. `internal/service/task.go`): *the logic*. Validation, defaults, deciding what to
  do, mapping DB rows → domain types. Returns `domain.ErrValidation` etc. on bad input.
- **repository** (`internal/repository/*.sql.go`): *generated*. One Go function per SQL query.

---

## 3. The request lifecycle — trace ONE call end-to-end

Follow **"Generate plan"** — it touches every layer and the async pipeline.

```
[Browser] click "Generate plan"
   │  api.generatePlan(token, {...})           frontend/lib/api.ts
   ▼
POST /api/plans/generate                        (Next.js proxy → :8080/plans/generate)
   │
   ▼  RequestID → Logger → Recovery → Auth → handler     internal/http/router.go
[Handler] GeneratePlan                          internal/http/handler/plan.go
   │   1. validate JSON body (date, minutes, mode…)
   │   2. read userID from context (the Auth middleware put it there)
   │   3. call service ──┐
   ▼                     ▼
[Service] PlanService.Generate                  internal/service/plan.go
   │   1. UpsertPlanQueued → write daily_plans row, status='queued'   (Postgres)
   │   2. Enqueue a Job (LPUSH) onto Redis                            (the plan id IS the job id)
   │   3. return the queued plan immediately  ── HTTP responds 202 in ~5ms
   ▼
... (the request is already DONE; the rest happens in the other process) ...

[Worker] dispatcher is blocked on queue.Claim (BLMOVE)  internal/worker/pool.go
   │   job arrives → handed to a free goroutine
   ▼
[Worker] PlanHandler                            internal/worker/planjob.go
   │   1. MarkRunning  → status='running'
   │   2. load the user's pending tasks (service)
   │   3. agents.Plan(...) → call Claude, get a Schedule   internal/ai
   │   4. Complete → status='done' + store plan_json       (or Fail → status='failed')
   ▼
[Postgres] daily_plans row now status='done' with the schedule JSON

[Browser] TanStack Query polls GET /api/plans/jobs/:id every 1.5s
   │   while status is queued|running → keep polling
   ▼   status flips to 'done' → stop polling, render the schedule
```

The key insight: **steps after "return the queued plan" run in a different program, minutes-of-CPU
later, while the user's HTTP request already finished.** That decoupling is the whole point.

---

## 4. The data layer: migrations + sqlc + pgx (no ORM)

Three pieces work together so you write **real SQL** but call it with **type-safe Go**.

**(a) Migrations** (`backend/migrations/*.sql`) — versioned schema changes. `000001_init.up.sql`
creates `users`/`tasks`/`daily_plans`; `000002_habits.up.sql` adds the habit tables. `make migrate-up`
applies them in order and records which ran, so the DB schema is reproducible from zero.
> *Migration = a numbered SQL file that changes the database structure. `.up` applies it, `.down`
> reverses it.*

**(b) sqlc** turns SQL into Go. You write a query with a magic comment:
```sql
-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1 AND user_id = $2;
```
`make sqlc` reads your schema + queries and **generates** `tasks.sql.go` with:
```go
func (q *Queries) GetTask(ctx context.Context, arg GetTaskParams) (Task, error)
```
A typo in a column name fails `make sqlc` on your laptop — not in production. You get ORM-like safety
with zero ORM runtime.

**(c) pgx** is the Postgres driver. `platform/db/db.go` creates **one connection pool** at startup and
shares it. A pool keeps a handful of open connections so each query doesn't pay connect cost.

### The `Querier` interface (why services are testable)

sqlc also generates an **interface** `Querier` listing every query method. Services depend on that
interface, not the concrete DB type:
```go
type TaskService struct { repo repository.Querier }   // ← an interface, not a *pgx pool
```
In production you pass the real pgx-backed implementation. In **tests** you pass a fake map-backed one
(`internal/service/fake_test.go`). That's why service tests run in milliseconds with **no database**.
This is *dependency inversion* — the single most useful OO idea in the whole backend.

> 🟢 **Node → Go:** like swapping a real Prisma client for a mock, but the "interface" is a real
> language feature the compiler checks, and the fake is plain Go you can read.

---

## 5. Concurrency: the worker pool (Go's showcase)

Read `internal/worker/pool.go` next to this section.

```go
deliveries := make(chan queue.Delivery)            // an UNBUFFERED channel
for i := 0; i < p.concurrency; i++ {               // start N workers (default 4)
    go func(id int) { for d := range deliveries { p.process(...) } }(i)
}
p.dispatch(ctx, deliveries)                        // 1 dispatcher feeds the channel
```

- A **goroutine** is a function running concurrently — extremely cheap (~KB), so thousands are fine.
- A **channel** is a typed pipe goroutines use to hand work to each other safely (no shared-memory
  locks).
- **One dispatcher** goroutine does the blocking Redis `Claim` and sends each job into the channel;
  **N workers** receive and process.

Four production details worth internalizing:

1. **Unbuffered channel = free backpressure.** `out <- d` *blocks* until some worker is ready to
   receive. So the dispatcher can't pull more jobs off Redis than the pool can handle. Flow control
   for free, no semaphore.
2. **Graceful shutdown via `context`.** Both mains use `signal.NotifyContext(..., SIGINT, SIGTERM)`.
   Ctrl-C cancels the context → dispatcher stops → `close(deliveries)` lets workers finish their
   *in-flight* job → `wg.Wait()` blocks until they're done. No job dropped mid-flight.
3. **Detached ack context.** `context.WithoutCancel(ctx)` makes a child context that ignores the
   shutdown signal, so a job that finishes *during* shutdown can still be marked done in Redis.
4. **Panic recovery.** `invoke()` wraps each handler in `recover()`, turning a crash into a normal
   retry. One poison job can't kill the pool.

> 🟢 **Node → Go:** Node is single-threaded; CPU work blocks everyone. Go gives real parallel
> goroutines + channels. `context.Context` is the standard way to thread cancellation/timeouts through
> every function — there's no Node equivalent baked into the language.

---

## 6. The queue: durability with three Redis lists

`internal/queue/queue.go` is an **at-least-once** queue using three lists per namespace:
`acos:jobs` (waiting), `acos:jobs:processing` (claimed/in-flight), `acos:jobs:dead` (gave up).

| Operation | Redis command | Why it's done this way |
|---|---|---|
| `Enqueue` | `LPUSH main` | add a job to the front |
| `Claim` | `BLMOVE main→processing` (blocking) | **atomically** move; if the worker dies after claiming, the job is still in `processing` — not lost |
| `Ack` | `LREM processing` | success → forget it |
| `Retry` | move back to `main` with attempt+1, or to `dead` once attempts run out | bounded retries with backoff |
| `Recover` | `LMOVE processing→main` at startup | re-queue jobs orphaned by a past crash |

Two rules fall out of this design:
- **Because delivery is at-least-once, every job handler must be idempotent** (safe to run twice). The
  plan handler upserts by `(user_id, plan_date)`, so re-running it just overwrites — harmless.
- A **dead-letter list** means failures are *parked for inspection* instead of looping forever.

Try it: `make enqueue n=3 fail=1` then watch the worker logs retry → back off → dead-letter, and check
`redis-cli LLEN acos:jobs:dead`.

---

## 7. ★ The AI layer — how Claude is wired in (read slowly)

This is the part you asked about most. The whole philosophy: **treat the LLM like a typed function.**
You send structured input, *demand* JSON back, parse it into a Go struct, and validate it. Free text
never escapes this package.

Everything is in `internal/ai/`:

```
ai/
├── client.go    the Completer interface + the real Claude client (the ONLY file that knows the SDK)
├── agents.go    the 4 "agents" (Plan / Prioritize / Break / Report) + the repair loop
├── prompts.go   the system prompts (versioned) + functions that build the user message
└── schema.go    the Go structs Claude must return + their Validate() rules + JSON parsing helpers
```

### 7.1 The `Completer` interface — the key abstraction

```go
// client.go
type Completer interface {
    Complete(ctx context.Context, system, user string) (string, error)
}
```

That's the entire surface the rest of the code needs: "give me a system prompt + a user prompt, get
back text." Two things implement/consume it:

- The **real `Client`** (`client.go`) implements `Complete` by calling the Anthropic Go SDK.
- In **tests**, a tiny fake implements `Complete` by returning canned JSON — so agent logic is tested
  with **no API key and no network** (`internal/ai/agents_test.go`).

```go
type Client struct {
    api       anthropic.Client   // the official SDK
    model     anthropic.Model    // e.g. claude-haiku-4-5 (from config)
    maxTokens int64
    timeout   time.Duration
}

func (c *Client) Complete(ctx, system, user string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, c.timeout)   // never hang forever
    defer cancel()
    resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
        Model: c.model, MaxTokens: c.maxTokens,
        System:   []anthropic.TextBlockParam{{Text: system}},
        Messages: []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
    })
    // ... concatenate the text blocks of the response and return the string
}
```

> 🟢 **Node → Go:** this is the same idea as injecting a mock OpenAI client, but the "interface" is a
> first-class language feature, so the fake is guaranteed to match the real shape at compile time.

### 7.2 The 4 agents — typed in, typed out

`Agents` is just a struct wrapping a `Completer`:
```go
type Agents struct { c Completer }
```
Each method is a real, typed function:

| Agent | Method | Returns | Used by |
|---|---|---|---|
| **Planner** | `Plan(ctx, PlanInput)` | `Schedule` | the **worker** (async) |
| **Priority** | `Prioritize(ctx, []Task)` | `PriorityResult` | the **server** (sync) |
| **Breakdown** | `Break(ctx, Task)` | `Breakdown` | the **server** (sync) |
| **Reporter** | `Report(ctx, ReportInput)` | `WeeklyReport` | the **server** (sync) |

Each output is a plain Go struct in `schema.go`, e.g.:
```go
type Schedule struct {
    Date     string         `json:"date"`
    Schedule []ScheduleItem `json:"schedule"`   // each: time, task, type(focus|rest|admin…)
    Summary  string         `json:"summary"`
}
func (s Schedule) Validate() error { /* date present? at least one block? valid type? */ }
```

### 7.3 The strict-JSON + repair loop (the clever bit)

LLMs *usually* return valid JSON when asked, but not always — sometimes they wrap it in ```` ```json ````
fences or add a sentence. So every agent runs this loop (`agents.go`):

```go
func (a *Agents) runWithRepair(ctx, system, user string, parse func(raw string) error) error {
    raw, err := a.c.Complete(ctx, system, user)        // 1. ask Claude
    if err != nil { return err }
    if perr := parse(raw); perr == nil { return nil }  // 2. try to parse+validate → success
    // 3. ONE repair attempt: tell Claude what was wrong and ask again
    raw2, err := a.c.Complete(ctx, system, repairUser(user, raw, perr))
    if err != nil { return err }
    return parse(raw2)                                  // 4. parse the corrected reply
}
```

And `parse` is `parseStrict` from `schema.go`:
```go
func parseStrict[T validatable](raw string, v *T) error {
    js := extractJSON(raw)                  // strip ```json fences / prose, keep { ... }
    if err := json.Unmarshal([]byte(js), v); err != nil { return err }  // text → struct
    return (*v).Validate()                  // business rules (non-empty, valid enum, …)
}
```

So the pipeline for every agent is:

```
build prompt → Claude → extractJSON → json.Unmarshal → Validate
                                   └─ if any step fails ─▶ one repair round-trip ─▶ try again
                                                                              └─ still bad ▶ return error
```

That's the production lesson: **never trust raw model text** — fence-strip, unmarshal into a typed
struct, validate, and have a fallback. (The frontend mirrors this with Zod; §8.)

### 7.4 Prompts are versioned Go strings

`prompts.go` holds each system prompt as a constant with a version (`planner-v1`, `reporter-v1`…), plus
builder functions that turn Go data into the user message:
```go
func buildPlannerUser(in PlanInput) string {
    s := fmt.Sprintf("Date: %s\nAvailable time: %d\nGoals: %s\nTasks: %s", ...)
    if g := modeGuidance(in.Mode); g != "" { s += "\nPlanning mode: " + g }  // ← focus modes!
    return s
}
```
`modeGuidance` is how **Focus modes** work: `deep_focus` / `stress_relief` / `light` each append a
different instruction to the prompt. No new agent — just a different sentence appended.

### 7.5 Sync vs async — two ways the same agents get called

This is the integration question. The exact same `ai.Agents` is wired in two places:

- **Async (daily plan):** heavy + benefits from retries/durability → goes through the **queue**. The
  **worker** builds the agents and calls `agents.Plan(...)` inside `PlanHandler`.
- **Sync (prioritize / breakdown / weekly report):** fast, one-shot, results are ephemeral (shown in
  the UI, not stored) → the **server** builds the agents and calls them *inside the HTTP request*. The
  browser shows a spinner for ~2s and gets the answer directly.

Both `cmd/server/main.go` and `cmd/worker/main.go` do the same setup:
```go
if cfg.AnthropicAPIKey != "" {
    client, _ := ai.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
    agents = ai.NewAgents(client)
}
// pass `agents` into the service. If the key is missing, agents is nil →
// the AI endpoints return 503 "unavailable" instead of crashing.
```
Why split this way? See `docs/DECISIONS.md` **ADR-011**: queue the heavy job; let cheap interactive
calls answer directly. The "never block on AI" rule is about not coupling *cheap CRUD* to AI, not about
forbidding a dedicated AI endpoint from awaiting its own result.

**The model is config**, not code: `ANTHROPIC_MODEL=claude-haiku-4-5-…` — a credit-efficient model,
swappable without recompiling.

---

## 8. ★ How the frontend talks to the backend (the round trip)

The second thing you asked about. Five pieces cooperate.

### 8.1 The proxy — why there's no CORS

`frontend/next.config.ts`:
```ts
async rewrites() {
  return [{ source: "/api/:path*", destination: "http://localhost:8080/:path*" }];
}
```
The browser only ever calls **its own origin** (`localhost:3000/api/...`). Next.js forwards those to
the Go backend server-side. So the browser never makes a cross-origin call → **no CORS headers to
configure**, and the backend URL is a deploy-time setting, not baked into the JS bundle.
> *CORS = the browser's rule that a page on site A can't freely call site B. The proxy sidesteps it by
> making everything look like "site A."*

### 8.2 The typed API client — one function for every call

`frontend/lib/api.ts` has a single `request()` helper and then one tiny method per endpoint:
```ts
async function request<T>(path, { method, body, token, schema }) {
  const res = await fetch(`/api${path}`, {
    method,
    headers: { "Content-Type": "application/json", Authorization: token && `Bearer ${token}` },
    body: body && JSON.stringify(body),
  });
  if (!res.ok) {                                   // backend sent an error envelope
    const { error } = await res.json();            // { error: { code, message } }
    throw new ApiError(res.status, error.code, error.message);
  }
  const json = await res.json();
  return schema ? schema.parse(json) : json;       // ← Zod validates the response shape
}

export const api = {
  generatePlan: (t, input) => request("/plans/generate", { method:"POST", body:input, token:t, schema: planJobSchema }),
  stats:        (t)        => request("/stats", { token:t, schema: statsSchema }),
  prioritize:   (t)        => request("/ai/prioritize", { method:"POST", token:t, schema: priorityResultSchema }),
  // …one line per endpoint
};
```
Two things to notice:
- **Auth token** is attached as `Authorization: Bearer <jwt>` on every call.
- The backend's **error envelope** is one consistent shape `{ "error": { "code", "message" } }`
  (built in `internal/http/handler/handler.go`), so the client maps any failure to one `ApiError`.

### 8.3 Zod — validate responses at the boundary

`frontend/lib/schemas.ts` declares the expected shape of every response, and `request()` runs
`schema.parse(json)`. If the backend ever sends an unexpected shape, it fails **loudly at one spot**
instead of becoming `undefined` deep inside a component. It's the client-side mirror of the server's
`Validate()` — defensive parsing on *both* ends of the wire.

### 8.4 TanStack Query — caching, polling, invalidation

React components don't call `api.*` directly in `useEffect`. They use **TanStack Query**, which owns
"server state": caching, loading flags, refetching, and polling. Two primitives:

- **`useQuery`** = "read something and cache it":
  ```ts
  const tasksQuery = useQuery({ queryKey: ["tasks"], queryFn: () => api.listTasks(token) });
  ```
- **`useMutation`** = "change something, then refresh":
  ```ts
  const create = useMutation({
    mutationFn: (input) => api.createTask(token, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),  // ← re-fetch the list
  });
  ```

**Polling is declarative.** The plan panel polls the job *only while it's running*:
```ts
const jobQuery = useQuery({
  queryKey: ["plan-job", planId],
  queryFn: () => api.getPlanJob(token, planId),
  enabled: !!planId && inProgress,
  refetchInterval: (q) => {
    const s = q.state.data?.status;
    return s === "queued" || s === "running" ? 1500 : false;   // poll, or stop
  },
});
```
No manual `setInterval`, no "isPolling" boolean — you describe *when* to poll and the library does it.
That's the same async result the worker produced in §3, surfaced in the UI.

### 8.5 The auth token flow (login → every request → logout)

```
register/login form ─POST /api/auth/register─▶ backend returns { user, token (JWT) }
        │
        ▼  signIn({token, user})                     frontend/lib/auth.tsx
   localStorage["acos.auth"] = {token, user}
        │
        ▼  useAuth() reads it via useSyncExternalStore (SSR-safe, no flash)
   every api.* call attaches  Authorization: Bearer <token>
        │
        ▼  middleware.Auth verifies the JWT, puts userID in the request context
   handlers read userID → every query is scoped to that user
        │
        ▼  signOut() clears localStorage → all calls now 401 → guards redirect to /login
```

`auth.tsx` keeps the session in `localStorage` and exposes it with **`useSyncExternalStore`** (React's
official way to read external mutable state) so there's no hydration flash and no `AuthProvider`
needed. Pages guard themselves: the dashboard redirects to `/login` if there's no token.

---

## 9. A feature tour — see the pattern repeat

Every feature is the *same layered path*. Once you see it 3 times it's automatic.

| Feature | DB | Service | HTTP | Frontend | AI? |
|---|---|---|---|---|---|
| **Tasks CRUD** | `tasks` table | `TaskService` | `/tasks…` | `task-panel.tsx` | no |
| **Daily plan** | `daily_plans` (+ queue) | `PlanService` + worker `PlanHandler` | `/plans/generate`, `/plans/jobs/:id` | `plan-panel.tsx` (polls) | **Planner** (async) |
| **Prioritize** | reads `tasks` | `AIService.Prioritize` | `POST /ai/prioritize` | Prioritize button + rank badges | **Priority** (sync) |
| **Breakdown** | reads one task | `AIService.Breakdown` | `POST /ai/breakdown/:id` | per-task expander | **Breakdown** (sync) |
| **Analytics** | SQL aggregation over `tasks`/`daily_plans` | `StatsService` | `GET /stats` | Insights panel (ring, bars) | no |
| **Focus modes** | (none) | `mode` flows in the job payload | `mode` on `/plans/generate` | mode selector | reshapes Planner prompt |
| **Weekly report** | reads stats + tasks | `AIService.WeeklyReport` | `POST /ai/weekly-report` | "Weekly review" card | **Reporter** (sync) |
| **Habits** | `habits` + `habit_checkins` | `HabitService` (streak logic) | `/habits…`, `/habits/:id/checkin` | streak grid `habits-panel.tsx` | no |

Two are worth a closer look:

- **Analytics** shows off *SQL doing the work*: one aggregation query with `COUNT(*) FILTER (WHERE …)`
  and a `GROUP BY day` returns counts, the priority mix, and a 7-day trend. The Go service just
  zero-fills missing days and computes the percentage. (`internal/repository/queries/stats.sql`)
- **Habits** is a full new domain: two tables, a check-in is a row in `habit_checkins (habit_id, day)`
  with a unique key (so checking twice is harmless). The **streak** is computed in Go
  (`currentStreak` in `service/habit.go`): count consecutive days backward from today (a streak stays
  "alive" until today is over). The grid UI just colors the days that have a check-in.

---

## 10. Production habits worth copying (the small stuff that matters)

- **Typed config, validated at boot** (`internal/config`): all env in one struct; a missing required
  var exits immediately with a clear message — *fail fast*, don't discover it three requests deep.
- **Sentinel errors + `%w` wrapping** (`domain/errors.go`): services return `domain.ErrNotFound` etc.;
  lower layers wrap with context (`fmt.Errorf("get task: %w", err)`); the handler maps them to status
  codes with `errors.Is`. `%w` keeps the chain intact.
  > 🟢 **Node → Go:** instead of throw/catch, Go returns `error` as the last value and you check
  > `if err != nil`. Verbose, but the control flow is always visible — no invisible exception bubbling.
- **Structured logging** (`log/slog`): logs are key/value (`"plan_id", id`), greppable and
  machine-parseable.
- **Middleware** (`internal/http/middleware`): `RequestID` (trace one request across logs), `Logger`
  (access logs), `Recovery` (panic → 500, not a crash), `Auth` (verify JWT, set userID).
- **Graceful HTTP shutdown** (`cmd/server/main.go`): on signal, `srv.Shutdown(ctx)` stops accepting
  new connections and lets in-flight requests finish — deploy/restart without dropping requests.

---

## 11. Auth, explained simply (JWT)

A **JWT** (JSON Web Token) is a signed string that says "this is user X" without the server storing a
session. Flow:

1. You log in → the server checks your password (bcrypt-hashed in `users.password_hash`) and, if good,
   **signs** a token containing your user id with a secret key (`auth/token.go`, `JWT_SECRET`).
2. The browser stores that token and sends it on every request (`Authorization: Bearer …`).
3. `middleware.Auth` **verifies the signature** (proving the token wasn't forged) and extracts the
   user id into the request context. Handlers read it, so every DB query is automatically scoped to
   *your* data.

Because the token is signed (not encrypted), the server doesn't need to remember sessions — it just
re-verifies the signature each time. That's *stateless auth*. (Trade-off: storing it in `localStorage`
is simple but XSS-exposed; an httpOnly cookie is more hardened — a good tradeoff to know.)

---

## 12. Testing strategy

Go keeps tests **next to the code** (`pool_test.go` beside `pool.go`). Three tiers:
- **Unit** (fast, no I/O): services against fakes (`service/*_test.go`, `service/fake_test.go`).
  `make test-unit` (`-short`).
- **Integration** (real infra via **testcontainers**, needs Docker): `repository_test.go` runs the
  actual SQL against a throwaway Postgres; `queue_test.go`/`pool_test.go` use a fake Redis.
- **Live** (opt-in, costs credits): `ai/live_test.go` hits the real Claude API; skipped unless a key is
  present.

`make test` runs everything (45 tests today). `go test ./...` is the whole command — no Jest/Mocha;
testing is in Go's standard library.

---

## 13. Glossary (plain-English, in one place)

- **goroutine** — a function running concurrently; very cheap, so you can have thousands.
- **channel** — a typed pipe goroutines use to pass data safely without locks.
- **`context.Context`** — a value threaded through calls carrying cancellation + deadlines; how Go
  does timeouts and graceful shutdown.
- **interface** — a list of method names; any type with those methods "satisfies" it. Enables fakes
  in tests (`Querier`, `Completer`, `Enqueuer`).
- **idempotent** — running it twice has the same effect as once (required because the queue may deliver
  a job more than once).
- **migration** — a numbered SQL file that changes the DB structure; applied in order, reversible.
- **sqlc** — a tool that generates type-safe Go functions from your `.sql` files.
- **pgx / pool** — the Postgres driver; the pool keeps a few open connections to reuse.
- **JSONB** — Postgres's binary JSON column type; the generated plan is stored as JSONB.
- **`BLMOVE`** — a Redis command that atomically moves an item between lists, blocking until one
  exists; the heart of the reliable queue.
- **at-least-once** — the queue guarantees a job runs *at least* once (maybe more) → handlers must be
  idempotent.
- **dead-letter** — a list where jobs that failed too many times are parked for inspection.
- **middleware** — a function that wraps every request (logging, auth, recovery) before the handler.
- **JWT** — a signed token proving who you are without server-side sessions.
- **CORS** — the browser rule blocking cross-origin calls; sidestepped here by the `/api` proxy.
- **TanStack Query** — the React library managing server data (cache/poll/invalidate).
- **Zod** — a TypeScript library that validates a value matches an expected shape at runtime.
- **hydration** — React reconciling server-rendered HTML with client JS on first load.

---

## 14. A study plan (do these in order)

1. **Run it and watch the DB.** Open Adminer (`localhost:8081`) and watch `daily_plans.status` flip
   `queued → running → done` as you click **Generate plan**.
2. **Trace one request** with §3 open. Add a `log.Info` in the handler, the service, and the worker
   handler; notice the last two fire in a *different process*.
3. **Read `internal/worker/pool.go` and `internal/ai/agents.go` line by line** — the two densest,
   highest-value files. Re-read §5 and §7 after.
4. **Break a job on purpose:** `make enqueue n=3 fail=1` → watch retry → backoff → dead-letter in the
   worker logs and `redis-cli LLEN acos:jobs:dead`.
5. **Add a tiny feature** to cement the pattern (e.g. a "notes" field on a task): migration → `.sql`
   query → `make sqlc` → service method → HTTP route → a frontend input. You'll touch every layer once.

### Companion docs
| Doc | Read it for |
|---|---|
| `docs/ARCHITECTURE.md` | the system design, more formally |
| `docs/DECISIONS.md` | *why* each choice was made (11 ADRs incl. sync-vs-async AI) |
| `docs/AI_DESIGN.md` | the agents, prompts, JSON contracts |
| `docs/API.md` | every REST endpoint with examples |
| `docs/SETUP.md` | environment + run instructions |
| `docs/TRACKER.md` | what's built (Batches 0–7) and what's left |

---

## 15. The dozen ideas to walk away with

1. **`cmd/` + `internal/`** = many binaries, one shared private library.
2. **Layered, inward-pointing dependencies**; `domain` depends on nothing.
3. **Interfaces at the seams** (`Querier`, `Completer`, `Enqueuer`) make everything testable.
4. **Fast HTTP + deferred work via a durable queue** is the core scaling pattern.
5. **Goroutines + an unbuffered channel** = a worker pool with built-in backpressure.
6. **`context.Context`** threads cancellation/timeouts through every layer.
7. **At-least-once delivery ⟹ idempotent handlers.** Always.
8. **Explicit errors with `%w` + `errors.Is`** beat invisible exceptions.
9. **sqlc**: real SQL, compile-time-checked, no ORM magic.
10. **Treat the LLM as a typed function:** structured prompt → JSON → struct → validate → repair.
11. **Same agents, two integrations:** queue the heavy plan, await the cheap interactive calls.
12. **Validate at every boundary** (Zod on the client, typed structs + `Validate()` on the server,
    parsed LLM output) so bad data dies early and loudly.
```
