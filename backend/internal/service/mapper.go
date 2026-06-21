package service

import (
	"encoding/json"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

const planDateLayout = "2006-01-02"

// toDomainPlan converts a repository row to a domain plan.
func toDomainPlan(p repository.DailyPlan) domain.Plan {
	plan := domain.Plan{
		ID:        p.ID,
		UserID:    p.UserID,
		Date:      p.PlanDate.Format(planDateLayout),
		Status:    domain.PlanStatus(p.Status),
		Schedule:  json.RawMessage(p.PlanJson),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if p.Error != nil {
		plan.Error = *p.Error
	}
	return plan
}

// toDomainUser converts a repository row to the public domain user
// (dropping the password hash).
func toDomainUser(u repository.User) domain.User {
	return domain.User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

// toDomainTask converts a repository row to a domain task.
func toDomainTask(t repository.Task) domain.Task {
	return domain.Task{
		ID:              t.ID,
		UserID:          t.UserID,
		Title:           t.Title,
		Description:     t.Description,
		Priority:        domain.Priority(t.Priority),
		DurationMinutes: t.DurationMinutes,
		Status:          domain.TaskStatus(t.Status),
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}
