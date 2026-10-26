package admin

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Mount only under the existing authenticated/audited admin finance group.
type FinanceChannelCostHandler struct {
	s *service.FinanceChannelCostService
}

func NewFinanceChannelCostHandler(s *service.FinanceChannelCostService) *FinanceChannelCostHandler {
	return &FinanceChannelCostHandler{s: s}
}

type financeChannelCostResponse struct {
	ChannelID                  int64                             `json:"channel_id"`
	Version                    string                            `json:"version"`
	ApplyPricingToAccountStats bool                              `json:"apply_pricing_to_account_stats"`
	AccountStatsPricingRules   []accountStatsPricingRuleResponse `json:"account_stats_pricing_rules"`
}

func financeChannelCostToResponse(config *service.FinanceChannelCostConfig) financeChannelCostResponse {
	ch := channelToResponse(&service.Channel{ApplyPricingToAccountStats: config.ApplyPricingToAccountStats, AccountStatsPricingRules: config.AccountStatsPricingRules})
	// Empty platform means ANY platform for cost rules, not anthropic. The
	// generic channel price response default must not rewrite this scope.
	for i, r := range config.AccountStatsPricingRules {
		for j, p := range r.Pricing {
			ch.AccountStatsPricingRules[i].Pricing[j].Platform = p.Platform
		}
	}
	return financeChannelCostResponse{ChannelID: config.ChannelID, Version: config.Version, ApplyPricingToAccountStats: config.ApplyPricingToAccountStats, AccountStatsPricingRules: ch.AccountStatsPricingRules}
}

func financeChannelCostID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_CHANNEL_ID", "Invalid channel ID"))
		return 0, false
	}
	return id, true
}

func (h *FinanceChannelCostHandler) Get(c *gin.Context) {
	id, ok := financeChannelCostID(c)
	if !ok {
		return
	}
	result, err := h.s.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, financeChannelCostToResponse(result))
}

func (h *FinanceChannelCostHandler) Update(c *gin.Context) {
	id, ok := financeChannelCostID(c)
	if !ok {
		return
	}
	var req struct {
		ExpectedVersion            string                            `json:"expected_version" binding:"required"`
		ApplyPricingToAccountStats *bool                             `json:"apply_pricing_to_account_stats"`
		AccountStatsPricingRules   *[]accountStatsPricingRuleRequest `json:"account_stats_pricing_rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", "Invalid channel cost configuration"))
		return
	}
	input := service.FinanceChannelCostUpdate{ExpectedVersion: req.ExpectedVersion, ApplyPricingToAccountStats: req.ApplyPricingToAccountStats}
	if req.AccountStatsPricingRules != nil {
		rules := make([]service.AccountStatsPricingRule, 0, len(*req.AccountStatsPricingRules))
		for i, r := range *req.AccountStatsPricingRules {
			// Keep every shared DTO field; service rejects unsupported cost fields
			// explicitly. false here would silently discard fast/flex/interval fields.
			rules = append(rules, service.AccountStatsPricingRule{Name: r.Name, GroupIDs: r.GroupIDs, AccountIDs: r.AccountIDs, SortOrder: i, Pricing: pricingRequestToService(r.Pricing, true)})
		}
		input.AccountStatsPricingRules = &rules
	}
	result, err := h.s.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, financeChannelCostToResponse(result))
}
