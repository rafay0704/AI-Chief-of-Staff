// Package middleware holds Gin middleware: request id, structured logging,
// panic recovery, and JWT authentication.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/logger"
)

const requestIDHeader = "X-Request-ID"

// RequestID ensures every request has an id, stores it in the request context,
// and echoes it back in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		ctx := logger.WithRequestID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// Logger logs each request as a structured slog line with timing and status.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		l := logger.FromContext(c.Request.Context(), log)
		l.Info("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// Recovery converts panics into a 500 JSON error and logs the stack.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.FromContext(c.Request.Context(), log).
					Error("panic recovered", "panic", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{"code": "internal", "message": "internal server error"},
				})
			}
		}()
		c.Next()
	}
}
