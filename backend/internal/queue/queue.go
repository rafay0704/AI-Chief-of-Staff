package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrEmpty is returned by Claim when the blocking pop times out with no job.
var ErrEmpty = errors.New("queue: no job available")

// Queue is a durable Redis-list job queue.
type Queue struct {
	rdb        *redis.Client
	main       string
	processing string
	dead       string
}

// New creates a Queue using the given namespace for its Redis keys, e.g.
// namespace "acos" -> lists "acos:jobs", "acos:jobs:processing", "acos:jobs:dead".
func New(rdb *redis.Client, namespace string) *Queue {
	return &Queue{
		rdb:        rdb,
		main:       namespace + ":jobs",
		processing: namespace + ":jobs:processing",
		dead:       namespace + ":jobs:dead",
	}
}

// Enqueue pushes a job onto the head of the main list.
func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	raw, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := q.rdb.LPush(ctx, q.main, raw).Err(); err != nil {
		return fmt.Errorf("lpush: %w", err)
	}
	return nil
}

// Delivery is a claimed job plus the exact raw payload needed to ack/retry it.
type Delivery struct {
	Job Job
	raw string
}

// Claim atomically moves one job from the tail of main to the head of the
// processing list, blocking up to timeout. Returns ErrEmpty on timeout.
func (q *Queue) Claim(ctx context.Context, timeout time.Duration) (Delivery, error) {
	raw, err := q.rdb.BLMove(ctx, q.main, q.processing, "RIGHT", "LEFT", timeout).Result()
	if errors.Is(err, redis.Nil) {
		return Delivery{}, ErrEmpty
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("blmove: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		// Poison message: drop it from processing so it can't wedge the queue.
		_ = q.rdb.LRem(ctx, q.processing, 1, raw).Err()
		return Delivery{}, fmt.Errorf("unmarshal claimed job: %w", err)
	}
	return Delivery{Job: job, raw: raw}, nil
}

// Ack removes a successfully-processed job from the processing list.
func (q *Queue) Ack(ctx context.Context, d Delivery) error {
	if err := q.rdb.LRem(ctx, q.processing, 1, d.raw).Err(); err != nil {
		return fmt.Errorf("ack lrem: %w", err)
	}
	return nil
}

// Retry removes the delivery from processing and either re-enqueues it (with an
// incremented attempt) onto main, or, once attempts are exhausted, moves it to
// the dead-letter list. Returns true if the job was dead-lettered.
func (q *Queue) Retry(ctx context.Context, d Delivery) (dead bool, err error) {
	pipe := q.rdb.TxPipeline()
	pipe.LRem(ctx, q.processing, 1, d.raw)

	job := d.Job
	job.Attempt++
	if job.Attempt >= maxAttempts(job) {
		raw, mErr := json.Marshal(job)
		if mErr != nil {
			return false, fmt.Errorf("marshal dead job: %w", mErr)
		}
		pipe.LPush(ctx, q.dead, raw)
		dead = true
	} else {
		raw, mErr := json.Marshal(job)
		if mErr != nil {
			return false, fmt.Errorf("marshal retry job: %w", mErr)
		}
		pipe.LPush(ctx, q.main, raw)
	}

	if _, err = pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("retry pipeline: %w", err)
	}
	return dead, nil
}

// Kill removes a delivery from processing and sends it straight to the
// dead-letter list, bypassing retries (used for non-retryable failures such as
// an unknown job type).
func (q *Queue) Kill(ctx context.Context, d Delivery) error {
	pipe := q.rdb.TxPipeline()
	pipe.LRem(ctx, q.processing, 1, d.raw)
	pipe.LPush(ctx, q.dead, d.raw)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("kill pipeline: %w", err)
	}
	return nil
}

// Recover moves any jobs left in the processing list (e.g. from a crashed
// worker) back onto the main queue. Called once at startup. Returns the count.
func (q *Queue) Recover(ctx context.Context) (int, error) {
	n := 0
	for {
		raw, err := q.rdb.LMove(ctx, q.processing, q.main, "RIGHT", "LEFT").Result()
		if errors.Is(err, redis.Nil) {
			return n, nil
		}
		if err != nil {
			return n, fmt.Errorf("recover lmove: %w", err)
		}
		_ = raw
		n++
	}
}

// Lengths returns the current depth of the main, processing, and dead lists
// (useful for tests and observability).
func (q *Queue) Lengths(ctx context.Context) (main, processing, dead int64, err error) {
	if main, err = q.rdb.LLen(ctx, q.main).Result(); err != nil {
		return
	}
	if processing, err = q.rdb.LLen(ctx, q.processing).Result(); err != nil {
		return
	}
	dead, err = q.rdb.LLen(ctx, q.dead).Result()
	return
}

func maxAttempts(j Job) int {
	if j.MaxAttempts <= 0 {
		return DefaultMaxAttempts
	}
	return j.MaxAttempts
}
