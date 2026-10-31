package handler

// CUSTOM: The client update protocol uses raw JSON, not the panel envelope.
import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ClientReleaseHandler struct{ releases *service.ClientReleaseService }

func NewClientReleaseHandler(releases *service.ClientReleaseService) *ClientReleaseHandler {
	return &ClientReleaseHandler{releases: releases}
}
func (h *ClientReleaseHandler) Latest(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	r, err := h.releases.Latest(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}
func (h *ClientReleaseHandler) Notes(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	limit := 0
	if value, ok := c.GetQuery("limit"); ok {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil || limit <= 0 {
			response.BadRequest(c, "Limit must be a positive integer")
			return
		}
	}
	notes, err := h.releases.Notes(c.Request.Context(), c.Query("after"), c.Query("upto"), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"releases": notes})
}
func (h *ClientReleaseHandler) File(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	path, err := h.releases.FilePath(c.Request.Context(), c.Param("version"), c.Param("name"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.File(path)
}
