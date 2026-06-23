package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

const trendDays = 7

// StatsService computes productivity analytics from the user's tasks and plans.
type StatsService struct {
	repo repository.Querier
}

// NewStatsService constructs a StatsService.
func NewStatsService(repo repository.Querier) *StatsService {
	return &StatsService{repo: repo}
}

// Get returns the user's productivity snapshot.
func (s *StatsService) Get(ctx context.Context, userID uuid.UUID) (domain.Stats, error) {
	ts, err := s.repo.TaskStats(ctx, userID)
	if err != nil {
		return domain.Stats{}, fmt.Errorf("task stats: %w", err)
	}
	plans, err := s.repo.PlanCount(ctx, userID)
	if err != nil {
		return domain.Stats{}, fmt.Errorf("plan count: %w", err)
	}

	// 7-day completion trend, zero-filled and ordered oldest → newest.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	since := today.AddDate(0, 0, -(trendDays - 1))
	rows, err := s.repo.CompletionTrend(ctx, repository.CompletionTrendParams{UserID: userID, UpdatedAt: since})
	if err != nil {
		return domain.Stats{}, fmt.Errorf("completion trend: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.Day.UTC().Format("2006-01-02")] = int(r.Count)
	}
	trend := make([]domain.DayCount, 0, trendDays)
	for i := 0; i < trendDays; i++ {
		d := since.AddDate(0, 0, i).Format("2006-01-02")
		trend = append(trend, domain.DayCount{Date: d, Completed: counts[d]})
	}

	rate := 0.0
	if ts.Total > 0 {
		rate = float64(ts.Completed) / float64(ts.Total)
	}

	return domain.Stats{
		TotalTasks:       int(ts.Total),
		Completed:        int(ts.Completed),
		Pending:          int(ts.Pending),
		CompletionRate:   rate,
		PendingMinutes:   int(ts.PendingMinutes),
		CompletedMinutes: int(ts.CompletedMinutes),
		ByPriority: domain.PriorityCounts{
			High:   int(ts.High),
			Medium: int(ts.Medium),
			Low:    int(ts.Low),
		},
		PlansGenerated: int(plans),
		Trend:          trend,
	}, nil
}
