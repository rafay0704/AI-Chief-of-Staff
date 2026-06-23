package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// HabitWindowDays is how many days of check-ins the grid shows.
const HabitWindowDays = 28

// HabitService manages habits and their daily check-ins.
type HabitService struct {
	repo repository.Querier
}

// NewHabitService constructs a HabitService.
func NewHabitService(repo repository.Querier) *HabitService {
	return &HabitService{repo: repo}
}

// Create adds a habit for the user.
func (s *HabitService) Create(ctx context.Context, userID uuid.UUID, name string) (domain.Habit, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Habit{}, fmt.Errorf("%w: habit name is required", domain.ErrValidation)
	}
	row, err := s.repo.CreateHabit(ctx, repository.CreateHabitParams{UserID: userID, Name: name})
	if err != nil {
		return domain.Habit{}, fmt.Errorf("create habit: %w", err)
	}
	return domain.Habit{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt, Checkins: []string{}}, nil
}

// List returns the user's habits, each with its recent check-ins and streak.
func (s *HabitService) List(ctx context.Context, userID uuid.UUID) ([]domain.Habit, error) {
	habits, err := s.repo.ListHabits(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	since := today.AddDate(0, 0, -(HabitWindowDays - 1))
	rows, err := s.repo.ListCheckinsSince(ctx, repository.ListCheckinsSinceParams{UserID: userID, Day: since})
	if err != nil {
		return nil, fmt.Errorf("list checkins: %w", err)
	}

	// Group check-in days per habit.
	byHabit := make(map[uuid.UUID]map[string]bool, len(habits))
	for _, r := range rows {
		set := byHabit[r.HabitID]
		if set == nil {
			set = make(map[string]bool)
			byHabit[r.HabitID] = set
		}
		set[r.Day.UTC().Format("2006-01-02")] = true
	}

	out := make([]domain.Habit, 0, len(habits))
	for _, h := range habits {
		set := byHabit[h.ID]
		days := make([]string, 0, len(set))
		for d := range set {
			days = append(days, d)
		}
		sort.Strings(days)
		out = append(out, domain.Habit{
			ID:        h.ID,
			Name:      h.Name,
			CreatedAt: h.CreatedAt,
			Streak:    currentStreak(set, today),
			Checkins:  days,
		})
	}
	return out, nil
}

// Delete removes a habit (and its check-ins via cascade).
func (s *HabitService) Delete(ctx context.Context, userID, habitID uuid.UUID) error {
	n, err := s.repo.DeleteHabit(ctx, repository.DeleteHabitParams{ID: habitID, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete habit: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Check records a check-in for a day (idempotent).
func (s *HabitService) Check(ctx context.Context, userID, habitID uuid.UUID, day string) error {
	d, err := s.ownedDay(ctx, userID, habitID, day)
	if err != nil {
		return err
	}
	return s.repo.CheckInHabit(ctx, repository.CheckInHabitParams{HabitID: habitID, Day: d})
}

// Uncheck removes a check-in for a day.
func (s *HabitService) Uncheck(ctx context.Context, userID, habitID uuid.UUID, day string) error {
	d, err := s.ownedDay(ctx, userID, habitID, day)
	if err != nil {
		return err
	}
	return s.repo.UncheckHabit(ctx, repository.UncheckHabitParams{HabitID: habitID, Day: d})
}

// ownedDay verifies the habit belongs to the user and parses the day.
func (s *HabitService) ownedDay(ctx context.Context, userID, habitID uuid.UUID, day string) (time.Time, error) {
	if _, err := s.repo.GetHabit(ctx, repository.GetHabitParams{ID: habitID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, domain.ErrNotFound
		}
		return time.Time{}, fmt.Errorf("get habit: %w", err)
	}
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: day must be YYYY-MM-DD", domain.ErrValidation)
	}
	return d, nil
}

// currentStreak counts consecutive checked days ending today (or yesterday, so a
// streak stays "alive" until the day is over).
func currentStreak(set map[string]bool, today time.Time) int {
	cur := today
	if !set[cur.Format("2006-01-02")] {
		cur = cur.AddDate(0, 0, -1)
		if !set[cur.Format("2006-01-02")] {
			return 0
		}
	}
	n := 0
	for set[cur.Format("2006-01-02")] {
		n++
		cur = cur.AddDate(0, 0, -1)
	}
	return n
}
