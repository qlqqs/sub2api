package admin

// CUSTOM(fork): 分组内单个模型倍率管理接口（design §15.3）。

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupModelRateHandler 分组模型倍率管理端接口。
type GroupModelRateHandler struct {
	rates *service.GroupModelRateService
	cfg   *config.Config
}

// NewGroupModelRateHandler 创建分组模型倍率 handler。
func NewGroupModelRateHandler(rates *service.GroupModelRateService, cfg *config.Config) *GroupModelRateHandler {
	return &GroupModelRateHandler{rates: rates, cfg: cfg}
}

// groupModelRateRuleDTO 是一条模型倍率规则（API 合约：{ model, rate_multiplier }）。
type groupModelRateRuleDTO struct {
	Model          string  `json:"model"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type groupModelRatesResponse struct {
	Rules []groupModelRateRuleDTO `json:"rules"`
}

type replaceGroupModelRatesRequest struct {
	Rules *[]groupModelRateRuleDTO `json:"rules"`
}

func toGroupModelRateRuleDTOs(rules []service.GroupModelRateRule) []groupModelRateRuleDTO {
	out := make([]groupModelRateRuleDTO, 0, len(rules))
	for _, rule := range rules {
		out = append(out, groupModelRateRuleDTO{Model: rule.Model, RateMultiplier: rule.RateMultiplier})
	}
	return out
}

func (h *GroupModelRateHandler) isSimpleMode() bool {
	return h != nil && h.cfg != nil && h.cfg.RunMode == config.RunModeSimple
}

// rejectSimpleMode 简易模式下不提供模型单独倍率配置（与分组专属倍率一致）。
func (h *GroupModelRateHandler) rejectSimpleMode(c *gin.Context) bool {
	if err := service.ValidateSimpleModeGroupOperation(h.cfg, service.AdminGroupOperation("model_rates")); err != nil {
		response.ErrorFrom(c, err)
		return true
	}
	return false
}

// ListAll 返回全部未删除分组的模型倍率（列表页提示用），键为分组 id。
// GET /api/v1/admin/groups/model-rates
func (h *GroupModelRateHandler) ListAll(c *gin.Context) {
	if h.isSimpleMode() {
		// 简易模式不展示该配置：返回空集合而不是报错，避免列表页整体失败。
		response.Success(c, map[string][]groupModelRateRuleDTO{})
		return
	}
	all, err := h.rates.ListAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make(map[string][]groupModelRateRuleDTO, len(all))
	for groupID, rules := range all {
		if len(rules) == 0 {
			continue
		}
		out[strconv.FormatInt(groupID, 10)] = toGroupModelRateRuleDTOs(rules)
	}
	response.Success(c, out)
}

// Get 返回单个分组的模型倍率（按 model 排序）。
// GET /api/v1/admin/groups/:id/model-rates
func (h *GroupModelRateHandler) Get(c *gin.Context) {
	if h.rejectSimpleMode(c) {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	rules, err := h.rates.ListForGroup(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, groupModelRatesResponse{Rules: toGroupModelRateRuleDTOs(rules)})
}

// Replace 整组替换模型倍率（空数组 = 清空），返回归一化后的规则。
// PUT /api/v1/admin/groups/:id/model-rates
func (h *GroupModelRateHandler) Replace(c *gin.Context) {
	if h.rejectSimpleMode(c) {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	var req replaceGroupModelRatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Rules == nil {
		response.BadRequest(c, "rules is required")
		return
	}
	rules := make([]service.GroupModelRateRule, 0, len(*req.Rules))
	for _, rule := range *req.Rules {
		rules = append(rules, service.GroupModelRateRule{Model: rule.Model, RateMultiplier: rule.RateMultiplier})
	}
	saved, err := h.rates.ReplaceForGroup(c.Request.Context(), groupID, rules)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, groupModelRatesResponse{Rules: toGroupModelRateRuleDTOs(saved)})
}
