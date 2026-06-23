package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// Prompt versions — bump when a system prompt changes (see docs/AI_DESIGN.md).
const (
	plannerPromptVersion   = "planner-v1"
	priorityPromptVersion  = "priority-v1"
	breakdownPromptVersion = "breakdown-v1"
	reporterPromptVersion  = "reporter-v1"
)

const plannerSystem = `You are an AI Chief of Staff — a precise daily planning engine.

Respond with ONLY valid JSON matching this schema, no prose and no markdown fences:
{
  "date": "YYYY-MM-DD",
  "schedule": [
    { "time": "09:00 - 10:00", "task": "string", "type": "focus|admin|meeting|rest|buffer" }
  ],
  "summary": "string"
}

Rules:
- Use ONLY the task titles provided; never invent tasks.
- Honor each task's duration and priority; schedule high-priority work in deep-focus morning blocks.
- Keep the total within the available time; insert short rest/buffer blocks between focus blocks.
- "type" must be one of: focus, admin, meeting, rest, buffer.`

const prioritySystem = `You are an AI Chief of Staff — a task triage engine.

Respond with ONLY valid JSON matching this schema, no prose and no markdown fences:
{
  "ranked": [ { "task_id": "uuid", "rank": 1, "reason": "string", "urgent": true } ],
  "drop_suggestions": [ { "task_id": "uuid", "reason": "string" } ]
}

Rules:
- Rank every provided task from 1 (most important) upward; ranks are unique.
- Use ONLY the task_id values provided; never invent ids.
- Mark "urgent": true only for time-sensitive or blocking tasks.
- "drop_suggestions" may be empty; only suggest dropping low-value tasks with no deadline.`

const breakdownSystem = `You are an AI Chief of Staff — a task breakdown engine.

Respond with ONLY valid JSON matching this schema, no prose and no markdown fences:
{
  "task_id": "uuid",
  "steps": [ { "order": 1, "title": "string", "duration_minutes": 30 } ]
}

Rules:
- Break the task into 2-7 concrete, ordered steps.
- Step durations should sum to roughly the task's total duration.
- Echo back the provided task_id exactly.`

const reporterSystem = `You are an AI Chief of Staff writing a candid weekly productivity review.

Respond with ONLY valid JSON matching this schema, no prose and no markdown fences:
{
  "headline": "string (one punchy sentence)",
  "summary": "string (2-3 sentences, warm but honest)",
  "wins": ["string"],
  "watch_outs": ["string"],
  "suggestions": ["string (concrete, actionable for next week)"]
}

Rules:
- Ground every point in the provided numbers and tasks; do not invent data.
- 2-4 items each for wins, watch_outs, and suggestions (arrays may be empty if truly nothing applies).
- Be specific and motivating, not generic.`

// ReportInput is the input to the Reporter agent.
type ReportInput struct {
	Completed        int
	Pending          int
	CompletedMinutes int
	PlansGenerated   int
	Tasks            []domain.Task
}

func buildReporterUser(in ReportInput) string {
	return fmt.Sprintf(
		"This period's numbers:\n- Tasks completed: %d\n- Tasks still pending: %d\n- Focus minutes completed: %d\n- Daily plans generated: %d\n\nTasks: %s",
		in.Completed, in.Pending, in.CompletedMinutes, in.PlansGenerated, mustJSON(toTaskViews(in.Tasks)),
	)
}

// taskView is the slim task representation sent to the model.
type taskView struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description,omitempty"`
	Priority        string `json:"priority"`
	DurationMinutes int32  `json:"duration_minutes"`
	Status          string `json:"status"`
}

func toTaskViews(tasks []domain.Task) []taskView {
	views := make([]taskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, taskView{
			ID:              t.ID.String(),
			Title:           t.Title,
			Description:     t.Description,
			Priority:        string(t.Priority),
			DurationMinutes: t.DurationMinutes,
			Status:          string(t.Status),
		})
	}
	return views
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func buildPlannerUser(in PlanInput) string {
	s := fmt.Sprintf(
		"Date: %s\nAvailable time (minutes): %d\nGoals: %s\nTasks: %s",
		in.Date, in.AvailableMinutes, mustJSON(in.Goals), mustJSON(toTaskViews(in.Tasks)),
	)
	if g := modeGuidance(in.Mode); g != "" {
		s += "\nPlanning mode: " + g
	}
	return s
}

// modeGuidance returns the extra planner instruction for a focus mode.
func modeGuidance(mode string) string {
	switch mode {
	case "deep_focus":
		return "DEEP FOCUS — maximize long uninterrupted focus blocks (90+ minutes), batch all admin into a single slot, and minimize context switching."
	case "stress_relief":
		return "STRESS RELIEF — the user is overwhelmed. Schedule only the 2–3 most important tasks, add generous rest and breaks, and keep the day light and achievable."
	case "light":
		return "LIGHT DAY — keep it easy: only essential tasks, shorter blocks, and plenty of buffer time."
	default:
		return "" // balanced
	}
}

func buildPriorityUser(tasks []domain.Task) string {
	return "Tasks: " + mustJSON(toTaskViews(tasks))
}

func buildBreakdownUser(t domain.Task) string {
	return "Task: " + mustJSON(toTaskViews([]domain.Task{t})[0])
}

// repairUser asks the model to fix output that failed schema validation.
func repairUser(originalUser, badOutput string, validationErr error) string {
	return fmt.Sprintf(
		"%s\n\nYour previous response was invalid (%s). It was:\n%s\n\nReturn corrected JSON only, matching the schema exactly.",
		originalUser, validationErr, strings.TrimSpace(badOutput),
	)
}
