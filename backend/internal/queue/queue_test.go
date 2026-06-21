package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
)

func newTestQueue(t *testing.T) *queue.Queue {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return queue.New(rdb, "test")
}

func TestEnqueueClaimAck(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	job, err := queue.NewJob("demo", map[string]string{"hello": "world"})
	require.NoError(t, err)
	require.NoError(t, q.Enqueue(ctx, job))

	main, _, _, err := q.Lengths(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), main)

	d, err := q.Claim(ctx, 100*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, job.ID, d.Job.ID)

	// While claimed, it lives in the processing list.
	main, processing, _, err := q.Lengths(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), main)
	require.Equal(t, int64(1), processing)

	require.NoError(t, q.Ack(ctx, d))
	main, processing, dead, err := q.Lengths(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), main)
	require.Equal(t, int64(0), processing)
	require.Equal(t, int64(0), dead)
}

func TestClaimTimeoutReturnsErrEmpty(t *testing.T) {
	q := newTestQueue(t)
	_, err := q.Claim(context.Background(), 50*time.Millisecond)
	require.ErrorIs(t, err, queue.ErrEmpty)
}

func TestRetryThenDeadLetter(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	job, err := queue.NewJob("demo", nil)
	require.NoError(t, err)
	job.MaxAttempts = 2
	require.NoError(t, q.Enqueue(ctx, job))

	// Attempt 0 -> retry (re-queued, not dead).
	d, err := q.Claim(ctx, 100*time.Millisecond)
	require.NoError(t, err)
	dead, err := q.Retry(ctx, d)
	require.NoError(t, err)
	require.False(t, dead)

	main, processing, _, err := q.Lengths(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), main)
	require.Equal(t, int64(0), processing)

	// Attempt 1 -> reaches MaxAttempts(2) -> dead-lettered.
	d, err = q.Claim(ctx, 100*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 1, d.Job.Attempt)
	dead, err = q.Retry(ctx, d)
	require.NoError(t, err)
	require.True(t, dead)

	main, processing, deadLen, err := q.Lengths(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), main)
	require.Equal(t, int64(0), processing)
	require.Equal(t, int64(1), deadLen)
}

func TestRecoverRequeuesProcessing(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	job, err := queue.NewJob("demo", nil)
	require.NoError(t, err)
	require.NoError(t, q.Enqueue(ctx, job))

	// Claim but never ack -> simulates a crashed worker.
	_, err = q.Claim(ctx, 100*time.Millisecond)
	require.NoError(t, err)

	n, err := q.Recover(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	main, processing, _, err := q.Lengths(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), main)
	require.Equal(t, int64(0), processing)
}
