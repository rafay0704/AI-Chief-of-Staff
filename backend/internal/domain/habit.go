package domain

import (
	"time"

	"github.com/google/uuid"
)

// Habit is a recurring practice the user checks in on daily.
type Habit struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Streak    int       `json:"streak"`
	Checkins  []string  `json:"checkins"` // YYYY-MM-DD dates within the display window
}
