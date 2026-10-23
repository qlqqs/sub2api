//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

// 计费路径接入分组模型倍率的回归（design §3 / §8）。

const (
	gmrModelX = anthropicCheapFixtureModel  // claude-sonnet-4：配置了单独倍率
	gmrModelY = anthropicPriceyFixtureModel // claude-opus-4.8：没有单独倍率
)

func newGatewayGroupModelRateTestService(t *testing.T, rules map[int64][]GroupModelRateRule, userRate *float64) (*GatewayService, *openAIRecordUsageLogRepoStub, *userGroupRateRepoHotpathStub) {
	t.Helper()
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	userRateRepo := &userGroupRateRepoHotpathStub{rate: userRate}
	svc.userGroupRateRepo = userRateRepo
	svc.userGroupRateResolver = newUserGroupRateResolver(userRateRepo, nil, time.Minute, nil, "service.gateway.test")
	rates, _ := newTestGroupModelRateService(rules)
	svc.SetGroupModelRateService(rates)
	return svc, usageRepo, userRateRepo
}

func gatewayGroupModelRateUsage(t *testing.T, svc *GatewayService, usageRepo *openAIRecordUsageLogRepoStub, group *Group, model string) *UsageLog {
	t.Helper()
	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "gmr-" + model + "-" + time.Now().Format(time.RFC3339Nano),
			Usage:     ClaudeUsage{InputTokens: 1000, OutputTokens: 500},
			Model:     model,
			Duration:  time.Second,
		},
		APIKey:  &APIKey{ID: 501, GroupID: &group.ID, Group: group},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	log := *usageRepo.lastLog
	require.Greater(t, log.TotalCost, 0.0)
	return &log
}

// AC2：模型倍率优先于用户专属倍率；其他模型仍按专属倍率。
func TestGatewayRecordUsage_GroupModelRateOverridesUserRate(t *testing.T) {
	group := &Group{ID: 71, Platform: PlatformAnthropic, RateMultiplier: 1.0}
	userRate := 0.5
	svc, usageRepo, userRateRepo := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
		group.ID: {{Model: gmrModelX, RateMultiplier: 2.0}},
	}, &userRate)

	x := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelX)
	require.InDelta(t, 2.0, x.RateMultiplier, 1e-12)
	require.InDelta(t, x.TotalCost*2.0, x.ActualCost, 1e-12)
	require.Zero(t, userRateRepo.calls.Load(), "命中模型倍率时不得查询用户专属倍率")

	y := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelY)
	require.InDelta(t, 0.5, y.RateMultiplier, 1e-12)
	require.InDelta(t, y.TotalCost*0.5, y.ActualCost, 1e-12)

	x2 := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelX)
	require.InDelta(t, 2.0, x2.RateMultiplier, 1e-12)
}

// AC3：同一用户先请求有单独倍率的模型、再请求没有的模型，后者不被前者的倍率污染（user:group 缓存）。
func TestGatewayRecordUsage_GroupModelRateDoesNotPolluteUserGroupCache(t *testing.T) {
	group := &Group{ID: 72, Platform: PlatformAnthropic, RateMultiplier: 1.0}
	svc, usageRepo, _ := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
		group.ID: {{Model: gmrModelX, RateMultiplier: 2.0}},
	}, nil)

	x := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelX)
	require.InDelta(t, 2.0, x.RateMultiplier, 1e-12)
	y := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelY)
	require.InDelta(t, 1.0, y.RateMultiplier, 1e-12, "无专属倍率的模型回到分组倍率，不得沿用前一模型的倍率")
	x2 := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelX)
	require.InDelta(t, 2.0, x2.RateMultiplier, 1e-12)
	y2 := gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelY)
	require.InDelta(t, 1.0, y2.RateMultiplier, 1e-12)
}

