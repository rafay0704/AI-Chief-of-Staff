package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ── Planner agent output ──────────────────────────────────────────────────────

// Schedule is the Planner agent's structured daily plan.
type Schedule struct {
	Date     string         `json:"date"`
	Schedule []ScheduleItem `json:"schedule"`
	Summary  string         `json:"summary"`
}

// ScheduleItem is one time block in the day.
type ScheduleItem struct {
	Time string `json:"time"`
	Task string `json:"task"`
	Type string `json:"type"` // focus | admin | meeting | rest | buffer
}

var validBlockTypes = map[string]bool{
	"focus": true, "admin": true, "meeting": true, "rest": true, "buffer": true,
}

// Validate checks the schedule is well-formed.
func (s Schedule) Validate() error {
	if strings.TrimSpace(s.Date) == "" {
		return fmt.Errorf("schedule: date is required")
	}
	if len(s.Schedule) == 0 {
		return fmt.Errorf("schedule: must contain at least one block")
	}
	for i, item := range s.Schedule {
		if strings.TrimSpace(item.Time) == "" || strings.TrimSpace(item.Task) == "" {
			return fmt.Errorf("schedule: block %d missing time or task", i)
		}
		if !validBlockTypes[item.Type] {
			return fmt.Errorf("schedule: block %d has invalid type %q", i, item.Type)
		}
	}
	return nil
}

// ── Priority agent output ─────────────────────────────────────────────────────

// PriorityResult ranks tasks and suggests drops.
type PriorityResult struct {
	Ranked          []RankedTask     `json:"ranked"`
	DropSuggestions []DropSuggestion `json:"drop_suggestions"`
}

// RankedTask is one task with its computed rank.
type RankedTask struct {
	TaskID string `json:"task_id"`
	Rank   int    `json:"rank"`
	Reason string `json:"reason"`
	Urgent bool   `json:"urgent"`
}

// DropSuggestion proposes removing a low-value task.
type DropSuggestion struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// Validate checks the priority result is well-formed.
func (p PriorityResult) Validate() error {
	if len(p.Ranked) == 0 {
		return fmt.Errorf("priority: ranked list is empty")
	}
	for i, r := range p.Ranked {
		if strings.TrimSpace(r.TaskID) == "" {
			return fmt.Errorf("priority: ranked %d missing task_id", i)
		}
		if r.Rank < 1 {
			return fmt.Errorf("priority: ranked %d has invalid rank %d", i, r.Rank)
		}
	}
	return nil
}

// ── Breakdown agent output ────────────────────────────────────────────────────

// Breakdown splits a task into ordered steps.
type Breakdown struct {
	TaskID string `json:"task_id"`
	Steps  []Step `json:"steps"`
}

// Step is one subtask in a breakdown.
type Step struct {
	Order           int    `json:"order"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"duration_minutes"`
}

// Validate checks the breakdown is well-formed.
func (b Breakdown) Validate() error {
	if len(b.Steps) == 0 {
		return fmt.Errorf("breakdown: must contain at least one step")
	}
	for i, s := range b.Steps {
		if strings.TrimSpace(s.Title) == "" {
			return fmt.Errorf("breakdown: step %d missing title", i)
		}
	}
	return nil
}

// ── Reporter agent output ─────────────────────────────────────────────────────

// WeeklyReport is the Reporter agent's narrative weekly review.
type WeeklyReport struct {
	Headline    string   `json:"headline"`
	Summary     string   `json:"summary"`
	Wins        []string `json:"wins"`
	WatchOuts   []string `json:"watch_outs"`
	Suggestions []string `json:"suggestions"`
}

// Validate checks the report has the essential narrative fields.
func (r WeeklyReport) Validate() error {
	if strings.TrimSpace(r.Headline) == "" {
		return fmt.Errorf("report: headline is required")
	}
	if strings.TrimSpace(r.Summary) == "" {
		return fmt.Errorf("report: summary is required")
	}
	return nil
}

// ── Parsing helpers ───────────────────────────────────────────────────────────

type validatable interface{ Validate() error }

// parseStrict extracts JSON from a model response, unmarshals it into v, and
// validates it. v must be a pointer to a type implementing Validate.
func parseStrict[T validatable](raw string, v *T) error {
	js := extractJSON(raw)
	if err := json.Unmarshal([]byte(js), v); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	if err := (*v).Validate(); err != nil {
		return err
	}
	return nil
}

// extractJSON strips common wrappers (markdown fences, prose) and returns the
// substring from the first '{' to the last '}'.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
