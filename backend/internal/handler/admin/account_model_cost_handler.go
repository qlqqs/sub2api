package admin

// CUSTOM(fork): 账号内单个模型倍率管理接口（finance design §3）。

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AccountModelCostHandler 账号模型倍率管理端接口。
type AccountModelCostHandler struct {
	rates *service.AccountModelCostService
}

// NewAccountModelCostHandler 创建账号模型倍率 handler。
func NewAccountModelCostHandler(rates *service.AccountModelCostService) *AccountModelCostHandler {
	return &AccountModelCostHandler{rates: rates}
}

// accountModelCostRuleDTO 是一条模型倍率规则（API 合约：{ model, rate_multiplier }）。
type accountModelCostRuleDTO struct {
	Model          string  `json:"model"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type accountModelCostsResponse struct {
	Rules []accountModelCostRuleDTO `json:"rules"`
}

type replaceAccountModelCostsRequest struct {
	Rules *[]struct {
		Model          string   `json:"model"`
		RateMultiplier *float64 `json:"rate_multiplier"`
	} `json:"rules"`
}

func toAccountModelCostRuleDTOs(rules []service.AccountModelCostRule) []accountModelCostRuleDTO {
	out := make([]accountModelCostRuleDTO, 0, len(rules))
	for _, rule := range rules {
		out = append(out, accountModelCostRuleDTO{Model: rule.Model, RateMultiplier: rule.RateMultiplier})
	}
	return out
}

// ListAll 返回全部未删除账号的模型倍率（列表页提示用），键为账号 id。
// GET /api/v1/admin/accounts/model-cost-multipliers
func (h *AccountModelCostHandler) ListAll(c *gin.Context) {
	all, err := h.rates.ListAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make(map[string][]accountModelCostRuleDTO, len(all))
	for accountID, rules := range all {
		if len(rules) == 0 {
			continue
		}
		out[strconv.FormatInt(accountID, 10)] = toAccountModelCostRuleDTOs(rules)
	}
	response.Success(c, out)
}

// Get 返回单个账号的模型倍率（按 model 排序）。
// GET /api/v1/admin/accounts/:id/model-cost-multipliers
func (h *AccountModelCostHandler) Get(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "账号 ID 无效")
		return
	}
	rules, err := h.rates.ListForAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountModelCostsResponse{Rules: toAccountModelCostRuleDTOs(rules)})
}

// Replace 整组替换模型倍率（空数组 = 清空），返回归一化后的规则。
// PUT /api/v1/admin/accounts/:id/model-cost-multipliers
func (h *AccountModelCostHandler) Replace(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "账号 ID 无效")
		return
	}
	var req replaceAccountModelCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_MODEL_COST_INVALID", "模型成本倍率格式无效"))
		return
	}
	if req.Rules == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_MODEL_COST_INVALID", "必须提供 rules 数组"))
		return
	}
	rules := make([]service.AccountModelCostRule, 0, len(*req.Rules))
	for _, rule := range *req.Rules {
		if rule.RateMultiplier == nil {
			response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_MODEL_COST_INVALID", "必须提供倍率，免费成本请显式填 0"))
			return
		}
		rules = append(rules, service.AccountModelCostRule{Model: rule.Model, RateMultiplier: *rule.RateMultiplier})
	}
	saved, err := h.rates.ReplaceForAccount(c.Request.Context(), accountID, rules)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountModelCostsResponse{Rules: toAccountModelCostRuleDTOs(saved)})
}
