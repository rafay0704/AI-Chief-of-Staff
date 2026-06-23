package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

func TestStatsAggregates(t *testing.T) {
	repo := newFakeQuerier()
	ctx := context.Background()
	userID := uuid.New()

	// 2 high, 1 low; complete one of them.
	mk := func(pr repository.TaskPriority, mins int32) repository.Task {
		task, _ := repo.CreateTask(ctx, repository.CreateTaskParams{
			UserID: userID, Title: "t", Priority: pr, DurationMinutes: mins,
		})
		return task
	}
	t1 := mk(repository.TaskPriorityHigh, 60)
	mk(repository.TaskPriorityHigh, 30)
	mk(repository.TaskPriorityLow, 15)
	completed := repository.TaskStatusCompleted
	if _, err := repo.UpdateTask(ctx, repository.UpdateTaskParams{ID: t1.ID, UserID: userID, Status: &completed}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	svc := NewStatsService(repo)
	s, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if s.TotalTasks != 3 || s.Completed != 1 || s.Pending != 2 {
		t.Fatalf("unexpected totals: %+v", s)
	}
	if s.ByPriority.High != 2 || s.ByPriority.Low != 1 {
		t.Fatalf("unexpected priority counts: %+v", s.ByPriority)
	}
	if s.CompletedMinutes != 60 || s.PendingMinutes != 45 {
		t.Fatalf("unexpected minutes: %+v", s)
	}
	wantRate := 1.0 / 3.0
	if s.CompletionRate < wantRate-0.001 || s.CompletionRate > wantRate+0.001 {
		t.Fatalf("unexpected rate %.3f", s.CompletionRate)
	}
	if len(s.Trend) != 7 {
		t.Fatalf("expected 7 trend days, got %d", len(s.Trend))
	}
	// The completed task's check-in lands on today (last trend bucket).
	if s.Trend[6].Completed != 1 {
		t.Fatalf("expected today's completion in trend, got %+v", s.Trend[6])
	}
}

func TestStatsEmptyUser(t *testing.T) {
	svc := NewStatsService(newFakeQuerier())
	s, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if s.TotalTasks != 0 || s.CompletionRate != 0 || len(s.Trend) != 7 {
		t.Fatalf("unexpected empty stats: %+v", s)
	}
}
