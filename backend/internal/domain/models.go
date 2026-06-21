package domain

import (
	"time"

	"github.com/google/uuid"
)

// Priority is a task's importance level.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

// TaskStatus is a task's completion state.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusCompleted TaskStatus = "completed"
)

// Valid reports whether s is a known status.
func (s TaskStatus) Valid() bool {
	switch s {
	case StatusPending, StatusCompleted:
		return true
	default:
		return false
	}
}

// User is an application user (without the password hash).
type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Task is a unit of work owned by a user.
type Task struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"-"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Priority        Priority   `json:"priority"`
	DurationMinutes int32      `json:"duration_minutes"`
	Status          TaskStatus `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
