package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/ai"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// AIService exposes the interactive Claude agents (priority ranking and task
// breakdown) synchronously — these are fast single calls, unlike daily planning
// which is queued. agents may be nil when no API key is configured, in which
// case the methods return domain.ErrUnavailable.
type AIService struct {
	tasks  *TaskService
	agents *ai.Agents
}

// NewAIService constructs an AIService. Pass agents=nil when AI is unconfigured.
func NewAIService(tasks *TaskService, agents *ai.Agents) *AIService {
	return &AIService{tasks: tasks, agents: agents}
}

// Prioritize ranks the user's pending tasks and flags drop candidates.
func (s *AIService) Prioritize(ctx context.Context, userID uuid.UUID) (ai.PriorityResult, error) {
	if s.agents == nil {
		return ai.PriorityResult{}, fmt.Errorf("%w: AI is not configured", domain.ErrUnavailable)
	}
	pending := domain.StatusPending
	tasks, err := s.tasks.List(ctx, userID, ListTaskFilter{Status: &pending})
	if err != nil {
		return ai.PriorityResult{}, err
	}
	if len(tasks) == 0 {
		return ai.PriorityResult{}, fmt.Errorf("%w: no pending tasks to prioritize", domain.ErrValidation)
	}
	return s.agents.Prioritize(ctx, tasks)
}

// Breakdown splits one of the user's tasks into ordered steps.
func (s *AIService) Breakdown(ctx context.Context, userID, taskID uuid.UUID) (ai.Breakdown, error) {
	if s.agents == nil {
		return ai.Breakdown{}, fmt.Errorf("%w: AI is not configured", domain.ErrUnavailable)
	}
	task, err := s.tasks.Get(ctx, userID, taskID) // ErrNotFound if missing / not owned
	if err != nil {
		return ai.Breakdown{}, err
	}
	return s.agents.Break(ctx, task)
}
