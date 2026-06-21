package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PlanStatus is the lifecycle state of a daily plan / planning job.
type PlanStatus string

const (
	PlanQueued  PlanStatus = "queued"
	PlanRunning PlanStatus = "running"
	PlanDone    PlanStatus = "done"
	PlanFailed  PlanStatus = "failed"
)

// Plan is a user's daily plan. Its ID doubles as the planning job id.
type Plan struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"-"`
	Date      string          `json:"date"` // YYYY-MM-DD
	Status    PlanStatus      `json:"status"`
	Schedule  json.RawMessage `json:"schedule,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
