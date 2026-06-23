// Package handler implements the Gin HTTP handlers, mapping requests to the
// service layer and domain errors to a consistent JSON error envelope.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/logger"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/service"
)

// ContextUserIDKey is the gin context key holding the authenticated user id.
const ContextUserIDKey = "userID"

// Handler bundles services and infra needed by the HTTP handlers.
type Handler struct {
	Auth   *service.AuthService
	Tasks  *service.TaskService
	Plans  *service.PlanService
	AI     *service.AIService
	Stats  *service.StatsService
	Habits *service.HabitService
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Log    *slog.Logger
}

// errorBody is the single error envelope used by every non-2xx response.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// respondError maps an error to the appropriate status + envelope. Unknown
// errors are logged and returned as a generic 500 (no internal leakage).
func (h *Handler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrConflict):
		writeError(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(c, http.StatusUnauthorized, "unauthorized", "invalid credentials", nil)
	case errors.Is(err, domain.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", err.Error(), nil)
	case errors.Is(err, domain.ErrValidation):
		writeError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
	case errors.Is(err, domain.ErrUnavailable):
		writeError(c, http.StatusServiceUnavailable, "unavailable", err.Error(), nil)
	default:
		logger.FromContext(c.Request.Context(), h.Log).Error("unhandled error", "err", err)
		writeError(c, http.StatusInternalServerError, "internal", "internal server error", nil)
	}
}

// respondValidation returns a 400 with optional field details.
func (h *Handler) respondValidation(c *gin.Context, msg string, details any) {
	writeError(c, http.StatusBadRequest, "validation_error", msg, details)
}

func writeError(c *gin.Context, status int, code, msg string, details any) {
	c.JSON(status, errorBody{Error: errorDetail{Code: code, Message: msg, Details: details}})
}

// userID extracts the authenticated user id placed by the auth middleware.
func userID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextUserIDKey)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}
