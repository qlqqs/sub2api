package handler

import (
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (h *UserHandler) WorkbenchDefaults(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	kind := c.Param("workbench")
	if kind != "studio" && kind != "chat" {
		response.BadRequest(c, "Invalid workbench")
		return
	}
	c.Header("Cache-Control", "no-store")
	if c.Request.Method == "GET" {
		state, err := h.authService.WorkbenchState(c.Request.Context(), subject.UserID, kind)
		if err != nil {
			response.InternalError(c, "Unable to load workbench defaults")
			return
		}
		response.Success(c, state)
		return
	}
	var input struct {
		Preferences json.RawMessage `json:"preferences"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.BadRequest(c, "Invalid preferences")
		return
	}
	prefs, err := h.authService.SaveWorkbench(c.Request.Context(), subject.UserID, kind, input.Preferences, c.Request.Method == "POST")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prefs)
}
