package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// setupDB spins up an ephemeral Postgres, applies the schema, and returns a pool.
func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("acos"),
		postgres.WithUsername("acos"),
		postgres.WithPassword("acos"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	schema, err := os.ReadFile("../../migrations/000001_init.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(schema))
	require.NoError(t, err)

	return pool
}

func TestRepositoryUserAndTaskFlow(t *testing.T) {
	pool := setupDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	// Create a user.
	user, err := repo.CreateUser(ctx, repository.CreateUserParams{
		Name:         "Rafay",
		Email:        "rafay@example.com",
		PasswordHash: "hash",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, user.ID)

	// Duplicate email violates the unique constraint.
	_, err = repo.CreateUser(ctx, repository.CreateUserParams{
		Name: "Other", Email: "rafay@example.com", PasswordHash: "h",
	})
	require.Error(t, err)

	// Create a task for the user.
	task, err := repo.CreateTask(ctx, repository.CreateTaskParams{
		UserID:          user.ID,
		Title:           "Learn worker pools",
		Description:     "channels + goroutines",
		Priority:        repository.TaskPriorityHigh,
		DurationMinutes: 90,
	})
	require.NoError(t, err)
	require.Equal(t, repository.TaskStatusPending, task.Status)

	// Partial update: only status.
	completed := repository.TaskStatusCompleted
	updated, err := repo.UpdateTask(ctx, repository.UpdateTaskParams{
		ID: task.ID, UserID: user.ID, Status: &completed,
	})
	require.NoError(t, err)
	require.Equal(t, repository.TaskStatusCompleted, updated.Status)
	require.Equal(t, "Learn worker pools", updated.Title) // unchanged via COALESCE

	// Filtered list.
	high := repository.TaskPriorityHigh
	tasks, err := repo.ListTasks(ctx, repository.ListTasksParams{UserID: user.ID, Priority: &high})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Delete.
	n, err := repo.DeleteTask(ctx, repository.DeleteTaskParams{ID: task.ID, UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}
