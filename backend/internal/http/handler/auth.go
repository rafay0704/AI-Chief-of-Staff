package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
)

type registerRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type authResponse struct {
	User  domain.User `json:"user"`
	Token string      `json:"token"`
}

// Register handles POST /auth/register.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondValidation(c, "invalid registration payload", err.Error())
		return
	}

	user, token, err := h.Auth.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, authResponse{User: user, Token: token})
}

// Login handles POST /auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondValidation(c, "invalid login payload", err.Error())
		return
	}

	user, token, err := h.Auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, authResponse{User: user, Token: token})
}

// Me handles GET /me (authenticated).
func (h *Handler) Me(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}
	user, err := h.Auth.GetUser(c.Request.Context(), uid)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}
