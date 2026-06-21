package worker_test

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/worker"
)

func newTestQueue(t *testing.T) *queue.Queue {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return queue.New(rdb, "test")
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitFor polls cond until true or the deadline elapses.
func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestPoolProcessesAllJobsConcurrently(t *testing.T) {
	q := newTestQueue(t)
	ctx, cancel := context.WithCancel(context.Background())

	const total = 12
	var processed atomic.Int64
	pool := worker.NewPool(q, 4, quietLogger(),
		worker.WithClaimTimeout(50*time.Millisecond),
		worker.WithBackoff(func(int) time.Duration { return 0 }),
	)
	pool.Register(worker.JobTypeDemo, func(_ context.Context, _ queue.Job) error {
		processed.Add(1)
		return nil
	})

	for i := 0; i < total; i++ {
		job, err := queue.NewJob(worker.JobTypeDemo, worker.DemoPayload{Message: "x"})
		require.NoError(t, err)
		require.NoError(t, q.Enqueue(ctx, job))
	}

	done := make(chan struct{})
	go func() { _ = pool.Run(ctx); close(done) }()

	waitFor(t, 3*time.Second, func() bool { return processed.Load() == total })

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pool did not shut down")
	}
	require.Equal(t, int64(total), processed.Load())
}

func TestPoolRetriesThenDeadLetters(t *testing.T) {
	q := newTestQueue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int64
	pool := worker.NewPool(q, 2, quietLogger(),
		worker.WithClaimTimeout(50*time.Millisecond),
		worker.WithBackoff(func(int) time.Duration { return 0 }),
	)
	pool.Register(worker.JobTypeDemo, func(_ context.Context, _ queue.Job) error {
		attempts.Add(1)
		return assertErr{}
	})

	job, err := queue.NewJob(worker.JobTypeDemo, worker.DemoPayload{Fail: true})
	require.NoError(t, err)
	job.MaxAttempts = 3
	require.NoError(t, q.Enqueue(ctx, job))

	done := make(chan struct{})
	go func() { _ = pool.Run(ctx); close(done) }()

	// Job should be tried 3 times then dead-lettered.
	waitFor(t, 3*time.Second, func() bool {
		_, _, dead, err := q.Lengths(context.Background())
		return err == nil && dead == 1
	})
	require.Equal(t, int64(3), attempts.Load())

	cancel()
	<-done
}

func TestPoolDeadLettersUnknownType(t *testing.T) {
	q := newTestQueue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := worker.NewPool(q, 1, quietLogger(), worker.WithClaimTimeout(50*time.Millisecond))
	// No handler registered for "demo".

	job, err := queue.NewJob(worker.JobTypeDemo, nil)
	require.NoError(t, err)
	require.NoError(t, q.Enqueue(ctx, job))

	done := make(chan struct{})
	go func() { _ = pool.Run(ctx); close(done) }()

	waitFor(t, 2*time.Second, func() bool {
		_, _, dead, err := q.Lengths(context.Background())
		return err == nil && dead == 1
	})
	cancel()
	<-done
}

type assertErr struct{}

func (assertErr) Error() string { return "intentional failure" }
