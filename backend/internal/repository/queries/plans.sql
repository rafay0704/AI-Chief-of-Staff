-- name: UpsertPlanQueued :one
INSERT INTO daily_plans (user_id, plan_date, status)
VALUES ($1, $2, 'queued')
ON CONFLICT (user_id, plan_date)
DO UPDATE SET status = 'queued', plan_json = NULL, error = NULL, updated_at = now()
RETURNING *;

-- name: GetPlanByID :one
SELECT * FROM daily_plans
WHERE id = $1 AND user_id = $2;

-- name: GetPlanByDate :one
SELECT * FROM daily_plans
WHERE user_id = $1 AND plan_date = $2;

-- name: SetPlanRunning :exec
UPDATE daily_plans
SET status = 'running', updated_at = now()
WHERE id = $1;

-- name: SetPlanDone :exec
UPDATE daily_plans
SET status = 'done', plan_json = $2, error = NULL, updated_at = now()
WHERE id = $1;

-- name: SetPlanFailed :exec
UPDATE daily_plans
SET status = 'failed', error = $2, updated_at = now()
WHERE id = $1;
