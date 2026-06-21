// Package queue implements a durable job queue on top of Redis lists.
//
// Reliability model (see docs/DECISIONS.md, ADR-008):
//   - Enqueue:   LPUSH onto the "main" list (head).
//   - Claim:     BLMOVE main(RIGHT) -> processing(LEFT). The job is atomically
//     moved to a processing list so an in-flight job survives a worker crash.
//   - Ack:       LREM the exact payload from the processing list.
//   - Retry:     on handler failure, increment Attempt; if attempts remain,
//     re-enqueue onto main, else move to the dead-letter list.
//   - Recover:   on startup, any leftovers in processing are moved back to main.
package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// DefaultMaxAttempts is used when a job is enqueued without an explicit limit.
const DefaultMaxAttempts = 3

// Job is the unit of work that travels through the queue.
type Job struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	EnqueuedAt  time.Time       `json:"enqueued_at"`
}

// NewJob builds a Job with a generated id and JSON-encoded payload.
func NewJob(jobType string, payload any) (Job, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Job{}, err
	}
	return Job{
		ID:          uuid.NewString(),
		Type:        jobType,
		Payload:     raw,
		Attempt:     0,
		MaxAttempts: DefaultMaxAttempts,
		EnqueuedAt:  time.Now().UTC(),
	}, nil
}

// Decode unmarshals the job payload into v.
func (j Job) Decode(v any) error {
	return json.Unmarshal(j.Payload, v)
}