// 没有任何规则时与引入前完全相同（用户专属 ?? 分组倍率）。
func TestGatewayRecordUsage_NoGroupModelRatesKeepsExistingBehavior(t *testing.T) {
	group := &Group{ID: 73, Platform: PlatformAnthropic, RateMultiplier: 1.3}
	userRate := 0.7
	svc, usageRepo, _ := newGatewayGroupModelRateTestService(t, nil, &userRate)
	require.InDelta(t, 0.7, gatewayGroupModelRateUsage(t, svc, usageRepo, group, gmrModelX).RateMultiplier, 1e-12)

	svcNoInjection, usageRepo2, _ := newGatewayGroupModelRateTestService(t, nil, nil)
	svcNoInjection.SetGroupModelRateService(nil)
	require.InDelta(t, 1.3, gatewayGroupModelRateUsage(t, svcNoInjection, usageRepo2, group, gmrModelX).RateMultiplier, 1e-12)
}

// 高峰倍率照常叠加在模型倍率上；搜索附加费与同一请求用同一个倍率。
func TestGatewayRecordUsage_GroupModelRateStacksPeakAndSurcharges(t *testing.T) {
	group := &Group{
		ID: 74, Platform: PlatformAnthropic, RateMultiplier: 1.0, SubscriptionType: SubscriptionTypeSubscription,
		PeakRateEnabled: true, PeakStart: "08:00", PeakEnd: "09:00", PeakRateMultiplier: 1.5,
	}
	svc, usageRepo, _ := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
		group.ID: {{Model: gmrModelX, RateMultiplier: 2.0}},
	}, nil)
	pricingAt := time.Date(2026, time.January, 15, 8, 30, 0, 0, timezone.Location())

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:   "gmr-peak-search",
			Usage:       ClaudeUsage{InputTokens: 1000, OutputTokens: 500},
			Model:       gmrModelX,
			SearchCount: 3,
			Duration:    time.Second,
		},
		APIKey:    &APIKey{ID: 501, GroupID: &group.ID, Group: group},
		User:      &User{ID: 601},
		Account:   &Account{ID: 701},
		PricingAt: pricingAt,
	})
	require.NoError(t, err)
	log := usageRepo.lastLog
	require.NotNil(t, log)
	require.InDelta(t, 3.0, log.RateMultiplier, 1e-12, "模型倍率 2.0 × 高峰 1.5")
	require.InDelta(t, log.TotalCost*3.0, log.ActualCost, 1e-12, "token 费用与搜索附加费同一倍率")
	searchOnly := svc.billingService.CalculateSearchCost(3, nil, 3.0)
	require.NotNil(t, searchOnly)
	require.Greater(t, searchOnly.TotalCost, 0.0)
}

// AC4：生图独立倍率开启时以独立倍率为准；关闭时生图模型同样适用模型单独倍率。
func TestGatewayRecordUsage_GroupModelRateRespectsImageIndependentRate(t *testing.T) {
	imagePrice := 0.19
	for _, tc := range []struct {
		name        string
		independent bool
		wantRate    float64
	}{
		{name: "independent_on", independent: true, wantRate: 0.7},
		{name: "independent_off", independent: false, wantRate: 3.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &Group{
				ID: 75, Platform: PlatformGemini, RateMultiplier: 1.0, ImagePrice2K: &imagePrice,
				ImageRateIndependent: tc.independent, ImageRateMultiplier: 0.7,
			}
			svc, usageRepo, _ := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
				group.ID: {{Model: "gemini-image", RateMultiplier: 3.0}},
			}, nil)
			err := svc.RecordUsage(context.Background(), &RecordUsageInput{
				Result: &ForwardResult{
					RequestID:      "gmr-image-" + tc.name,
					Model:          "gemini-image",
					ImageCount:     1,
					ImageInputSize: "auto",
					Duration:       time.Second,
				},
				APIKey:  &APIKey{ID: 801, GroupID: &group.ID, Group: group},
				User:    &User{ID: 601},
				Account: &Account{ID: 701},
			})
			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.InDelta(t, tc.wantRate, usageRepo.lastLog.RateMultiplier, 1e-12)
			require.InDelta(t, 0.19, usageRepo.lastLog.TotalCost, 1e-12)
			require.InDelta(t, 0.19*tc.wantRate, usageRepo.lastLog.ActualCost, 1e-12)
		})
	}
}

