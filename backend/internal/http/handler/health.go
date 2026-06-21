package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Healthz is a liveness probe — it does not touch dependencies.
func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz is a readiness probe — it pings Postgres and Redis.
func (h *Handler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := gin.H{}
	ready := true

	if err := h.DB.Ping(ctx); err != nil {
		checks["postgres"] = "error: " + err.Error()
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "error: " + err.Error()
		ready = false
	} else {
		checks["redis"] = "ok"
	}

	status := http.StatusOK
	statusText := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		statusText = "unavailable"
	}
	c.JSON(status, gin.H{"status": statusText, "checks": checks})
}
