package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

const defaultAvailableMinutes = 480

type generatePlanRequest struct {
	Date             string   `json:"date" binding:"required"`
	AvailableMinutes int      `json:"available_minutes" binding:"omitempty,gt=0"`
	Goals            []string `json:"goals" binding:"omitempty,dive,max=200"`
}

// GeneratePlan handles POST /plans/generate — enqueues a planning job.
func (h *Handler) GeneratePlan(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}

	var req generatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondValidation(c, "invalid plan payload", err.Error())
		return
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		h.respondValidation(c, "date must be YYYY-MM-DD", nil)
		return
	}
	minutes := req.AvailableMinutes
	if minutes == 0 {
		minutes = defaultAvailableMinutes
	}

	plan, err := h.Plans.Generate(c.Request.Context(), uid, req.Date, minutes, req.Goals)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"job_id": plan.ID,
		"status": plan.Status,
		"date":   plan.Date,
	})
}

// GetPlanJob handles GET /plans/jobs/:id — poll a planning job's status.
func (h *Handler) GetPlanJob(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondValidation(c, "invalid job id", nil)
		return
	}

	plan, err := h.Plans.GetByID(c.Request.Context(), uid, jobID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	resp := gin.H{"job_id": plan.ID, "status": plan.Status, "date": plan.Date}
	if len(plan.Schedule) > 0 {
		resp["schedule"] = plan.Schedule
	}
	if plan.Error != "" {
		resp["error"] = plan.Error
	}
	c.JSON(http.StatusOK, resp)
}

// GetPlanByDate handles GET /plans?date=YYYY-MM-DD — fetch the plan for a date.
func (h *Handler) GetPlanByDate(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	date := c.Query("date")
	if date == "" {
		h.respondValidation(c, "date query parameter is required (YYYY-MM-DD)", nil)
		return
	}

	plan, err := h.Plans.GetByDate(c.Request.Context(), uid, date)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}