// QuoteUsageCost（Gemini 缓存固定费报价）与 RecordUsage 同一倍率。
func TestGatewayQuoteUsageCost_UsesGroupModelRate(t *testing.T) {
	group := &Group{ID: 76, Platform: PlatformAnthropic, RateMultiplier: 1.0}
	userRate := 0.5
	svc, _, _ := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
		group.ID: {{Model: gmrModelX, RateMultiplier: 2.0}},
	}, &userRate)
	input := func(model string) *RecordUsageInput {
		return &RecordUsageInput{
			Result: &ForwardResult{Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 500}, Model: model},
			APIKey: &APIKey{ID: 501, GroupID: &group.ID, Group: group},
			User:   &User{ID: 601},
		}
	}
	base, err := svc.billingService.CalculateCost(gmrModelX, UsageTokens{InputTokens: 1000, OutputTokens: 500}, 1.0)
	require.NoError(t, err)
	require.InDelta(t, base.TotalCost*2.0, svc.QuoteUsageCost(context.Background(), input(gmrModelX)), 1e-12)
	baseY, err := svc.billingService.CalculateCost(gmrModelY, UsageTokens{InputTokens: 1000, OutputTokens: 500}, 1.0)
	require.NoError(t, err)
	require.InDelta(t, baseY.TotalCost*0.5, svc.QuoteUsageCost(context.Background(), input(gmrModelY)), 1e-12)
}

// --- 按响应模型重新计费：倍率按响应模型重新解析，「不比原来贵」比较实扣金额 ---

func gatewayResponseModelUsage(t *testing.T, svc *GatewayService, usageRepo *openAIRecordUsageLogRepoStub, group *Group, baseline, response string) *UsageLog {
	t.Helper()
	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:             "gmr-response-" + baseline + "-" + response,
			Usage:                 ClaudeUsage{InputTokens: 100, OutputTokens: 50},
			Model:                 baseline,
			UpstreamResponseModel: response,
			Duration:              time.Second,
		},
		APIKey:  &APIKey{ID: 501, GroupID: &group.ID, Group: group},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID:          9,
			OriginalModel:      baseline,
			ChannelMappedModel: baseline,
			BillingModelSource: BillingModelSourceResponse,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	log := *usageRepo.lastLog
	return &log
}

func TestGatewayRecordUsage_ResponseModelRebillingUsesResponseModelRate(t *testing.T) {
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50}
	group := &Group{ID: 77, Platform: PlatformAnthropic, RateMultiplier: 1.0}

	t.Run("cheaper_total_but_pricier_actual_rejected", func(t *testing.T) {
		probe, _, _ := newGatewayGroupModelRateTestService(t, nil, nil)
		cheaper, pricier, cheaperCost, pricierCost := orderedResponseBillingModels(t, probe.billingService, tokens, anthropicCheapFixtureModel, anthropicPriceyFixtureModel)
		// 响应模型原价更低，但单独倍率高到让实扣更贵：必须沿用基线。
		highRate := pricierCost.TotalCost/cheaperCost.TotalCost*2 + 1
		svc, usageRepo, _ := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
			group.ID: {{Model: cheaper, RateMultiplier: highRate}},
		}, nil)
		log := gatewayResponseModelUsage(t, svc, usageRepo, group, pricier, cheaper)
		require.InDelta(t, 1.0, log.RateMultiplier, 1e-12)
		require.InDelta(t, log.TotalCost, log.ActualCost, 1e-12)
		require.InDelta(t, pricierCost.TotalCost, log.TotalCost, 1e-12, "沿用基线（pricier）成本")
	})

	t.Run("cheaper_actual_adopted_with_response_rate", func(t *testing.T) {
		probe, _, _ := newGatewayGroupModelRateTestService(t, nil, nil)
		cheaper, pricier, cheaperCost, _ := orderedResponseBillingModels(t, probe.billingService, tokens, anthropicCheapFixtureModel, anthropicPriceyFixtureModel)
		svc, usageRepo, _ := newGatewayGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
			group.ID: {{Model: cheaper, RateMultiplier: 0.5}},
		}, nil)
		log := gatewayResponseModelUsage(t, svc, usageRepo, group, pricier, cheaper)
		require.InDelta(t, 0.5, log.RateMultiplier, 1e-12, "usage_logs 记录实际采用（响应模型）的倍率")
		require.InDelta(t, cheaperCost.TotalCost, log.TotalCost, 1e-12)
		require.InDelta(t, log.TotalCost*0.5, log.ActualCost, 1e-12)
	})
}

