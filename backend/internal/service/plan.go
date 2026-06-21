package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// dateLayout is the canonical plan date format (YYYY-MM-DD).
const dateLayout = "2006-01-02"

// JobTypePlan is the queue job type for daily plan generation.
const JobTypePlan = "plan"

// PlanJobPayload is the queue payload for a plan generation job.
type PlanJobPayload struct {
	PlanID           uuid.UUID `json:"plan_id"`
	UserID           uuid.UUID `json:"user_id"`
	Date             string    `json:"date"`
	AvailableMinutes int       `json:"available_minutes"`
	Goals            []string  `json:"goals"`
}

// Enqueuer is the subset of the queue used by PlanService (eases testing).
type Enqueuer interface {
	Enqueue(ctx context.Context, job queue.Job) error
}

// PlanService manages daily plans and enqueues their async generation.
type PlanService struct {
	repo  repository.Querier
	queue Enqueuer
}

// NewPlanService constructs a PlanService.
func NewPlanService(repo repository.Querier, q Enqueuer) *PlanService {
	return &PlanService{repo: repo, queue: q}
}

// Generate creates (or resets) the plan for a date and enqueues a job to build
// it. It returns immediately with the queued plan; the worker fills it in.
func (s *PlanService) Generate(ctx context.Context, userID uuid.UUID, date string, availableMinutes int, goals []string) (domain.Plan, error) {
	d, err := time.Parse(dateLayout, date)
	if err != nil {
		return domain.Plan{}, fmt.Errorf("%w: date must be YYYY-MM-DD", domain.ErrValidation)
	}

	row, err := s.repo.UpsertPlanQueued(ctx, repository.UpsertPlanQueuedParams{UserID: userID, PlanDate: d})
	if err != nil {
		return domain.Plan{}, fmt.Errorf("upsert plan: %w", err)
	}

	job, err := queue.NewJob(JobTypePlan, PlanJobPayload{
		PlanID:           row.ID,
		UserID:           userID,
		Date:             date,
		AvailableMinutes: availableMinutes,
		Goals:            goals,
	})
	if err != nil {
		return domain.Plan{}, fmt.Errorf("build job: %w", err)
	}
	if err := s.queue.Enqueue(ctx, job); err != nil {
		return domain.Plan{}, fmt.Errorf("enqueue plan job: %w", err)
	}
	return toDomainPlan(row), nil
}

// GetByID returns a plan (job) by id, scoped to the user.
func (s *PlanService) GetByID(ctx context.Context, userID, planID uuid.UUID) (domain.Plan, error) {
	row, err := s.repo.GetPlanByID(ctx, repository.GetPlanByIDParams{ID: planID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Plan{}, domain.ErrNotFound
		}
		return domain.Plan{}, fmt.Errorf("get plan: %w", err)
	}
	return toDomainPlan(row), nil
}

// GetByDate returns the user's plan for a date.
func (s *PlanService) GetByDate(ctx context.Context, userID uuid.UUID, date string) (domain.Plan, error) {
	d, err := time.Parse(dateLayout, date)
	if err != nil {
		return domain.Plan{}, fmt.Errorf("%w: date must be YYYY-MM-DD", domain.ErrValidation)
	}
	row, err := s.repo.GetPlanByDate(ctx, repository.GetPlanByDateParams{UserID: userID, PlanDate: d})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Plan{}, domain.ErrNotFound
		}
		return domain.Plan{}, fmt.Errorf("get plan: %w", err)
	}
	return toDomainPlan(row), nil
}

// MarkRunning flags a plan as in-progress (called by the worker).
func (s *PlanService) MarkRunning(ctx context.Context, planID uuid.UUID) error {
	return s.repo.SetPlanRunning(ctx, planID)
}

// Complete stores the generated schedule and marks the plan done.
func (s *PlanService) Complete(ctx context.Context, planID uuid.UUID, scheduleJSON []byte) error {
	return s.repo.SetPlanDone(ctx, repository.SetPlanDoneParams{ID: planID, PlanJson: scheduleJSON})
}

// Fail records a failure reason and marks the plan failed.
func (s *PlanService) Fail(ctx context.Context, planID uuid.UUID, reason string) error {
	return s.repo.SetPlanFailed(ctx, repository.SetPlanFailedParams{ID: planID, Error: &reason})
}
