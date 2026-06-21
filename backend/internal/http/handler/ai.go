package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// Prioritize handles POST /ai/prioritize — rank the user's pending tasks.
func (h *Handler) Prioritize(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	result, err := h.AI.Prioritize(c.Request.Context(), uid)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// BreakdownTask handles POST /ai/breakdown/:id — split a task into steps.
func (h *Handler) BreakdownTask(c *gin.Context) {
	uid, taskID, ok := h.userAndTaskID(c)
	if !ok {
		return
	}
	result, err := h.AI.Breakdown(c.Request.Context(), uid, taskID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
