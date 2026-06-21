package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// fakeCompleter returns canned responses in sequence, recording each call.
type fakeCompleter struct {
	responses []string
	errs      []error
	calls     int
}

func (f *fakeCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	i := f.calls
	f.calls++
	var resp string
	if i < len(f.responses) {
		resp = f.responses[i]
	}
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	return resp, err
}

func sampleTasks() []domain.Task {
	return []domain.Task{
		{ID: uuid.New(), Title: "Learn Go concurrency", Priority: domain.PriorityHigh, DurationMinutes: 90, Status: domain.StatusPending},
		{ID: uuid.New(), Title: "Email triage", Priority: domain.PriorityLow, DurationMinutes: 30, Status: domain.StatusPending},
	}
}

func TestPlanParsesValidJSON(t *testing.T) {
	raw := `{"date":"2026-06-22","schedule":[{"time":"09:00 - 10:30","task":"Learn Go concurrency","type":"focus"},{"time":"10:30 - 11:00","task":"Break","type":"rest"}],"summary":"Focused morning"}`
	a := NewAgents(&fakeCompleter{responses: []string{raw}})

	sched, err := a.Plan(context.Background(), PlanInput{Date: "2026-06-22", AvailableMinutes: 240, Tasks: sampleTasks()})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(sched.Schedule) != 2 || sched.Date != "2026-06-22" {
		t.Fatalf("unexpected schedule: %+v", sched)
	}
}

func TestPlanStripsMarkdownFences(t *testing.T) {
	raw := "```json\n{\"date\":\"2026-06-22\",\"schedule\":[{\"time\":\"09:00 - 10:00\",\"task\":\"X\",\"type\":\"focus\"}],\"summary\":\"ok\"}\n```"
	a := NewAgents(&fakeCompleter{responses: []string{raw}})

	if _, err := a.Plan(context.Background(), PlanInput{Date: "2026-06-22", Tasks: sampleTasks()}); err != nil {
		t.Fatalf("expected fenced JSON to parse, got %v", err)
	}
}

func TestPlanRepairsAfterInvalidFirstResponse(t *testing.T) {
	bad := `{"date":"2026-06-22","schedule":[],"summary":"empty"}` // fails validation: no blocks
	good := `{"date":"2026-06-22","schedule":[{"time":"09:00 - 10:00","task":"X","type":"focus"}],"summary":"ok"}`
	fc := &fakeCompleter{responses: []string{bad, good}}
	a := NewAgents(fc)

	if _, err := a.Plan(context.Background(), PlanInput{Date: "2026-06-22", Tasks: sampleTasks()}); err != nil {
		t.Fatalf("expected repair to succeed, got %v", err)
	}
	if fc.calls != 2 {
		t.Fatalf("expected exactly 2 completions (initial + repair), got %d", fc.calls)
	}
}

func TestPlanFailsAfterTwoBadResponses(t *testing.T) {
	bad := `{"date":"","schedule":[],"summary":""}`
	a := NewAgents(&fakeCompleter{responses: []string{bad, bad}})

	if _, err := a.Plan(context.Background(), PlanInput{Date: "2026-06-22", Tasks: sampleTasks()}); err == nil {
		t.Fatal("expected error after two invalid responses")
	}
}

func TestPrioritizeAndBreakParse(t *testing.T) {
	id := uuid.New()
	prio := `{"ranked":[{"task_id":"` + id.String() + `","rank":1,"reason":"blocks","urgent":true}],"drop_suggestions":[]}`
	brk := `{"task_id":"` + id.String() + `","steps":[{"order":1,"title":"Read docs","duration_minutes":30}]}`
	a := NewAgents(&fakeCompleter{responses: []string{prio, brk}})

	pr, err := a.Prioritize(context.Background(), sampleTasks())
	if err != nil {
		t.Fatalf("prioritize: %v", err)
	}
	if len(pr.Ranked) != 1 || pr.Ranked[0].Rank != 1 {
		t.Fatalf("unexpected priority result: %+v", pr)
	}

	bd, err := a.Break(context.Background(), sampleTasks()[0])
	if err != nil {
		t.Fatalf("break: %v", err)
	}
	if len(bd.Steps) != 1 {
		t.Fatalf("unexpected breakdown: %+v", bd)
	}
}

func TestCompleterErrorPropagates(t *testing.T) {
	a := NewAgents(&fakeCompleter{errs: []error{errors.New("network down")}})
	if _, err := a.Plan(context.Background(), PlanInput{Date: "2026-06-22", Tasks: sampleTasks()}); err == nil {
		t.Fatal("expected completer error to propagate")
	}
}

func TestNewClientRequiresAPIKey(t *testing.T) {
	if _, err := NewClient("", "claude-haiku-4-5-20251001"); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}
