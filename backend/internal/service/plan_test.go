package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
)

// fakeEnqueuer records enqueued jobs.
type fakeEnqueuer struct{ jobs []queue.Job }

func (f *fakeEnqueuer) Enqueue(_ context.Context, job queue.Job) error {
	f.jobs = append(f.jobs, job)
	return nil
}

func TestGenerateEnqueuesAndPersistsQueued(t *testing.T) {
	repo := newFakeQuerier()
	enq := &fakeEnqueuer{}
	svc := NewPlanService(repo, enq)
	ctx := context.Background()
	userID := uuid.New()

	plan, err := svc.Generate(ctx, userID, "2026-06-22", 300, []string{"ship batch 4"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if plan.Status != domain.PlanQueued || plan.Date != "2026-06-22" {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	// A job was enqueued with the plan id in its payload.
	if len(enq.jobs) != 1 {
		t.Fatalf("expected 1 enqueued job, got %d", len(enq.jobs))
	}
	if enq.jobs[0].Type != JobTypePlan {
		t.Fatalf("unexpected job type %q", enq.jobs[0].Type)
	}
	var payload PlanJobPayload
	if err := enq.jobs[0].Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.PlanID != plan.ID || payload.AvailableMinutes != 300 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestGenerateRejectsBadDate(t *testing.T) {
	svc := NewPlanService(newFakeQuerier(), &fakeEnqueuer{})
	_, err := svc.Generate(context.Background(), uuid.New(), "22-06-2026", 300, nil)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestGenerateIsIdempotentPerDate(t *testing.T) {
	repo := newFakeQuerier()
	svc := NewPlanService(repo, &fakeEnqueuer{})
	ctx := context.Background()
	userID := uuid.New()

	p1, err := svc.Generate(ctx, userID, "2026-06-22", 300, nil)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	p2, err := svc.Generate(ctx, userID, "2026-06-22", 480, nil)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if p1.ID != p2.ID {
		t.Fatalf("re-generating the same date should reuse the plan row: %s != %s", p1.ID, p2.ID)
	}
}

func TestPlanLifecycleStatusTransitions(t *testing.T) {
	repo := newFakeQuerier()
	svc := NewPlanService(repo, &fakeEnqueuer{})
	ctx := context.Background()
	userID := uuid.New()

	plan, err := svc.Generate(ctx, userID, "2026-06-22", 300, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if err := svc.MarkRunning(ctx, plan.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := svc.Complete(ctx, plan.ID, []byte(`{"date":"2026-06-22","schedule":[]}`)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	got, err := svc.GetByID(ctx, userID, plan.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Status != domain.PlanDone || len(got.Schedule) == 0 {
		t.Fatalf("expected done plan with schedule, got %+v", got)
	}

	// Fetch by date returns the same plan.
	byDate, err := svc.GetByDate(ctx, userID, "2026-06-22")
	if err != nil {
		t.Fatalf("get by date: %v", err)
	}
	if byDate.ID != plan.ID {
		t.Fatalf("date lookup mismatch: %s != %s", byDate.ID, plan.ID)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	svc := NewPlanService(newFakeQuerier(), &fakeEnqueuer{})
	_, err := svc.GetByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
