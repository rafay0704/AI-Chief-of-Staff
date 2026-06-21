package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// TaskService handles task CRUD scoped to a single user.
type TaskService struct {
	repo repository.Querier
}

// NewTaskService constructs a TaskService.
func NewTaskService(repo repository.Querier) *TaskService {
	return &TaskService{repo: repo}
}

// CreateTaskInput is the validated input for creating a task.
type CreateTaskInput struct {
	Title           string
	Description     string
	Priority        domain.Priority
	DurationMinutes int32
}

// UpdateTaskInput holds optional fields; nil means "leave unchanged".
type UpdateTaskInput struct {
	Title           *string
	Description     *string
	Priority        *domain.Priority
	DurationMinutes *int32
	Status          *domain.TaskStatus
}

// ListTaskFilter holds optional list filters.
type ListTaskFilter struct {
	Status   *domain.TaskStatus
	Priority *domain.Priority
}

// Create inserts a new task for the user.
func (s *TaskService) Create(ctx context.Context, userID uuid.UUID, in CreateTaskInput) (domain.Task, error) {
	priority := in.Priority
	if priority == "" {
		priority = domain.PriorityMedium
	}
	row, err := s.repo.CreateTask(ctx, repository.CreateTaskParams{
		UserID:          userID,
		Title:           in.Title,
		Description:     in.Description,
		Priority:        repository.TaskPriority(priority),
		DurationMinutes: in.DurationMinutes,
	})
	if err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	return toDomainTask(row), nil
}

// Get returns one task owned by the user.
func (s *TaskService) Get(ctx context.Context, userID, taskID uuid.UUID) (domain.Task, error) {
	row, err := s.repo.GetTask(ctx, repository.GetTaskParams{ID: taskID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("get task: %w", err)
	}
	return toDomainTask(row), nil
}

// List returns the user's tasks, optionally filtered.
func (s *TaskService) List(ctx context.Context, userID uuid.UUID, f ListTaskFilter) ([]domain.Task, error) {
	params := repository.ListTasksParams{UserID: userID}
	if f.Status != nil {
		st := repository.TaskStatus(*f.Status)
		params.Status = &st
	}
	if f.Priority != nil {
		pr := repository.TaskPriority(*f.Priority)
		params.Priority = &pr
	}

	rows, err := s.repo.ListTasks(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	tasks := make([]domain.Task, 0, len(rows))
	for _, r := range rows {
		tasks = append(tasks, toDomainTask(r))
	}
	return tasks, nil
}

// Update applies a partial update to a task owned by the user.
func (s *TaskService) Update(ctx context.Context, userID, taskID uuid.UUID, in UpdateTaskInput) (domain.Task, error) {
	params := repository.UpdateTaskParams{
		ID:              taskID,
		UserID:          userID,
		Title:           in.Title,
		Description:     in.Description,
		DurationMinutes: in.DurationMinutes,
	}
	if in.Priority != nil {
		pr := repository.TaskPriority(*in.Priority)
		params.Priority = &pr
	}
	if in.Status != nil {
		st := repository.TaskStatus(*in.Status)
		params.Status = &st
	}

	row, err := s.repo.UpdateTask(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	return toDomainTask(row), nil
}

// Delete removes a task owned by the user.
func (s *TaskService) Delete(ctx context.Context, userID, taskID uuid.UUID) error {
	n, err := s.repo.DeleteTask(ctx, repository.DeleteTaskParams{ID: taskID, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
