// Package worker implements a bounded goroutine worker pool that consumes jobs
// from the durable queue, dispatching them to type-registered handlers.
//
// Concurrency design:
//   - One dispatcher goroutine performs the blocking Claim and fans deliveries
//     out over an unbuffered channel (natural backpressure: it can't claim a
//     new job until a worker is free to receive).
//   - N worker goroutines receive deliveries, invoke the handler with panic
//     recovery, and ack / retry / dead-letter the result.
//   - Graceful shutdown: when the context is cancelled the dispatcher stops
//     claiming and closes the channel; workers finish their in-flight job and
//     exit. Ack/retry use a detached context so they complete during shutdown.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
)

// HandlerFunc processes a single job. Returning a non-nil error triggers retry
// (and eventually dead-lettering once attempts are exhausted).
type HandlerFunc func(ctx context.Context, job queue.Job) error

// Pool is a bounded worker pool bound to a queue.
type Pool struct {
	queue        *queue.Queue
	concurrency  int
	log          *slog.Logger
	handlers     map[string]HandlerFunc
	claimTimeout time.Duration
	backoff      func(attempt int) time.Duration
	ackTimeout   time.Duration
}

// Option configures a Pool.
type Option func(*Pool)

// WithClaimTimeout sets how long the dispatcher blocks per Claim.
func WithClaimTimeout(d time.Duration) Option { return func(p *Pool) { p.claimTimeout = d } }

// WithBackoff overrides the retry backoff function.
func WithBackoff(fn func(attempt int) time.Duration) Option {
	return func(p *Pool) { p.backoff = fn }
}

// NewPool builds a Pool. concurrency < 1 is treated as 1.
func NewPool(q *queue.Queue, concurrency int, log *slog.Logger, opts ...Option) *Pool {
	if concurrency < 1 {
		concurrency = 1
	}
	p := &Pool{
		queue:        q,
		concurrency:  concurrency,
		log:          log,
		handlers:     make(map[string]HandlerFunc),
		claimTimeout: 2 * time.Second,
		backoff:      defaultBackoff,
		ackTimeout:   5 * time.Second,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Register binds a handler to a job type. Not safe to call after Run.
func (p *Pool) Register(jobType string, h HandlerFunc) {
	p.handlers[jobType] = h
}

// Run starts the pool and blocks until ctx is cancelled and all workers drain.
func (p *Pool) Run(ctx context.Context) error {
	if n, err := p.queue.Recover(ctx); err != nil {
		p.log.Warn("queue recover failed", "err", err)
	} else if n > 0 {
		p.log.Info("recovered orphaned jobs", "count", n)
	}

	deliveries := make(chan queue.Delivery)
	var wg sync.WaitGroup

	// Workers.
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			wl := p.log.With("worker", id)
			for d := range deliveries {
				p.process(ctx, wl, d)
			}
		}(i)
	}

	// Dispatcher (this goroutine).
	p.log.Info("worker pool started", "concurrency", p.concurrency)
	p.dispatch(ctx, deliveries)

	close(deliveries)
	wg.Wait()
	p.log.Info("worker pool stopped")
	return nil
}

// dispatch claims jobs until the context is cancelled.
func (p *Pool) dispatch(ctx context.Context, out chan<- queue.Delivery) {
	for {
		if ctx.Err() != nil {
			return
		}
		d, err := p.queue.Claim(ctx, p.claimTimeout)
		switch {
		case err == nil:
			select {
			case out <- d:
			case <-ctx.Done():
				// Shutting down before hand-off: leave it in processing so
				// Recover re-queues it on the next start.
				return
			}
		case ctx.Err() != nil:
			return
		default:
			// ErrEmpty (timeout) or transient error: loop again.
			if !isEmpty(err) {
				p.log.Warn("claim error", "err", err)
			}
		}
	}
}

// process invokes the handler and acks/retries/kills based on the outcome.
func (p *Pool) process(ctx context.Context, log *slog.Logger, d queue.Delivery) {
	log = log.With("job_id", d.Job.ID, "type", d.Job.Type, "attempt", d.Job.Attempt)

	// Detached context so ack/retry survive shutdown of the main context.
	ackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.ackTimeout)
	defer cancel()

	h, ok := p.handlers[d.Job.Type]
	if !ok {
		log.Error("no handler registered; dead-lettering")
		if err := p.queue.Kill(ackCtx, d); err != nil {
			log.Error("kill failed", "err", err)
		}
		return
	}

	start := time.Now()
	err := invoke(ctx, h, d.Job)
	if err == nil {
		log.Info("job done", "duration_ms", time.Since(start).Milliseconds())
		if ackErr := p.queue.Ack(ackCtx, d); ackErr != nil {
			log.Error("ack failed", "err", ackErr)
		}
		return
	}

	log.Warn("job failed", "err", err)
	if d := p.backoff(d.Job.Attempt); d > 0 {
		sleep(ctx, d)
	}
	dead, rErr := p.queue.Retry(ackCtx, d)
	switch {
	case rErr != nil:
		log.Error("retry failed", "err", rErr)
	case dead:
		log.Error("job exhausted retries; dead-lettered")
	default:
		log.Info("job re-queued for retry")
	}
}

// invoke runs the handler, converting panics into errors.
func invoke(ctx context.Context, h HandlerFunc, job queue.Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Value: r}
		}
	}()
	return h(ctx, job)
}

// PanicError wraps a recovered panic value from a handler.
type PanicError struct{ Value any }

func (e *PanicError) Error() string { return "handler panic" }

func defaultBackoff(attempt int) time.Duration {
	d := 100 * time.Millisecond * (1 << attempt) // 100ms, 200ms, 400ms, ...
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

func isEmpty(err error) bool {
	return errors.Is(err, queue.ErrEmpty)
}
