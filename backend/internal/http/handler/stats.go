package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

// GetStats handles GET /stats — the user's productivity snapshot.
func (h *Handler) GetStats(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	stats, err := h.Stats.Get(c.Request.Context(), uid)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}
