-- name: CreateHabit :one
INSERT INTO habits (user_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: ListHabits :many
SELECT * FROM habits
WHERE user_id = $1
ORDER BY created_at;

-- name: GetHabit :one
SELECT * FROM habits
WHERE id = $1 AND user_id = $2;

-- name: DeleteHabit :execrows
DELETE FROM habits
WHERE id = $1 AND user_id = $2;

-- name: CheckInHabit :exec
INSERT INTO habit_checkins (habit_id, day)
VALUES ($1, $2)
ON CONFLICT (habit_id, day) DO NOTHING;

-- name: UncheckHabit :exec
DELETE FROM habit_checkins
WHERE habit_id = $1 AND day = $2;

-- name: ListCheckinsSince :many
SELECT c.habit_id, c.day
FROM habit_checkins c
JOIN habits h ON h.id = c.habit_id
WHERE h.user_id = $1 AND c.day >= $2
ORDER BY c.day;
