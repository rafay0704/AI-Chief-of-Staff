package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// fakeQuerier is an in-memory implementation of repository.Querier for unit
// tests, so the service layer can be exercised without a real database.
type fakeQuerier struct {
	users    map[uuid.UUID]repository.User
	tasks    map[uuid.UUID]repository.Task
	plans    map[uuid.UUID]repository.DailyPlan
	habits   map[uuid.UUID]repository.Habit
	checkins map[uuid.UUID]map[string]bool // habitID -> day -> true
}

func newFakeQuerier() *fakeQuerier {
	return &fakeQuerier{
		users:    make(map[uuid.UUID]repository.User),
		tasks:    make(map[uuid.UUID]repository.Task),
		plans:    make(map[uuid.UUID]repository.DailyPlan),
		habits:   make(map[uuid.UUID]repository.Habit),
		checkins: make(map[uuid.UUID]map[string]bool),
	}
}

var _ repository.Querier = (*fakeQuerier)(nil)

func (f *fakeQuerier) CreateUser(_ context.Context, arg repository.CreateUserParams) (repository.User, error) {
	for _, u := range f.users {
		if u.Email == arg.Email {
			return repository.User{}, &pgconn.PgError{Code: "23505"}
		}
	}
	now := time.Now()
	u := repository.User{
		ID:           uuid.New(),
		Name:         arg.Name,
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeQuerier) GetUserByEmail(_ context.Context, email string) (repository.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeQuerier) GetUserByID(_ context.Context, id uuid.UUID) (repository.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return repository.User{}, pgx.ErrNoRows
}

func (f *fakeQuerier) CreateTask(_ context.Context, arg repository.CreateTaskParams) (repository.Task, error) {
	now := time.Now()
	t := repository.Task{
		ID:              uuid.New(),
		UserID:          arg.UserID,
		Title:           arg.Title,
		Description:     arg.Description,
		Priority:        arg.Priority,
		DurationMinutes: arg.DurationMinutes,
		Status:          repository.TaskStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	f.tasks[t.ID] = t
	return t, nil
}

func (f *fakeQuerier) GetTask(_ context.Context, arg repository.GetTaskParams) (repository.Task, error) {
	if t, ok := f.tasks[arg.ID]; ok && t.UserID == arg.UserID {
		return t, nil
	}
	return repository.Task{}, pgx.ErrNoRows
}

func (f *fakeQuerier) ListTasks(_ context.Context, arg repository.ListTasksParams) ([]repository.Task, error) {
	out := []repository.Task{}
	for _, t := range f.tasks {
		if t.UserID != arg.UserID {
			continue
		}
		if arg.Status != nil && t.Status != *arg.Status {
			continue
		}
		if arg.Priority != nil && t.Priority != *arg.Priority {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeQuerier) UpdateTask(_ context.Context, arg repository.UpdateTaskParams) (repository.Task, error) {
	t, ok := f.tasks[arg.ID]
	if !ok || t.UserID != arg.UserID {
		return repository.Task{}, pgx.ErrNoRows
	}
	if arg.Title != nil {
		t.Title = *arg.Title
	}
	if arg.Description != nil {
		t.Description = *arg.Description
	}
	if arg.Priority != nil {
		t.Priority = *arg.Priority
	}
	if arg.DurationMinutes != nil {
		t.DurationMinutes = *arg.DurationMinutes
	}
	if arg.Status != nil {
		t.Status = *arg.Status
	}
	t.UpdatedAt = time.Now()
	f.tasks[t.ID] = t
	return t, nil
}

func (f *fakeQuerier) DeleteTask(_ context.Context, arg repository.DeleteTaskParams) (int64, error) {
	if t, ok := f.tasks[arg.ID]; ok && t.UserID == arg.UserID {
		delete(f.tasks, arg.ID)
		return 1, nil
	}
	return 0, nil
}

func (f *fakeQuerier) UpsertPlanQueued(_ context.Context, arg repository.UpsertPlanQueuedParams) (repository.DailyPlan, error) {
	for id, p := range f.plans {
		if p.UserID == arg.UserID && p.PlanDate.Equal(arg.PlanDate) {
			p.Status = repository.PlanStatusQueued
			p.PlanJson = nil
			p.Error = nil
			p.UpdatedAt = time.Now()
			f.plans[id] = p
			return p, nil
		}
	}
	now := time.Now()
	p := repository.DailyPlan{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		PlanDate:  arg.PlanDate,
		Status:    repository.PlanStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	f.plans[p.ID] = p
	return p, nil
}

func (f *fakeQuerier) GetPlanByID(_ context.Context, arg repository.GetPlanByIDParams) (repository.DailyPlan, error) {
	if p, ok := f.plans[arg.ID]; ok && p.UserID == arg.UserID {
		return p, nil
	}
	return repository.DailyPlan{}, pgx.ErrNoRows
}

func (f *fakeQuerier) GetPlanByDate(_ context.Context, arg repository.GetPlanByDateParams) (repository.DailyPlan, error) {
	for _, p := range f.plans {
		if p.UserID == arg.UserID && p.PlanDate.Equal(arg.PlanDate) {
			return p, nil
		}
	}
	return repository.DailyPlan{}, pgx.ErrNoRows
}

func (f *fakeQuerier) SetPlanRunning(_ context.Context, id uuid.UUID) error {
	if p, ok := f.plans[id]; ok {
		p.Status = repository.PlanStatusRunning
		f.plans[id] = p
	}
	return nil
}

func (f *fakeQuerier) SetPlanDone(_ context.Context, arg repository.SetPlanDoneParams) error {
	if p, ok := f.plans[arg.ID]; ok {
		p.Status = repository.PlanStatusDone
		p.PlanJson = arg.PlanJson
		p.Error = nil
		f.plans[arg.ID] = p
	}
	return nil
}

func (f *fakeQuerier) SetPlanFailed(_ context.Context, arg repository.SetPlanFailedParams) error {
	if p, ok := f.plans[arg.ID]; ok {
		p.Status = repository.PlanStatusFailed
		p.Error = arg.Error
		f.plans[arg.ID] = p
	}
	return nil
}

func (f *fakeQuerier) TaskStats(_ context.Context, userID uuid.UUID) (repository.TaskStatsRow, error) {
	var r repository.TaskStatsRow
	for _, t := range f.tasks {
		if t.UserID != userID {
			continue
		}
		r.Total++
		switch t.Status {
		case repository.TaskStatusCompleted:
			r.Completed++
			r.CompletedMinutes += t.DurationMinutes
		case repository.TaskStatusPending:
			r.Pending++
			r.PendingMinutes += t.DurationMinutes
		}
		switch t.Priority {
		case repository.TaskPriorityHigh:
			r.High++
		case repository.TaskPriorityMedium:
			r.Medium++
		case repository.TaskPriorityLow:
			r.Low++
		}
	}
	return r, nil
}

func (f *fakeQuerier) PlanCount(_ context.Context, userID uuid.UUID) (int32, error) {
	var n int32
	for _, p := range f.plans {
		if p.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (f *fakeQuerier) CompletionTrend(_ context.Context, arg repository.CompletionTrendParams) ([]repository.CompletionTrendRow, error) {
	byDay := map[string]int32{}
	for _, t := range f.tasks {
		if t.UserID == arg.UserID && t.Status == repository.TaskStatusCompleted && !t.UpdatedAt.Before(arg.UpdatedAt) {
			byDay[t.UpdatedAt.UTC().Format("2006-01-02")]++
		}
	}
	out := []repository.CompletionTrendRow{}
	for d, c := range byDay {
		day, _ := time.Parse("2006-01-02", d)
		out = append(out, repository.CompletionTrendRow{Day: day, Count: c})
	}
	return out, nil
}

func (f *fakeQuerier) CreateHabit(_ context.Context, arg repository.CreateHabitParams) (repository.Habit, error) {
	h := repository.Habit{ID: uuid.New(), UserID: arg.UserID, Name: arg.Name, CreatedAt: time.Now()}
	f.habits[h.ID] = h
	return h, nil
}

func (f *fakeQuerier) ListHabits(_ context.Context, userID uuid.UUID) ([]repository.Habit, error) {
	out := []repository.Habit{}
	for _, h := range f.habits {
		if h.UserID == userID {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *fakeQuerier) GetHabit(_ context.Context, arg repository.GetHabitParams) (repository.Habit, error) {
	if h, ok := f.habits[arg.ID]; ok && h.UserID == arg.UserID {
		return h, nil
	}
	return repository.Habit{}, pgx.ErrNoRows
}

func (f *fakeQuerier) DeleteHabit(_ context.Context, arg repository.DeleteHabitParams) (int64, error) {
	if h, ok := f.habits[arg.ID]; ok && h.UserID == arg.UserID {
		delete(f.habits, arg.ID)
		delete(f.checkins, arg.ID)
		return 1, nil
	}
	return 0, nil
}

func (f *fakeQuerier) CheckInHabit(_ context.Context, arg repository.CheckInHabitParams) error {
	set := f.checkins[arg.HabitID]
	if set == nil {
		set = map[string]bool{}
		f.checkins[arg.HabitID] = set
	}
	set[arg.Day.UTC().Format("2006-01-02")] = true
	return nil
}

func (f *fakeQuerier) UncheckHabit(_ context.Context, arg repository.UncheckHabitParams) error {
	if set := f.checkins[arg.HabitID]; set != nil {
		delete(set, arg.Day.UTC().Format("2006-01-02"))
	}
	return nil
}

func (f *fakeQuerier) ListCheckinsSince(_ context.Context, arg repository.ListCheckinsSinceParams) ([]repository.ListCheckinsSinceRow, error) {
	out := []repository.ListCheckinsSinceRow{}
	for hid, set := range f.checkins {
		h, ok := f.habits[hid]
		if !ok || h.UserID != arg.UserID {
			continue
		}
		for d := range set {
			day, _ := time.Parse("2006-01-02", d)
			if !day.Before(arg.Day) {
				out = append(out, repository.ListCheckinsSinceRow{HabitID: hid, Day: day})
			}
		}
	}
	return out, nil
}