func TestResponseModelBillingAdoptable_ComparesActualCost(t *testing.T) {
	baseline := &CostBreakdown{TotalCost: 1.0, ActualCost: 1.0}
	// 原价更低但实扣更高（响应模型倍率更高）：拒绝。
	require.False(t, responseModelBillingAdoptable(baseline, &CostBreakdown{TotalCost: 0.5, ActualCost: 1.5}, false, false))
	// 原价与实扣都更低：采纳。
	require.True(t, responseModelBillingAdoptable(baseline, &CostBreakdown{TotalCost: 0.5, ActualCost: 0.25}, false, false))
	// 实扣被归零：拒绝。
	require.False(t, responseModelBillingAdoptable(baseline, &CostBreakdown{TotalCost: 0.5, ActualCost: 0}, false, false))
	// 原有 TotalCost 条件保留：原价更高即拒绝，即使实扣更低。
	require.False(t, responseModelBillingAdoptable(baseline, &CostBreakdown{TotalCost: 1.5, ActualCost: 0.5}, false, false))
	// 基线倍率 > 1 时，epsilon 按倍率放大，浮点末位噪声不误拒。
	scaled := &CostBreakdown{TotalCost: 1.0, ActualCost: 3.0}
	require.True(t, responseModelBillingAdoptable(scaled, &CostBreakdown{TotalCost: 1.0, ActualCost: 3.0 + 2e-12}, false, false))
}

// --- OpenAI 兼容网关 ---

func newOpenAIGroupModelRateTestService(t *testing.T, rules map[int64][]GroupModelRateRule, userRate *float64) (*OpenAIGatewayService, *openAIRecordUsageLogRepoStub, *openAIUserGroupRateRepoStub) {
	t.Helper()
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	rateRepo := &openAIUserGroupRateRepoStub{rate: userRate}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, rateRepo)
	rates, _ := newTestGroupModelRateService(rules)
	svc.SetGroupModelRateService(rates)
	return svc, usageRepo, rateRepo
}

func openAIGroupModelRateUsage(t *testing.T, svc *OpenAIGatewayService, usageRepo *openAIRecordUsageLogRepoStub, group *Group, model string) *UsageLog {
	t.Helper()
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "gmr-openai-" + model + "-" + time.Now().Format(time.RFC3339Nano),
			Usage:     OpenAIUsage{InputTokens: 1000, OutputTokens: 500},
			Model:     model,
			Duration:  time.Second,
		},
		APIKey:  &APIKey{ID: 1001, GroupID: &group.ID, Group: group},
		User:    &User{ID: 2001},
		Account: &Account{ID: 3001, Type: AccountTypeAPIKey},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	log := *usageRepo.lastLog
	require.Greater(t, log.TotalCost, 0.0)
	return &log
}

func TestOpenAIRecordUsage_GroupModelRateOverridesUserRateWithoutPollution(t *testing.T) {
	group := &Group{ID: 81, Platform: PlatformOpenAI, RateMultiplier: 1.0}
	userRate := 0.5
	svc, usageRepo, rateRepo := newOpenAIGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
		group.ID: {{Model: openAIPriceyFixtureModel, RateMultiplier: 2.0}},
	}, &userRate)

	x := openAIGroupModelRateUsage(t, svc, usageRepo, group, openAIPriceyFixtureModel)
	require.InDelta(t, 2.0, x.RateMultiplier, 1e-12)
	require.InDelta(t, x.TotalCost*2.0, x.ActualCost, 1e-12)
	require.Zero(t, rateRepo.calls, "命中模型倍率时不得查询用户专属倍率")

	y := openAIGroupModelRateUsage(t, svc, usageRepo, group, openAICheapFixtureModel)
	require.InDelta(t, 0.5, y.RateMultiplier, 1e-12)

	// 去掉专属倍率（新服务实例，无缓存）后：其他模型回到分组倍率，单独倍率模型不变。
	svc2, usageRepo2, _ := newOpenAIGroupModelRateTestService(t, map[int64][]GroupModelRateRule{
		group.ID: {{Model: openAIPriceyFixtureModel, RateMultiplier: 2.0}},
	}, nil)
	require.InDelta(t, 2.0, openAIGroupModelRateUsage(t, svc2, usageRepo2, group, openAIPriceyFixtureModel).RateMultiplier, 1e-12)
	require.InDelta(t, 1.0, openAIGroupModelRateUsage(t, svc2, usageRepo2, group, openAICheapFixtureModel).RateMultiplier, 1e-12,
		"同一用户先请求有单独倍率的模型，后者不被污染")
}

