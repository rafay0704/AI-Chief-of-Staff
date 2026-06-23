package domain

// Stats is a snapshot of a user's productivity.
type Stats struct {
	TotalTasks       int            `json:"total_tasks"`
	Completed        int            `json:"completed"`
	Pending          int            `json:"pending"`
	CompletionRate   float64        `json:"completion_rate"` // 0..1
	PendingMinutes   int            `json:"pending_minutes"`
	CompletedMinutes int            `json:"completed_minutes"`
	ByPriority       PriorityCounts `json:"by_priority"`
	PlansGenerated   int            `json:"plans_generated"`
	Trend            []DayCount     `json:"trend"` // last 7 days, oldest first, zero-filled
}

// PriorityCounts breaks tasks down by priority.
type PriorityCounts struct {
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

// DayCount is the number of tasks completed on a given day.
type DayCount struct {
	Date      string `json:"date"` // YYYY-MM-DD
	Completed int    `json:"completed"`
}
