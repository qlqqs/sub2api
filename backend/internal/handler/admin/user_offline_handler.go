package admin

// CUSTOM(fork): Administrative receipt backfill never credits balance or rewards.
import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (h *UserHandler) BackfillOfflineRecharge(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	redeemID, err := strconv.ParseInt(c.Param("redeem_id"), 10, 64)
	if err != nil || redeemID <= 0 {
		response.BadRequest(c, "Invalid history ID")
		return
	}
	var body struct {
		PaidAmount     float64 `json:"paid_amount"`
		OfflineChannel string  `json:"offline_channel"`
		TradeNo        string  `json:"trade_no"`
		Notes          string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	out, err := h.paymentService.AdminBackfillOfflineRecharge(c.Request.Context(), redeemID, service.AdminOfflineRechargeInput{UserID: userID, AdminID: subject.UserID, PaidAmount: body.PaidAmount, OfflineChannel: body.OfflineChannel, TradeNo: body.TradeNo, Notes: body.Notes})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