// --- 预扣冻结：倍率按候选模型逐个算；免费分组只在无 > 0 模型倍率时跳过 ---

func TestInflightEstimate_GroupModelRatePerCandidate(t *testing.T) {
	cfg := testConfig()
	billing := NewBillingService(cfg, nil)
	group := &Group{ID: 91, Platform: PlatformAnthropic, RateMultiplier: 1.0}
	apiKey := &APIKey{ID: 1, GroupID: &group.ID, Group: group, User: &User{ID: 9}}
	rates, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{group.ID: {{Model: gmrModelX, RateMultiplier: 3.0}}})
	req := InflightEstimateRequest{Model: gmrModelX, BodyBytes: 4000, MaxTokens: 1000}

	plain := inflightEstimateDeps{cfg: cfg, billing: billing}
	base, ok := plain.estimate(context.Background(), apiKey, req)
	require.True(t, ok)
	require.Greater(t, base, 0.0)

	withRates := inflightEstimateDeps{cfg: cfg, billing: billing, groupModelRates: rates}
	rated, ok := withRates.estimate(context.Background(), apiKey, req)
	require.True(t, ok)
	require.InDelta(t, base*3.0, rated, 1e-9)

	other, ok := withRates.estimate(context.Background(), apiKey, InflightEstimateRequest{Model: gmrModelY, BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, ok)
	otherPlain, _ := plain.estimate(context.Background(), apiKey, InflightEstimateRequest{Model: gmrModelY, BodyBytes: 4000, MaxTokens: 1000})
	require.InDelta(t, otherPlain, other, 1e-12, "未命中模型沿用分组倍率")
}

func TestInflightEstimate_FreeGroupSkipsOnlyWithoutPositiveModelRate(t *testing.T) {
	cfg := testConfig()
	billing := NewBillingService(cfg, nil)
	group := &Group{ID: 92, Platform: PlatformAnthropic, RateMultiplier: 0}
	apiKey := &APIKey{ID: 1, GroupID: &group.ID, Group: group, User: &User{ID: 9}}
	req := InflightEstimateRequest{Model: gmrModelX, BodyBytes: 4000, MaxTokens: 1000}

	noRules, _ := newTestGroupModelRateService(nil)
	cost, ok := inflightEstimateDeps{cfg: cfg, billing: billing, groupModelRates: noRules}.estimate(context.Background(), apiKey, req)
	require.True(t, ok)
	require.Zero(t, cost, "免费分组且无模型倍率：直接跳过")

	rates, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{group.ID: {{Model: gmrModelX, RateMultiplier: 2.0}}})
	deps := inflightEstimateDeps{cfg: cfg, billing: billing, groupModelRates: rates}
	cost, ok = deps.estimate(context.Background(), apiKey, req)
	require.True(t, ok)
	require.Greater(t, cost, 0.0, "免费分组但该模型有 > 0 的模型倍率：需要预留")

	cost, ok = deps.estimate(context.Background(), apiKey, InflightEstimateRequest{Model: gmrModelY, BodyBytes: 4000, MaxTokens: 1000})
	require.True(t, ok, "免费分组中未命中模型倍率的模型仍按免费跳过，不得判为无法定价")
	require.Zero(t, cost)
}

// --- 批量生图定价快照 ---

type batchImageGroupLiteRepoStub struct {
	BatchImageGroupPricingRepository
	group *Group
}

func (r batchImageGroupLiteRepoStub) GetByIDLite(context.Context, int64) (*Group, error) {
	return r.group, nil
}

type batchImageUserRateRepoStub struct {
	rate  *float64
	calls int
}

func (r *batchImageUserRateRepoStub) GetByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	r.calls++
	return r.rate, nil
}

