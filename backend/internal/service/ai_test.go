package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/ai"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// stubCompleter returns a fixed JSON response, satisfying ai.Completer.
type stubCompleter struct{ resp string }

func (s stubCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	return s.resp, nil
}

func seedTask(t *testing.T, repo *fakeQuerier, userID uuid.UUID, title string) repository.Task {
	t.Helper()
	task, err := repo.CreateTask(context.Background(), repository.CreateTaskParams{
		UserID:          userID,
		Title:           title,
		Priority:        repository.TaskPriorityHigh,
		DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task
}

func TestPrioritizeReturnsRanking(t *testing.T) {
	repo := newFakeQuerier()
	userID := uuid.New()
	task := seedTask(t, repo, userID, "Write tests")

	agents := ai.NewAgents(stubCompleter{
		resp: `{"ranked":[{"task_id":"` + task.ID.String() + `","rank":1,"reason":"blocks release","urgent":true}],"drop_suggestions":[]}`,
	})
	svc := NewAIService(NewTaskService(repo), NewStatsService(repo), agents)

	res, err := svc.Prioritize(context.Background(), userID)
	if err != nil {
		t.Fatalf("prioritize: %v", err)
	}
	if len(res.Ranked) != 1 || res.Ranked[0].Rank != 1 || !res.Ranked[0].Urgent {
		t.Fatalf("unexpected ranking: %+v", res)
	}
}

func TestPrioritizeNoTasksIsValidationError(t *testing.T) {
	svc := NewAIService(NewTaskService(newFakeQuerier()), NewStatsService(newFakeQuerier()), ai.NewAgents(stubCompleter{}))
	_, err := svc.Prioritize(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestBreakdownReturnsSteps(t *testing.T) {
	repo := newFakeQuerier()
	userID := uuid.New()
	task := seedTask(t, repo, userID, "Build the worker pool")

	agents := ai.NewAgents(stubCompleter{
		resp: `{"task_id":"` + task.ID.String() + `","steps":[{"order":1,"title":"Sketch the API","duration_minutes":20},{"order":2,"title":"Implement","duration_minutes":40}]}`,
	})
	svc := NewAIService(NewTaskService(repo), NewStatsService(repo), agents)

	res, err := svc.Breakdown(context.Background(), userID, task.ID)
	if err != nil {
		t.Fatalf("breakdown: %v", err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %+v", res)
	}
}

func TestBreakdownMissingTaskIsNotFound(t *testing.T) {
	svc := NewAIService(NewTaskService(newFakeQuerier()), NewStatsService(newFakeQuerier()), ai.NewAgents(stubCompleter{resp: "{}"}))
	_, err := svc.Breakdown(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWeeklyReportReturnsNarrative(t *testing.T) {
	repo := newFakeQuerier()
	userID := uuid.New()
	seedTask(t, repo, userID, "Ship the release")

	agents := ai.NewAgents(stubCompleter{
		resp: `{"headline":"Strong week","summary":"You shipped the release.","wins":["Shipped"],"watch_outs":[],"suggestions":["Rest"]}`,
	})
	svc := NewAIService(NewTaskService(repo), NewStatsService(repo), agents)

	rep, err := svc.WeeklyReport(context.Background(), userID)
	if err != nil {
		t.Fatalf("weekly report: %v", err)
	}
	if rep.Headline == "" || rep.Summary == "" {
		t.Fatalf("expected narrative fields, got %+v", rep)
	}
}

func TestWeeklyReportNoTasksIsValidationError(t *testing.T) {
	svc := NewAIService(NewTaskService(newFakeQuerier()), NewStatsService(newFakeQuerier()), ai.NewAgents(stubCompleter{}))
	if _, err := svc.WeeklyReport(context.Background(), uuid.New()); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestAIUnavailableWhenAgentsNil(t *testing.T) {
	svc := NewAIService(NewTaskService(newFakeQuerier()), NewStatsService(newFakeQuerier()), nil)
	if _, err := svc.Prioritize(context.Background(), uuid.New()); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable from Prioritize, got %v", err)
	}
	if _, err := svc.Breakdown(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable from Breakdown, got %v", err)
	}
	if _, err := svc.WeeklyReport(context.Background(), uuid.New()); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable from WeeklyReport, got %v", err)
	}
}
