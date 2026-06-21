-- name: CreateTask :one
INSERT INTO tasks (user_id, title, description, priority, duration_minutes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks
WHERE id = $1 AND user_id = $2;

-- name: ListTasks :many
SELECT * FROM tasks
WHERE user_id = $1
  AND (sqlc.narg('status')::task_status IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::task_priority IS NULL OR priority = sqlc.narg('priority'))
ORDER BY created_at DESC;

-- name: UpdateTask :one
UPDATE tasks SET
    title            = COALESCE(sqlc.narg('title'), title),
    description      = COALESCE(sqlc.narg('description'), description),
    priority         = COALESCE(sqlc.narg('priority'), priority),
    duration_minutes = COALESCE(sqlc.narg('duration_minutes'), duration_minutes),
    status           = COALESCE(sqlc.narg('status'), status),
    updated_at       = now()
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: DeleteTask :execrows
DELETE FROM tasks
WHERE id = $1 AND user_id = $2;