func TestBatchImagePricingSnapshot_UsesGroupModelRate(t *testing.T) {
	unit := 0.04
	userRate := 0.5
	for _, tc := range []struct {
		name        string
		model       string
		independent bool
		want        float64
		wantLookups int
	}{
		{name: "model_rate_wins_over_user_rate", model: "gemini-image-x", want: 2.0, wantLookups: 0},
		{name: "unmatched_uses_user_rate", model: "gemini-image-y", want: 0.5, wantLookups: 1},
		{name: "image_independent_wins", model: "gemini-image-x", independent: true, want: 0.8, wantLookups: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &Group{
				ID: 93, Platform: PlatformGemini, RateMultiplier: 1.0, AllowBatchImageGeneration: true,
				ImagePrice1K: &unit, BatchImageDiscountMultiplier: 1, BatchImageHoldMultiplier: 1,
				ImageRateIndependent: tc.independent, ImageRateMultiplier: 0.8,
			}
			userRates := &batchImageUserRateRepoStub{rate: &userRate}
			rates, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{group.ID: {{Model: "gemini-image-x", RateMultiplier: 2.0}}})
			svc := &BatchImagePublicService{GroupRepo: batchImageGroupLiteRepoStub{group: group}, UserGroupRateRepo: userRates}
			svc.SetGroupModelRateService(rates)

			snapshot, err := svc.resolvePricingSnapshot(context.Background(), BatchImageOwner{UserID: 5, GroupID: &group.ID},
				BatchImageSubmitRequest{Model: tc.model, ImageSize: "1K"}, "vertex", nil)
			require.NoError(t, err)
			require.InDelta(t, tc.want, snapshot.GroupRateMultiplier, 1e-12)
			require.Equal(t, tc.wantLookups, userRates.calls)
		})
	}
}

// --- 利润门：D 按计费模型的倍率；门按「同一分组且同一模型」复用 ---

func TestGatewayProfitGate_UsesGroupModelRateAndReusesPerModel(t *testing.T) {
	group := gatewayProfitTestGroup(111, PlatformAnthropic) // 分组倍率 0.5，margin/buffer 0
	svc := &GatewayService{}
	rates, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{group.ID: {{Model: gmrModelX, RateMultiplier: 2.0}}})
	svc.SetGroupModelRateService(rates)
	ctx := gatewayProfitTestContext(group)

	xCtx := svc.withGatewayProfitControlGate(ctx, &group.ID, gmrModelX)
	xGate, _ := xCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	require.NotNil(t, xGate)
	require.InDelta(t, 2.0, xGate.threshold, 1e-12, "D = 模型倍率")

	require.Same(t, xGate, gateFromCtx(svc.withGatewayProfitControlGate(xCtx, &group.ID, gmrModelX)), "同组同模型复用")

	yCtx := svc.withGatewayProfitControlGate(xCtx, &group.ID, gmrModelY)
	yGate := gateFromCtx(yCtx)
	require.NotSame(t, xGate, yGate, "同组不同模型不复用")
	require.InDelta(t, 0.5, yGate.threshold, 1e-12, "未命中模型按分组倍率")
}

func gateFromCtx(ctx context.Context) *openAIProfitControlGate {
	gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	return gate
}

func TestOpenAIProfitGate_UsesGroupModelRateAndPerTurnModel(t *testing.T) {
	groupID := int64(112)
	group := profitControlTestGroup(groupID, 0.5, 0) // 分组倍率 1.0，阈值 = D × 0.5
	svc := &OpenAIGatewayService{}
	rates, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{groupID: {{Model: "gpt-5.5", RateMultiplier: 4.0}}})
	svc.SetGroupModelRateService(rates)
	base := profitControlTestCtx(group)

	ctx, _ := svc.WithOpenAIRequestPricingContext(base, &groupID, "GPT-5.5")
	gate := gateFromCtx(ctx)
	require.NotNil(t, gate)
	require.InDelta(t, 2.0, gate.threshold, 1e-12, "D = 4.0 × (1 − 0.5)")
	require.Same(t, gate, gateFromCtx(svc.withOpenAIProfitControlGate(ctx, &groupID, "gpt-5.5")), "同组同模型（归一化后）复用")

	other := gateFromCtx(svc.withOpenAIProfitControlGate(ctx, &groupID, "gpt-5.4-nano"))
	require.NotSame(t, gate, other)
	require.InDelta(t, 0.5, other.threshold, 1e-12)

	// 不传模型：沿用分组 / 专属倍率（与引入前相同）。
	legacy := gateFromCtx(svc.withOpenAIProfitControlGate(base, &groupID))
	require.InDelta(t, 0.5, legacy.threshold, 1e-12)

	// WS 每轮重装：传入本轮模型时按本轮模型；不传时沿用连接上门的模型。
	turnCtx, _ := svc.WithOpenAITurnPricingContext(ctx, &groupID, "gpt-5.4-nano")
	require.InDelta(t, 0.5, gateFromCtx(turnCtx).threshold, 1e-12)
	keepCtx, _ := svc.WithOpenAITurnPricingContext(ctx, &groupID)
	require.InDelta(t, 2.0, gateFromCtx(keepCtx).threshold, 1e-12)
}

