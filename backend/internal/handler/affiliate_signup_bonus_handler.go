package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// PreviewAffiliateSignupBonus is an anonymous, read-only registration preview.
func (h *UserHandler) PreviewAffiliateSignupBonus(c *gin.Context) {
	var req struct {
		AffCode string `json:"aff_code" binding:"max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid invitation code")
		return
	}
	amount, err := h.affiliateService.PreviewInviteeSignupBonus(c.Request.Context(), req.AffCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, struct {
		Amount float64 `json:"amount"`
	}{Amount: amount})
}
