// Package http wires the Gin router: middleware, routes, and handler bindings.
package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/auth"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/http/handler"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/http/middleware"
)

// Router builds the fully-configured gin.Engine.
func Router(h *handler.Handler, tokens *auth.TokenManager, log *slog.Logger, production bool) *gin.Engine {
	if production {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(middleware.Recovery(log))

	// System
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz", h.Readyz)

	// Auth (public)
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)

	// Authenticated
	authed := r.Group("/")
	authed.Use(middleware.Auth(tokens))
	{
		authed.GET("/me", h.Me)

		authed.POST("/tasks", h.CreateTask)
		authed.GET("/tasks", h.ListTasks)
		authed.GET("/tasks/:id", h.GetTask)
		authed.PATCH("/tasks/:id", h.UpdateTask)
		authed.DELETE("/tasks/:id", h.DeleteTask)

		authed.POST("/plans/generate", h.GeneratePlan)
		authed.GET("/plans/jobs/:id", h.GetPlanJob)
		authed.GET("/plans", h.GetPlanByDate)

		// Interactive (synchronous) AI endpoints.
		authed.POST("/ai/prioritize", h.Prioritize)
		authed.POST("/ai/breakdown/:id", h.BreakdownTask)
		authed.POST("/ai/weekly-report", h.WeeklyReport)

		authed.GET("/stats", h.GetStats)

		authed.GET("/habits", h.ListHabits)
		authed.POST("/habits", h.CreateHabit)
		authed.DELETE("/habits/:id", h.DeleteHabit)
		authed.POST("/habits/:id/checkin", h.CheckHabit)
		authed.DELETE("/habits/:id/checkin", h.UncheckHabit)
	}

	return r
}
