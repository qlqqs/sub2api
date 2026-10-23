package service

// CUSTOM(fork): 分组模型倍率在各计费 / 预扣 / 利润门路径上的接入辅助函数。
// 上游文件里只保留一两行调用，逻辑集中在这里。

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// responseModelBillingActualCostAdoptable 是 responseModelBillingAdoptable 的实扣金额条件：
// 按响应模型重算出的实扣金额不得高于基线（不得更贵），也不得把基线 > 0 的实扣归零。
// epsilon 按基线的「实扣 / 原始」比例放大，使两侧倍率相同时本条件被 TotalCost 条件蕴含。
func responseModelBillingActualCostAdoptable(baseline, response *CostBreakdown) bool {
	if baseline == nil || response == nil {
		return false
	}
	epsilon := responseModelBillingCostEpsilon
	if baseline.TotalCost > 0 && baseline.ActualCost > baseline.TotalCost {
		epsilon *= baseline.ActualCost / baseline.TotalCost
	}
	if response.ActualCost > baseline.ActualCost+epsilon {
		return false
	}
	if response.ActualCost <= 0 && baseline.ActualCost > 0 {
		return false
	}
	return true
}

// openAIUsageMultipliers 解析 OpenAI 兼容网关一次计费的倍率：
//   - base：基础倍率（分组模型倍率 > 用户专属 > 分组默认 > 系统默认，不含高峰因子），
//     也是网页搜索 / 语音 / 搜索附加费按次口径的倍率；
//   - multiplier / imageMultiplier：token 倍率叠加高峰因子（图片按次倍率不受高峰影响）；
//   - videoMultiplier：视频独立倍率开启时取独立倍率，否则取基础倍率。
//
// 高峰因子按请求级 PricingAt 现算（与利润门 D 同源同刻），不并入专属倍率解析，以免污染
// user:group 倍率缓存。model 为计费模型（候选列表的首个模型）。
func (s *OpenAIGatewayService) openAIUsageMultipliers(ctx context.Context, apiKey *APIKey, user *User, model string, pricingAt time.Time) (base, multiplier, imageMultiplier, videoMultiplier float64) {
	base = 1.0
	if s.cfg != nil {
		base = s.cfg.Default.RateMultiplier
	}
	if apiKey.GroupID != nil && apiKey.Group != nil {
		groupID := *apiKey.GroupID
		base = resolveBaseRate(ctx, s.groupModelRates, groupID, model, func() float64 {
			var userID int64
			if user != nil {
				userID = user.ID
			}
			return s.ResolveUserGroupRateMultiplier(ctx, userID, groupID, apiKey.Group.RateMultiplier)
		})
	}
	multiplier, imageMultiplier = computePeakAwareMultipliers(apiKey, base, pricingAt)
	videoMultiplier = resolveVideoRateMultiplier(apiKey, base)
	return base, multiplier, imageMultiplier, videoMultiplier
}

// inflightModelRates 返回预扣估算某个候选计费模型的 token / 图片倍率：命中分组模型倍率时
// 按模型倍率叠加高峰因子，否则沿用分组级倍率（baseText / baseImage，即原逻辑）。
func (d inflightEstimateDeps) inflightModelRates(ctx context.Context, apiKey *APIKey, model string, baseText, baseImage float64) (float64, float64) {
	if apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil {
		return baseText, baseImage
	}
	if rate, ok := d.groupModelRates.Match(ctx, *apiKey.GroupID, model); ok {
		return computePeakAwareMultipliers(apiKey, rate, timezone.Now())
	}
	return baseText, baseImage
}

// inflightGroupHasModelRates 判断请求分组是否配置了模型倍率（规则倍率恒 > 0）。
func (d inflightEstimateDeps) inflightGroupHasModelRates(ctx context.Context, apiKey *APIKey) bool {
	if apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil {
		return false
	}
	return d.groupModelRates.HasRules(ctx, *apiKey.GroupID)
}

// inflightAnyCandidateHasModelRate 判断候选计费模型中是否有命中分组模型倍率的。
// 免费分组（分组级倍率为 0）只有在候选模型命中 > 0 的模型倍率时才需要预扣。
func (d inflightEstimateDeps) inflightAnyCandidateHasModelRate(ctx context.Context, apiKey *APIKey, groups ...[]string) bool {
	if apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil {
		return false
	}
	for _, models := range groups {
		for _, m := range models {
			if _, ok := d.groupModelRates.Match(ctx, *apiKey.GroupID, m); ok {
				return true
			}
		}
	}
	return false
}

// profitGateModel 取利润门安装点传入的计费模型（可选参数），归一化后作为门的复用键之一。
func profitGateModel(models []string) string {
	for _, m := range models {
		if normalized := normalizeGroupModelRateName(m); normalized != "" {
			return normalized
		}
	}
	return ""
}
