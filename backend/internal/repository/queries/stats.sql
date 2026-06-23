-- name: TaskStats :one
SELECT
    (COUNT(*))::int                                              AS total,
    (COUNT(*) FILTER (WHERE status = 'completed'))::int          AS completed,
    (COUNT(*) FILTER (WHERE status = 'pending'))::int            AS pending,
    (COUNT(*) FILTER (WHERE priority = 'high'))::int             AS high,
    (COUNT(*) FILTER (WHERE priority = 'medium'))::int           AS medium,
    (COUNT(*) FILTER (WHERE priority = 'low'))::int              AS low,
    (COALESCE(SUM(duration_minutes) FILTER (WHERE status = 'pending'), 0))::int   AS pending_minutes,
    (COALESCE(SUM(duration_minutes) FILTER (WHERE status = 'completed'), 0))::int AS completed_minutes
FROM tasks
WHERE user_id = $1;

-- name: PlanCount :one
SELECT (COUNT(*))::int AS count
FROM daily_plans
WHERE user_id = $1;

-- name: CompletionTrend :many
SELECT
    (updated_at AT TIME ZONE 'UTC')::date AS day,
    (COUNT(*))::int                       AS count
FROM tasks
WHERE user_id = $1 AND status = 'completed' AND updated_at >= $2
GROUP BY day
ORDER BY day;
