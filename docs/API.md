# 🌐 API Reference

Base URL (dev): `http://localhost:8080`
All request/response bodies are JSON. Authenticated routes require `Authorization: Bearer <token>`.

## Conventions

**Error envelope** (all non-2xx):
```json
{ "error": { "code": "not_found", "message": "task not found", "details": null } }
```

| code | HTTP |
|---|---|
| `validation_error` | 400 |
| `unauthorized` | 401 |
| `forbidden` | 403 |
| `not_found` | 404 |
| `conflict` | 409 |
| `internal` | 500 |

---

## System

### `GET /healthz`
Liveness. → `200 {"status":"ok"}`

### `GET /readyz`
Readiness — pings Postgres + Redis. → `200 {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}`
or `503` with failing checks.

---

## Auth  *(Batch 1)*

### `POST /auth/register`
```json
{ "name": "Rafay", "email": "rafay@example.com", "password": "supersecret" }
```
→ `201 { "user": { "id", "name", "email", "created_at" }, "token": "<jwt>" }`

### `POST /auth/login`
```json
{ "email": "rafay@example.com", "password": "supersecret" }
```
→ `200 { "user": {...}, "token": "<jwt>" }`

### `GET /me`  🔒
→ `200 { "user": { "id", "name", "email", "created_at" } }`

---

## Tasks  🔒  *(Batch 1)*

Task object:
```json
{
  "id": "uuid",
  "title": "Learn Go concurrency",
  "description": "channels + worker pools",
  "priority": "low | medium | high",
  "duration_minutes": 60,
  "status": "pending | completed",
  "created_at": "2026-06-21T10:00:00Z",
  "updated_at": "2026-06-21T10:00:00Z"
}
```

| Method | Path | Body | Result |
|---|---|---|---|
| `POST` | `/tasks` | title*, description, priority, duration_minutes | `201` task |
| `GET` | `/tasks` | — (query: `status`, `priority`) | `200 { "tasks": [...] }` |
| `GET` | `/tasks/:id` | — | `200` task |
| `PATCH` | `/tasks/:id` | any subset of fields | `200` task |
| `DELETE` | `/tasks/:id` | — | `204` |

\* required.

---

## Planning  🔒

The flow is async: `POST /plans/generate` enqueues a job (the API never blocks on Claude); a worker
loads the user's pending tasks, calls the Planner agent, and persists the schedule. Poll the job, then
fetch the plan. The plan id **is** the job id.

### `POST /plans/generate`
```json
{ "date": "2026-06-22", "available_minutes": 480, "goals": ["ship batch 4"] }
```
`available_minutes` defaults to 480; `goals` optional. → `202 { "job_id": "uuid", "status": "queued", "date": "2026-06-22" }`

### `GET /plans/jobs/:id`  — poll job status
→ `200 { "job_id", "status": "queued|running|done|failed", "date", "schedule?", "error?" }`
(`schedule` present once `done`; `error` present if `failed`.)

### `GET /plans?date=YYYY-MM-DD`  — fetch the plan for a date
→ `200` plan object: `{ "id", "date", "status", "schedule", "created_at", "updated_at" }`.
The `schedule` matches the Planner contract in [AI_DESIGN.md](AI_DESIGN.md). → `404` if no plan for that date.

---

## Interactive AI  🔒

Synchronous Claude calls (return directly, ~1–2s) — unlike the queued daily plan. Return `503`
(`unavailable`) if no `ANTHROPIC_API_KEY` is configured.

### `POST /ai/prioritize`  — rank the user's pending tasks
→ `200`
```json
{
  "ranked": [ { "task_id": "uuid", "rank": 1, "reason": "blocks the release", "urgent": true } ],
  "drop_suggestions": [ { "task_id": "uuid", "reason": "low value, no deadline" } ]
}
```
→ `400` if there are no pending tasks to rank.

### `POST /ai/breakdown/:id`  — split a task into ordered steps
→ `200`
```json
{ "task_id": "uuid", "steps": [ { "order": 1, "title": "Read pgx docs", "duration_minutes": 30 } ] }
```
→ `404` if the task doesn't exist / isn't yours.

### `POST /ai/weekly-report`  — narrative productivity review
→ `200`
```json
{ "headline": "string", "summary": "string", "wins": ["..."], "watch_outs": ["..."], "suggestions": ["..."] }
```
→ `400` if there are no tasks to report on.

> **Planner focus mode:** `POST /plans/generate` accepts an optional `"mode"`:
> `balanced` (default) · `deep_focus` · `stress_relief` · `light`, which reshapes the generated day.

---

## Analytics  🔒

### `GET /stats`  — productivity snapshot
→ `200`
```json
{
  "total_tasks": 8, "completed": 3, "pending": 5,
  "completion_rate": 0.375,
  "pending_minutes": 240, "completed_minutes": 130,
  "by_priority": { "high": 2, "medium": 3, "low": 3 },
  "plans_generated": 2,
  "trend": [ { "date": "2026-06-18", "completed": 0 }, ... 7 days ... ]
}
```

---

## Habits  🔒

Habit object: `{ "id", "name", "created_at", "streak", "checkins": ["YYYY-MM-DD", ...] }`
(`checkins` covers the last 28 days; `streak` is consecutive checked days ending today/yesterday).

| Method | Path | Body | Result |
|---|---|---|---|
| `GET` | `/habits` | — | `200 { "habits": [...] }` |
| `POST` | `/habits` | `{ "name" }` | `201` habit |
| `DELETE` | `/habits/:id` | — | `204` |
| `POST` | `/habits/:id/checkin` | `{ "date"? }` (default today) | `204` |
| `DELETE` | `/habits/:id/checkin?date=YYYY-MM-DD` | — (default today) | `204` |
