// CUSTOM: Exposes the read-only upstream account balance query action.
package admin

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (handler *AccountHandler) QueryUpstreamBalance(ctx *gin.Context) {
	accountID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.ErrorFrom(ctx, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account ID"))
		return
	}
	if handler == nil || handler.upstreamBalanceService == nil {
		response.ErrorFrom(ctx, infraerrors.ServiceUnavailable("UPSTREAM_BALANCE_UNAVAILABLE", "upstream balance service is unavailable"))
		return
	}

	result, err := handler.upstreamBalanceService.QueryAccountBalance(ctx.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(ctx, err)
		return
	}
	response.Success(ctx, result)
}
