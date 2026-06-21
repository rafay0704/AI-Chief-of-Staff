package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/ai"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/service"
)

// PlanHandler builds a handler for plan-generation jobs: load the user's
// pending tasks, ask the Planner agent for a schedule, and persist the result.
//
// An AI failure marks the plan "failed" and acks the job (no retry storm — the
// user can re-generate). Infrastructure errors (DB, decode) are returned so the
// queue retries them.
func PlanHandler(plans *service.PlanService, tasks *service.TaskService, agents *ai.Agents, log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job queue.Job) error {
		var p service.PlanJobPayload
		if err := job.Decode(&p); err != nil {
			return err // malformed payload → dead-letter
		}
		l := log.With("plan_id", p.PlanID, "date", p.Date)

		if err := plans.MarkRunning(ctx, p.PlanID); err != nil {
			return err
		}

		pending := domain.StatusPending
		userTasks, err := tasks.List(ctx, p.UserID, service.ListTaskFilter{Status: &pending})
		if err != nil {
			return err
		}

		schedule, err := agents.Plan(ctx, ai.PlanInput{
			Date:             p.Date,
			AvailableMinutes: p.AvailableMinutes,
			Goals:            p.Goals,
			Tasks:            userTasks,
		})
		if err != nil {
			l.Warn("plan generation failed", "err", err)
			if ferr := plans.Fail(ctx, p.PlanID, err.Error()); ferr != nil {
				return ferr
			}
			return nil
		}

		raw, err := json.Marshal(schedule)
		if err != nil {
			return err
		}
		if err := plans.Complete(ctx, p.PlanID, raw); err != nil {
			return err
		}
		l.Info("plan generated", "blocks", len(schedule.Schedule))
		return nil
	}
}

// PlanUnavailableHandler is registered when no AI client is configured; it marks
// plan jobs failed instead of silently dead-lettering them.
func PlanUnavailableHandler(plans *service.PlanService, log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job queue.Job) error {
		var p service.PlanJobPayload
		if err := job.Decode(&p); err != nil {
			return err
		}
		log.Warn("plan job received but AI is not configured", "plan_id", p.PlanID)
		return plans.Fail(ctx, p.PlanID, "AI not configured (ANTHROPIC_API_KEY missing)")
	}
}
