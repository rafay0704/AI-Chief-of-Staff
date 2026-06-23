# 🤖 AI Design (Claude)

> Full implementation lands in **Batch 3**. This doc defines the contracts up front so the rest of
> the system can be built against them.

## Principles

1. **Strict JSON only.** Every agent returns JSON matching a Go struct; we parse + validate and never
   treat the model output as free text.
2. **Typed in, typed out.** Inputs are built from domain structs; outputs unmarshal into schemas in
   `internal/ai/schema.go`.
3. **Context everywhere.** All calls take `context.Context` with timeouts; retry on 429/5xx with
   backoff.
4. **Versioned prompts.** Each agent's system prompt has a version constant so we can track changes.
5. **Defensive parsing.** On invalid JSON, attempt one repair round-trip, then fail the job cleanly.

## Model

Default `ANTHROPIC_MODEL=claude-haiku-4-5-20251001` (configurable) — credit-efficient and plenty
capable for structured JSON planning; bump to `claude-sonnet-4-6` for higher-quality plans. Uses the
official `anthropic-sdk-go`. Requests are plain (Haiku does not take `effort`/`thinking`); the SDK
auto-retries 429/5xx, and we add a per-request timeout.

**Implemented (Batch 3):** agents depend on a `Completer` interface (`internal/ai/client.go`) so they
can be unit-tested with a fake; the real `Client` calls Claude. Each agent enforces strict JSON via the
system prompt, parses defensively (`extractJSON` strips markdown fences), validates against the Go
schema, and makes exactly **one repair round-trip** on failure before erroring.

## Agents

### 1. Planner Agent
Builds a structured daily schedule from tasks + available time + goals.

**Output contract:**
```json
{
  "date": "2026-06-22",
  "schedule": [
    { "time": "09:00 - 10:00", "task": "Learn Go concurrency", "type": "focus" },
    { "time": "10:00 - 10:30", "task": "Break", "type": "rest" }
  ],
  "summary": "Balanced productive day with a learning focus"
}
```
`type` ∈ `focus | admin | meeting | rest | buffer`.

### 2. Priority Agent
Ranks tasks by importance/urgency and flags optimizations.

**Output contract:**
```json
{
  "ranked": [
    { "task_id": "uuid", "rank": 1, "reason": "blocks everything else", "urgent": true }
  ],
  "drop_suggestions": [ { "task_id": "uuid", "reason": "low value, no deadline" } ]
}
```

### 3. Breakdown Agent
Splits a large task into ordered subtasks.

**Output contract:**
```json
{
  "task_id": "uuid",
  "steps": [
    { "order": 1, "title": "Read pgx docs", "duration_minutes": 30 }
  ]
}
```

## System prompt skeleton

```
You are an AI Chief of Staff — a precise planning engine.

Given:
- Tasks: {{tasks_json}}
- Available time (minutes): {{available_minutes}}
- Goals: {{goals}}

Rules:
- Respond with ONLY valid JSON matching the schema. No prose, no markdown fences.
- Optimize for deep-focus blocks, realistic durations, and adequate breaks.
- Never invent task ids; only use ids provided.
```

## Failure handling

| Failure | Handling |
|---|---|
| Non-JSON / schema mismatch | one repair attempt → else job `failed` with reason |
| 429 / 5xx | exponential backoff retry (capped) |
| Timeout | cancel via context, mark transient, re-enqueue |
| Empty schedule | validation error → job `failed` |

## V2 additions (Batch 7)

### 4. Reporter Agent (`reporter-v1`)
Synchronous (`POST /ai/weekly-report`). Given the user's task counts, focus minutes, plans generated,
and task list, returns a narrative review:
```json
{ "headline": "string", "summary": "string", "wins": ["..."], "watch_outs": ["..."], "suggestions": ["..."] }
```

### Planner focus modes
`POST /plans/generate` accepts `mode` ∈ `balanced | deep_focus | stress_relief | light`. The planner
appends a mode-specific instruction to the prompt (e.g. stress-relief trims to 2–3 tasks with more rest).
All four interactive AI calls (prioritize, breakdown, weekly-report) are synchronous; only daily plan
generation is queued. See `docs/DECISIONS.md` ADR-011.
