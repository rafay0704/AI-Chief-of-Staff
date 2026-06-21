package ai_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/ai"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/config"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// TestLivePlanGeneration hits the real Claude API. It is skipped in -short mode
// and when no ANTHROPIC_API_KEY is configured (via env or repo-root .env).
func TestLivePlanGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live API test in -short mode")
	}

	// Best-effort load of the repo-root .env so the key is available locally.
	cfg, err := config.Load("../../../.env", "../../.env", "../.env", ".env")
	if err != nil || cfg.AnthropicAPIKey == "" {
		t.Skip("skipping live API test: ANTHROPIC_API_KEY not set")
	}

	client, err := ai.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicModel, ai.WithTimeout(45*time.Second))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	agents := ai.NewAgents(client)

	tasks := []domain.Task{
		{ID: uuid.New(), Title: "Write the worker pool", Priority: domain.PriorityHigh, DurationMinutes: 120, Status: domain.StatusPending},
		{ID: uuid.New(), Title: "Review pull requests", Priority: domain.PriorityMedium, DurationMinutes: 45, Status: domain.StatusPending},
		{ID: uuid.New(), Title: "Inbox zero", Priority: domain.PriorityLow, DurationMinutes: 30, Status: domain.StatusPending},
	}

	sched, err := agents.Plan(context.Background(), ai.PlanInput{
		Date:             "2026-06-22",
		AvailableMinutes: 300,
		Goals:            []string{"ship the async worker batch"},
		Tasks:            tasks,
	})
	if err != nil {
		t.Fatalf("live plan generation: %v", err)
	}
	if len(sched.Schedule) == 0 {
		t.Fatal("live plan returned an empty schedule")
	}
	t.Logf("live plan: %d blocks, summary=%q", len(sched.Schedule), sched.Summary)
}
