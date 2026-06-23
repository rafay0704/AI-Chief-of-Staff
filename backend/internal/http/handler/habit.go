package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

type createHabitRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

type checkinRequest struct {
	Date string `json:"date" binding:"omitempty"`
}

// ListHabits handles GET /habits.
func (h *Handler) ListHabits(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	habits, err := h.Habits.List(c.Request.Context(), uid)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"habits": habits})
}

// CreateHabit handles POST /habits.
func (h *Handler) CreateHabit(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	var req createHabitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondValidation(c, "invalid habit payload", err.Error())
		return
	}
	habit, err := h.Habits.Create(c.Request.Context(), uid, req.Name)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, habit)
}

// DeleteHabit handles DELETE /habits/:id.
func (h *Handler) DeleteHabit(c *gin.Context) {
	uid, habitID, ok := h.userAndHabitID(c)
	if !ok {
		return
	}
	if err := h.Habits.Delete(c.Request.Context(), uid, habitID); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CheckHabit handles POST /habits/:id/checkin (body: optional date, default today).
func (h *Handler) CheckHabit(c *gin.Context) {
	uid, habitID, ok := h.userAndHabitID(c)
	if !ok {
		return
	}
	var req checkinRequest
	_ = c.ShouldBindJSON(&req) // body is optional
	if err := h.Habits.Check(c.Request.Context(), uid, habitID, dayOrToday(req.Date)); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UncheckHabit handles DELETE /habits/:id/checkin?date=YYYY-MM-DD (default today).
func (h *Handler) UncheckHabit(c *gin.Context) {
	uid, habitID, ok := h.userAndHabitID(c)
	if !ok {
		return
	}
	if err := h.Habits.Uncheck(c.Request.Context(), uid, habitID, dayOrToday(c.Query("date"))); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) userAndHabitID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}
	habitID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondValidation(c, "invalid habit id", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return uid, habitID, true
}

func dayOrToday(s string) string {
	if s == "" {
		return time.Now().UTC().Format("2006-01-02")
	}
	return s
}