// --- 利润预览按模型分别计算 ---

func TestPreviewProfitAdmission_PerModelRates(t *testing.T) {
	rate := func(v float64) *float64 { return &v }
	group := &Group{ID: 121, Name: "g", Platform: PlatformAnthropic, RateMultiplier: 1.0, ProfitControlEnabled: true}
	accounts := []*Account{
		{ID: 1, Name: "cheap", Platform: PlatformAnthropic, RateMultiplier: rate(0.9)},
		{ID: 2, Name: "pricey", Platform: PlatformAnthropic, RateMultiplier: rate(1.5)},
	}
	reports := PreviewProfitAdmission([]ProfitPreviewGroupInput{{
		Group:         group,
		Accounts:      accounts,
		UserOverrides: map[int64]float64{7: 0.5},
		Models:        []string{"claude-opus-4-5", "claude-sonnet-4"},
		ModelRates:    []GroupModelRateRule{{Model: "claude-opus-*", RateMultiplier: 2.0}},
	}}, time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC))
	require.Len(t, reports, 1)
	report := reports[0]

	require.InDelta(t, 2.0, report.ModelD["claude-opus-4-5"], 1e-12)
	require.InDelta(t, 2.0, report.ThresholdByModel["claude-opus-4-5"], 1e-12)
	_, hasSonnet := report.ModelD["claude-sonnet-4"]
	require.False(t, hasSonnet)

	require.Equal(t, 2, report.RemainingByModel["claude-opus-4-5"], "模型倍率 2.0 下两个账号都准入")
	require.Equal(t, 2, report.RemainingByModelMinD["claude-opus-4-5"], "用户覆盖倍率不作用于单独倍率模型")
	require.Equal(t, 1, report.RemainingByModel["claude-sonnet-4"], "分组倍率 1.0 下只有 cheap 准入")
	require.Equal(t, 0, report.RemainingByModelMinD["claude-sonnet-4"], "最低有效 D 0.5 下全部拒绝")
}

// --- 模型广场：每个模型的公开倍率 = 模型单独倍率 ?? 分组倍率 ---

func TestListPlazaGroups_FillsModelRateMultiplier(t *testing.T) {
	channels := []Channel{plazaPricedChannel(1, "ch", []int64{10}, PlatformAnthropic, "claude-opus-4-5", "claude-sonnet-4")}
	groups := []Group{{ID: 10, Name: "g", Platform: PlatformAnthropic, RateMultiplier: 0.6}}
	svc := newPlazaService(channels, groups, nil)
	rates, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{10: {{Model: "claude-opus-*", RateMultiplier: 1.0}}})
	svc.SetGroupModelRateService(rates)

	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.InDelta(t, 0.6, out[0].RateMultiplier, 1e-12, "分组级倍率不变（排序 / 旧前端兼容）")
	byName := plazaModelsByName(out[0].Models)
	require.True(t, byName["claude-opus-4-5"].HasModelRate)
	require.InDelta(t, 1.0, byName["claude-opus-4-5"].RateMultiplier, 1e-12)
	require.False(t, byName["claude-sonnet-4"].HasModelRate)
	require.InDelta(t, 0.6, byName["claude-sonnet-4"].RateMultiplier, 1e-12)
}
