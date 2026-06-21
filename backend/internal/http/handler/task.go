package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/service"
)

const defaultTaskDuration int32 = 30

type createTaskRequest struct {
	Title           string `json:"title" binding:"required,min=1,max=200"`
	Description     string `json:"description" binding:"max=2000"`
	Priority        string `json:"priority" binding:"omitempty,oneof=low medium high"`
	DurationMinutes int32  `json:"duration_minutes" binding:"omitempty,gt=0"`
}

type updateTaskRequest struct {
	Title           *string `json:"title" binding:"omitempty,min=1,max=200"`
	Description     *string `json:"description" binding:"omitempty,max=2000"`
	Priority        *string `json:"priority" binding:"omitempty,oneof=low medium high"`
	DurationMinutes *int32  `json:"duration_minutes" binding:"omitempty,gt=0"`
	Status          *string `json:"status" binding:"omitempty,oneof=pending completed"`
}

// CreateTask handles POST /tasks.
func (h *Handler) CreateTask(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondValidation(c, "invalid task payload", err.Error())
		return
	}
	duration := req.DurationMinutes
	if duration == 0 {
		duration = defaultTaskDuration
	}

	task, err := h.Tasks.Create(c.Request.Context(), uid, service.CreateTaskInput{
		Title:           req.Title,
		Description:     req.Description,
		Priority:        domain.Priority(req.Priority),
		DurationMinutes: duration,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /tasks with optional ?status= & ?priority= filters.
func (h *Handler) ListTasks(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return
	}

	var filter service.ListTaskFilter
	if s := c.Query("status"); s != "" {
		st := domain.TaskStatus(s)
		if !st.Valid() {
			h.respondValidation(c, "invalid status filter", nil)
			return
		}
		filter.Status = &st
	}
	if p := c.Query("priority"); p != "" {
		pr := domain.Priority(p)
		if !pr.Valid() {
			h.respondValidation(c, "invalid priority filter", nil)
			return
		}
		filter.Priority = &pr
	}

	tasks, err := h.Tasks.List(c.Request.Context(), uid, filter)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// GetTask handles GET /tasks/:id.
func (h *Handler) GetTask(c *gin.Context) {
	uid, taskID, ok := h.userAndTaskID(c)
	if !ok {
		return
	}
	task, err := h.Tasks.Get(c.Request.Context(), uid, taskID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

// UpdateTask handles PATCH /tasks/:id.
func (h *Handler) UpdateTask(c *gin.Context) {
	uid, taskID, ok := h.userAndTaskID(c)
	if !ok {
		return
	}

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondValidation(c, "invalid task payload", err.Error())
		return
	}

	in := service.UpdateTaskInput{
		Title:           req.Title,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
	}
	if req.Priority != nil {
		p := domain.Priority(*req.Priority)
		in.Priority = &p
	}
	if req.Status != nil {
		s := domain.TaskStatus(*req.Status)
		in.Status = &s
	}

	task, err := h.Tasks.Update(c.Request.Context(), uid, taskID, in)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

// DeleteTask handles DELETE /tasks/:id.
func (h *Handler) DeleteTask(c *gin.Context) {
	uid, taskID, ok := h.userAndTaskID(c)
	if !ok {
		return
	}
	if err := h.Tasks.Delete(c.Request.Context(), uid, taskID); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// userAndTaskID resolves the authenticated user and the :id path param,
// writing the appropriate error response and returning ok=false on failure.
func (h *Handler) userAndTaskID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := userID(c)
	if !ok {
		h.respondError(c, domain.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.respondValidation(c, "invalid task id", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return uid, taskID, true
}
