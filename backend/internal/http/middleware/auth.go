package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/auth"
)

// ContextUserIDKey mirrors handler.ContextUserIDKey; kept here to avoid an
// import cycle. Both must stay in sync.
const ContextUserIDKey = "userID"

// Auth validates the Bearer token and stores the user id in the gin context.
// Requests without a valid token are rejected with 401.
func Auth(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := bearerToken(header)
		if !ok {
			abortUnauthorized(c)
			return
		}

		userID, err := tokens.Parse(token)
		if err != nil {
			abortUnauthorized(c)
			return
		}
		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(header[len(prefix):]), true
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"code": "unauthorized", "message": "missing or invalid token"},
	})
}
