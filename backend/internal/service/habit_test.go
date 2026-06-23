package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

func TestHabitCreateListDelete(t *testing.T) {
	svc := NewHabitService(newFakeQuerier())
	ctx := context.Background()
	userID := uuid.New()

	h, err := svc.Create(ctx, userID, "  Meditate  ")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if h.Name != "Meditate" { // trimmed
		t.Fatalf("expected trimmed name, got %q", h.Name)
	}

	list, err := svc.List(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Streak != 0 {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := svc.Delete(ctx, userID, h.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.Delete(ctx, userID, h.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestHabitCreateEmptyNameRejected(t *testing.T) {
	svc := NewHabitService(newFakeQuerier())
	if _, err := svc.Create(context.Background(), uuid.New(), "   "); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestHabitStreakAndCheckins(t *testing.T) {
	svc := NewHabitService(newFakeQuerier())
	ctx := context.Background()
	userID := uuid.New()

	h, _ := svc.Create(ctx, userID, "Read")
	today := time.Now().UTC()
	// Check in today, yesterday, and the day before → streak 3.
	for i := 0; i < 3; i++ {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		if err := svc.Check(ctx, userID, h.ID, d); err != nil {
			t.Fatalf("check %s: %v", d, err)
		}
	}
	// Idempotent: checking today again changes nothing.
	_ = svc.Check(ctx, userID, h.ID, today.Format("2006-01-02"))

	list, _ := svc.List(ctx, userID)
	if list[0].Streak != 3 {
		t.Fatalf("expected streak 3, got %d", list[0].Streak)
	}
	if len(list[0].Checkins) != 3 {
		t.Fatalf("expected 3 checkins, got %v", list[0].Checkins)
	}

	// Uncheck today → streak stays alive at 2 (yesterday + day before).
	if err := svc.Uncheck(ctx, userID, h.ID, today.Format("2006-01-02")); err != nil {
		t.Fatalf("uncheck: %v", err)
	}
	list, _ = svc.List(ctx, userID)
	if list[0].Streak != 2 {
		t.Fatalf("expected streak 2 after unchecking today, got %d", list[0].Streak)
	}
}

func TestHabitCheckRejectsForeignHabit(t *testing.T) {
	svc := NewHabitService(newFakeQuerier())
	ctx := context.Background()
	owner := uuid.New()
	h, _ := svc.Create(ctx, owner, "Private")

	other := uuid.New()
	err := svc.Check(ctx, other, h.ID, time.Now().UTC().Format("2006-01-02"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound checking another user's habit, got %v", err)
	}
}
