package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
)

// JobTypeDemo is a placeholder job type used to exercise the pool in Batch 2.
// Real job types (e.g. plan generation) are added in Batch 4.
const JobTypeDemo = "demo"

// DemoPayload is the payload for a demo job.
type DemoPayload struct {
	Message string `json:"message"`
	SleepMs int    `json:"sleep_ms"`
	// Fail, when true, makes the handler return an error to exercise retries.
	Fail bool `json:"fail"`
}

// DemoHandler returns a handler that logs the message, optionally simulates
// work via SleepMs, and optionally fails to demonstrate retry/dead-lettering.
func DemoHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job queue.Job) error {
		var p DemoPayload
		if err := job.Decode(&p); err != nil {
			return err
		}
		if p.SleepMs > 0 {
			sleep(ctx, time.Duration(p.SleepMs)*time.Millisecond)
		}
		if p.Fail {
			return errors.New("demo: simulated failure")
		}
		log.Info("demo job processed", "message", p.Message)
		return nil
	}
}
