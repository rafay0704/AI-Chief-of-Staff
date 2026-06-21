package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

func TestTaskCRUD(t *testing.T) {
	svc := NewTaskService(newFakeQuerier())
	ctx := context.Background()
	userID := uuid.New()

	// Create (empty priority should default to medium).
	created, err := svc.Create(ctx, userID, CreateTaskInput{
		Title:           "Learn channels",
		DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Priority != domain.PriorityMedium {
		t.Fatalf("expected default priority medium, got %q", created.Priority)
	}

	// Get.
	got, err := svc.Get(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Learn channels" {
		t.Fatalf("unexpected title %q", got.Title)
	}

	// Update status to completed.
	completed := domain.StatusCompleted
	updated, err := svc.Update(ctx, userID, created.ID, UpdateTaskInput{Status: &completed})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != domain.StatusCompleted {
		t.Fatalf("expected completed, got %q", updated.Status)
	}

	// List.
	tasks, err := svc.List(ctx, userID, ListTaskFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// Delete.
	if err := svc.Delete(ctx, userID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(ctx, userID, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTaskScopedToOwner(t *testing.T) {
	svc := NewTaskService(newFakeQuerier())
	ctx := context.Background()
	owner, other := uuid.New(), uuid.New()

	created, err := svc.Create(ctx, owner, CreateTaskInput{Title: "private", DurationMinutes: 30})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// A different user must not see or fetch it.
	if _, err := svc.Get(ctx, other, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for other user, got %v", err)
	}
	if err := svc.Delete(ctx, other, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting other's task, got %v", err)
	}
}

func TestDeleteMissingTaskNotFound(t *testing.T) {
	svc := NewTaskService(newFakeQuerier())
	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
