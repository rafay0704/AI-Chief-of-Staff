package ai

import (
	"context"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// PlanInput is the input to the Planner agent.
type PlanInput struct {
	Date             string
	AvailableMinutes int
	Goals            []string
	Tasks            []domain.Task
}

// Agents bundles the three planning agents over a shared Completer.
type Agents struct {
	c Completer
}

// NewAgents constructs the agent set from a Completer.
func NewAgents(c Completer) *Agents {
	return &Agents{c: c}
}

// Plan generates a structured daily schedule.
func (a *Agents) Plan(ctx context.Context, in PlanInput) (Schedule, error) {
	user := buildPlannerUser(in)
	var out Schedule
	err := a.runWithRepair(ctx, plannerSystem, user, func(raw string) error {
		out = Schedule{}
		return parseStrict(raw, &out)
	})
	return out, err
}

// Prioritize ranks the given tasks and flags drops.
func (a *Agents) Prioritize(ctx context.Context, tasks []domain.Task) (PriorityResult, error) {
	user := buildPriorityUser(tasks)
	var out PriorityResult
	err := a.runWithRepair(ctx, prioritySystem, user, func(raw string) error {
		out = PriorityResult{}
		return parseStrict(raw, &out)
	})
	return out, err
}

// Break splits a single task into ordered steps.
func (a *Agents) Break(ctx context.Context, task domain.Task) (Breakdown, error) {
	user := buildBreakdownUser(task)
	var out Breakdown
	err := a.runWithRepair(ctx, breakdownSystem, user, func(raw string) error {
		out = Breakdown{}
		return parseStrict(raw, &out)
	})
	return out, err
}

// runWithRepair completes the prompt, parses it, and on a parse/validation
// failure makes exactly one repair round-trip before giving up.
func (a *Agents) runWithRepair(ctx context.Context, system, user string, parse func(raw string) error) error {
	raw, err := a.c.Complete(ctx, system, user)
	if err != nil {
		return err
	}
	if perr := parse(raw); perr == nil {
		return nil
	} else {
		// One repair attempt: tell the model what was wrong.
		raw2, rerr := a.c.Complete(ctx, system, repairUser(user, raw, perr))
		if rerr != nil {
			return rerr
		}
		return parse(raw2)
	}
}
