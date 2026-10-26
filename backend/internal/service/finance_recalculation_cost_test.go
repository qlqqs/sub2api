//go:build unit

package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func financeString(value string) *string { return &value }

func financeCostFixture() (FinanceUsageCostInput, FinanceUsageCostSnapshot) {
	in := FinanceUsageCostInput{
		ID: 81, AccountID: 5, GroupID: 10,
		CreatedAt: time.Date(2026, 10, 20, 12, 0, 0, 0, AffiliateTeamLocation),
		Model:     "billing-model", RequestedModel: "public-model", UpstreamModel: "upstream-model",
		CacheStorageCost: financeString("0"), TotalCost: "2.1000000000", AccountStatsCost: financeString("0.0500000000"), AccountRateMultiplier: financeString("1.0000"),
		Tokens: UsageTokens{InputTokens: 1000, OutputTokens: 500},
	}
	snapshot := FinanceUsageCostSnapshot{AccountID: in.AccountID, GroupID: in.GroupID, Model: in.UpstreamModel, Platform: PlatformOpenAI, DefaultMultiplier: "1.0000", LongContextPricingEnabled: true}
	return in, snapshot
}

func financeCostRule(price ChannelModelPricing) *FinanceCostChannelSnapshot {
	price.Models = []string{"upstream-*"}
	return &FinanceCostChannelSnapshot{ID: 3, Rules: []AccountStatsPricingRule{{AccountIDs: []int64{5}, Pricing: []ChannelModelPricing{price}}}}
}

func TestFinanceUsageCostImagePreviewAndImmutability(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.ImageCount = 42
	in.MediaType = "image"
	in.AccountStatsCost = financeString("2.1000000000")
	snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.03)})
	before, err := json.Marshal([]any{in, snapshot})
	require.NoError(t, err)
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationChanged, got.Status)
	require.Equal(t, "channel_cost_rule", got.Source)
	require.Equal(t, "1.2600000000", *got.NewAccountStatsCost)
	require.Equal(t, "2.1", got.OldCost)
	require.Equal(t, "1.26", got.NewCost)
	require.Equal(t, "-0.84", got.Delta)
	after, err := json.Marshal([]any{in, snapshot})
	require.NoError(t, err)
	require.Equal(t, before, after, "no mutation of input, price rules or customer billing")
	*got.OldAccountStatsCost = "99"
	require.Equal(t, "2.1000000000", *in.AccountStatsCost, "audit result must not alias caller data")
}

func TestFinanceUsageCostImageEvidenceIsNotDefaultedToOne(t *testing.T) {
	for _, mode := range []BillingMode{BillingModeImage, BillingModePerRequest} {
		in, snapshot := financeCostFixture()
		in.MediaType = "image"
		snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: mode, PerRequestPrice: testPtrFloat64(0.03)})
		got, err := CalculateFinanceUsageCost(in, snapshot)
		require.NoError(t, err)
		require.Equal(t, "missing_image_count", got.Reason)
		require.Equal(t, FinanceRecalculationSkipped, got.Status)
		require.Nil(t, got.NewAccountStatsCost)
		require.Nil(t, got.NewAccountRateMultiplier)
		require.Empty(t, got.NewCost, "skipped costs are not zero")
	}
}

func TestFinanceUsageCostModelMultiplierPriority(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.AccountStatsCost, in.AccountRateMultiplier = nil, nil
	snapshot.ModelRates = []AccountModelCostRule{{Model: "up*", RateMultiplier: 3}, {Model: "upstream-*", RateMultiplier: 2}, {Model: "upstream-model", RateMultiplier: 0}, {Model: "public-model", RateMultiplier: 4}}
	for _, tc := range []struct{ model, want string }{{"upstream-model", "0"}, {"upstream-other", "4.2"}, {"upper", "6.3"}, {"unmatched", "2.1"}} {
		t.Run(tc.model, func(t *testing.T) {
			in.UpstreamModel, snapshot.Model = tc.model, tc.model
			got, err := CalculateFinanceUsageCost(in, snapshot)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.NewCost)
			require.Nil(t, got.NewAccountStatsCost, "no channel preserves the total_cost fallback")
			require.Nil(t, got.OldAccountRateMultiplier)
		})
	}
	in.UpstreamModel, snapshot.Model = "", "public-model"
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "8.4", got.NewCost)
	in.RequestedModel, snapshot.Model = "", in.Model
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, in.Model, got.Model, "legacy model fallback")
}

func TestFinanceUsageCostCustomRuleOrderAndScope(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.ImageCount = 2
	p := func(pattern string, value float64) ChannelModelPricing {
		return ChannelModelPricing{Models: []string{pattern}, BillingMode: BillingModeImage, PerRequestPrice: &value}
	}
	snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3, Rules: []AccountStatsPricingRule{
		{AccountIDs: []int64{99}, Pricing: []ChannelModelPricing{p("upstream-model", 9)}},
		{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{p("up*", 1), p("upstream-model", 0.03)}},
		{AccountIDs: []int64{5}, Pricing: []ChannelModelPricing{p("upstream-model", 2)}},
	}}
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "0.06", got.NewCost, "first matching rule, exact model before wildcard")
}

func TestFinanceUsageCostPriorityAndExactDecimal(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.TotalCost = "9999999999.1234567890"
	in.AccountStatsCost = nil
	snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3, ApplyPricingToAccountStats: true}
	snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 99}
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "original_customer_base", got.Source)
	require.Equal(t, in.TotalCost, *got.NewAccountStatsCost, "never round-trip persisted total through float64")
	require.Equal(t, "0", got.Delta)
	// A NULL -> explicit equal value still changes the snapshot contract.
	require.Equal(t, FinanceRecalculationChanged, got.Status)
	in.AccountStatsCost = financeString("9999999999.123456789")
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationUnchanged, got.Status)
}

func TestFinanceUsageCostNoChannelDoesNotUseModelFile(t *testing.T) {
	in, snapshot := financeCostFixture()
	snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 99}
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "original_total_cost", got.Source)
	require.Nil(t, got.NewAccountStatsCost)
	require.Equal(t, "2.1", got.NewCost)
}

func TestFinanceUsageCostCustomTokenRuleMatchesOnline(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.ReasoningEffort = "high"
	in.Tokens = UsageTokens{InputTokens: 2000, OutputTokens: 150, CacheCreationTokens: 100, CacheCreation5mTokens: 60, CacheCreation1hTokens: 40, CacheReadTokens: 20}
	p := ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(0.001), OutputPrice: testPtrFloat64(0.002), CacheWritePrice: testPtrFloat64(0.003), CacheWrite1hPrice: testPtrFloat64(0.005), CacheReadPrice: testPtrFloat64(0.0001), ReasoningEffortMultipliers: map[string]float64{"high": 1.5}, Intervals: []PricingInterval{{MinTokens: 1000, InputPrice: testPtrFloat64(0.004), OutputPrice: testPtrFloat64(0.006), CacheWritePrice: testPtrFloat64(0.007), CacheWrite1hPrice: testPtrFloat64(0.009), CacheReadPrice: testPtrFloat64(0.0002)}}}
	snapshot.Channel = financeCostRule(p)
	snapshot.DefaultMultiplier = "0.2200"
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	expected := *calculateStatsCost(&p, in.Tokens, 1) * 1.5
	base := decimal.NewFromFloat(expected).Round(10)
	require.Equal(t, base.StringFixed(10), *got.NewAccountStatsCost)
	require.Equal(t, base.Mul(decimal.RequireFromString("0.22")).String(), got.NewCost)
}

func TestFinanceUsageCostFrozenModelPolicyAndEvidence(t *testing.T) {
	in, snapshot := financeCostFixture()
	snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3}
	snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 0.001, OutputPricePerToken: 0.002, FastMultiplier: testPtrFloat64(3), ReasoningEffortMultipliers: map[string]float64{"high": 2}, LongContextInputThreshold: 100, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 2}
	in.ServiceTier, in.ReasoningEffort = "priority", "high"
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "model_price", got.Source)
	require.Equal(t, "24", got.NewCost)
	snapshot.LongContextPricingEnabled = false
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "12", got.NewCost)
	in.Tokens = UsageTokens{}
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "missing_token_usage", got.Reason)
}

func TestFinanceUsageCostInvalidEvidenceAndConfig(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		change       func(*FinanceUsageCostInput)
	}{
		{"missing identity", "missing_usage_identity", func(in *FinanceUsageCostInput) { in.CreatedAt = time.Time{} }},
		{"corrupt old amount", "invalid_original_cost", func(in *FinanceUsageCostInput) { in.AccountStatsCost = financeString("NaN") }},
		{"negative count", "invalid_usage_counts", func(in *FinanceUsageCostInput) { in.Tokens.InputTokens = -1 }},
		{"count overflow", "invalid_usage_counts", func(in *FinanceUsageCostInput) { in.Tokens.InputTokens = math.MaxInt32 + 1 }},
		{"image total mismatch", "inconsistent_image_tokens", func(in *FinanceUsageCostInput) { in.Tokens.ImageInputTokens = 2000 }},
		{"cache total mismatch", "inconsistent_cache_tokens", func(in *FinanceUsageCostInput) { in.Tokens.CacheCreation1hTokens = 100 }},
		{"video not guessed", "unsupported_video_cost_evidence", func(in *FinanceUsageCostInput) { in.MediaType = "video" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, snapshot := financeCostFixture()
			snapshot.Channel = financeCostRule(ChannelModelPricing{InputPrice: testPtrFloat64(0.001)})
			tc.change(&in)
			got, err := CalculateFinanceUsageCost(in, snapshot)
			require.NoError(t, err)
			require.Equal(t, tc.reason, got.Reason)
			require.Nil(t, got.NewAccountRateMultiplier)
		})
	}
	in, snapshot := financeCostFixture()
	snapshot.AccountID = 999
	_, err := CalculateFinanceUsageCost(in, snapshot)
	require.Error(t, err, "never use another account's snapshot")
	snapshot.AccountID = in.AccountID
	snapshot.ModelRates = []AccountModelCostRule{{Model: "*", RateMultiplier: 1}}
	_, err = CalculateFinanceUsageCost(in, snapshot)
	require.Error(t, err)
	snapshot.ModelRates = nil
	snapshot.ModelPricing = &ModelPricing{InputPricePerToken: math.NaN()}
	_, err = CalculateFinanceUsageCost(in, snapshot)
	require.Error(t, err, "invalid prices cannot be snapshotted")
}

func TestFinanceUsageCostUnusableRuleDoesNotFallThrough(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.ImageCount = 2
	snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0)})
	snapshot.Channel.ApplyPricingToAccountStats = true
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "unusable_cost_rule", got.Reason)
	require.Nil(t, got.NewAccountStatsCost)
}

func TestFinanceUsageCostFingerprints(t *testing.T) {
	in, snapshot := financeCostFixture()
	a, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	b, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, a, b)
	in.ImageCount++
	b, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.NotEqual(t, a.InputFingerprint, b.InputFingerprint)
	require.Equal(t, a.RuleFingerprint, b.RuleFingerprint)
	snapshot.DefaultMultiplier = "0.2000"
	c, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, b.InputFingerprint, c.InputFingerprint)
	require.NotEqual(t, b.RuleFingerprint, c.RuleFingerprint)
}

func TestFinancePaymentFeeNullZeroAndExactAmounts(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		old                   *string
		oldFee, delta, status string
	}{
		{"null falls back but materializes", nil, "3.2", "0", FinanceRecalculationChanged},
		{"zero is not null", financeString("0"), "0", "3.2", FinanceRecalculationChanged},
		{"same numeric rate", financeString("0.032"), "3.2", "0", FinanceRecalculationUnchanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateFinancePaymentFee(FinancePaymentFeeInput{OrderID: 1, ProviderID: 2, PayAmount: "100", OriginalFeeRate: tc.old, CurrentFeeRate: "0.0320"})
			require.NoError(t, err)
			require.Equal(t, tc.old, got.OriginalFeeRate)
			require.Equal(t, tc.oldFee, got.OldFee)
			require.Equal(t, tc.delta, got.Delta)
			require.Equal(t, tc.status, got.Status)
			require.Equal(t, "0.0320", got.NewFeeRate)
		})
	}
	in := FinancePaymentFeeInput{OrderID: 1, ProviderID: 2, PayAmount: "123456789.12345678", OriginalFeeRate: financeString("0.01"), CurrentFeeRate: "0.032"}
	got, err := CalculateFinancePaymentFee(in)
	require.NoError(t, err)
	require.Equal(t, decimal.RequireFromString(in.PayAmount).Mul(decimal.RequireFromString("0.022")).String(), got.Delta)
	for _, rate := range []string{"", "3.2", "0.2001", "-0.01", "NaN", "1e-2"} {
		in.CurrentFeeRate = rate
		_, err = CalculateFinancePaymentFee(in)
		require.Error(t, err, rate)
	}
}

func TestFinanceUsageCostModelFileMatchesOnlineAtHistoricalTime(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(2026, 10, 20, 9, 0, 0, 0, AffiliateTeamLocation),
		time.Date(2026, 10, 20, 13, 0, 0, 0, AffiliateTeamLocation),
	} {
		in, snapshot := financeCostFixture()
		in.UpstreamModel, snapshot.Model = "deepseek-v4-flash", "deepseek-v4-flash"
		in.CreatedAt = at
		in.PricingAt = &at
		snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3}
		snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 0.001, OutputPricePerToken: 0.002}
		bs := &BillingService{fallbackPrices: map[string]*ModelPricing{snapshot.Model: snapshot.ModelPricing}}
		before, err := json.Marshal(snapshot)
		require.NoError(t, err)
		online := tryModelFilePricing(bs, snapshot.Model, in.Tokens, in.ServiceTier, at, snapshot.LongContextPricingEnabled)
		require.NotNil(t, online)
		got, err := CalculateFinanceUsageCost(in, snapshot)
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromFloat(*online).Round(10).String(), got.NewCost)
		after, err := json.Marshal(snapshot)
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
}

func TestFinanceUsageCostStorageRoundingBeforeMultiplier(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.Tokens = UsageTokens{InputTokens: 1}
	snapshot.DefaultMultiplier = "0.1234"
	snapshot.Channel = financeCostRule(ChannelModelPricing{InputPrice: testPtrFloat64(0.12345678905)})
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "0.1234567891", *got.NewAccountStatsCost)
	require.Equal(t, decimal.RequireFromString("0.1234567891").Mul(decimal.RequireFromString("0.1234")).String(), got.NewCost)
	in.AccountStatsCost, in.AccountRateMultiplier = got.NewAccountStatsCost, got.NewAccountRateMultiplier
	second, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationUnchanged, second.Status)
	require.Equal(t, "0", second.Delta)
}

func TestFinanceUsageCostInvalidRuleDoesNotCancelNegativePrices(t *testing.T) {
	in, snapshot := financeCostFixture()
	// The sum would be positive, but accepting it would hide invalid config.
	snapshot.Channel = financeCostRule(ChannelModelPricing{InputPrice: testPtrFloat64(-0.001), OutputPrice: testPtrFloat64(1)})
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "invalid_cost_rule", got.Reason)
	require.Nil(t, got.NewAccountStatsCost)
}

func TestFinanceUsageCostPreservesStorageSurchargeOrSkips(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.TotalCost, in.AccountStatsCost = "3", financeString("3")
	in.Tokens = UsageTokens{InputTokens: 1000}
	snapshot.Channel = financeCostRule(ChannelModelPricing{InputPrice: testPtrFloat64(0.001)})
	in.CacheStorageCost = financeString("2")
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "3", got.NewCost)
	require.Equal(t, FinanceRecalculationUnchanged, got.Status)
	in.CacheStorageCost = nil
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "unknown_cache_storage_cost", got.Reason)
	require.Nil(t, got.NewAccountStatsCost)
	// Paths that preserve the full historical total do not need to guess its components.
	snapshot.Channel.ApplyPricingToAccountStats = true
	snapshot.Channel.Rules = nil
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "3", got.NewCost)
}

func TestFinanceUsageCostRequiresRealPricingTimeForTimeSensitiveModels(t *testing.T) {
	in, snapshot := financeCostFixture()
	in.UpstreamModel, snapshot.Model = "deepseek-v4-flash", "deepseek-v4-flash"
	snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3}
	snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 0.001, OutputPricePerToken: 0.002}
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "missing_pricing_time", got.Reason)
	pricingAt := time.Date(2026, 10, 20, 8, 59, 59, 0, AffiliateTeamLocation)
	in.PricingAt = &pricingAt
	in.CreatedAt = pricingAt.Add(2 * time.Minute)
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationChanged, got.Status)
	// Moving insertion time across a peak boundary must not change the price.
	in.CreatedAt = pricingAt
	other, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, got.NewCost, other.NewCost)
}

func TestFinanceUsageCostRejectsPartialCacheDurationsWhenNeeded(t *testing.T) {
	for _, part := range []int{0, 40} {
		in, snapshot := financeCostFixture()
		in.Tokens.CacheCreationTokens, in.Tokens.CacheCreation5mTokens = 100, part
		snapshot.Channel = financeCostRule(ChannelModelPricing{InputPrice: testPtrFloat64(0.001), CacheWritePrice: testPtrFloat64(0.002), CacheWrite1hPrice: testPtrFloat64(0.004)})
		got, err := CalculateFinanceUsageCost(in, snapshot)
		require.NoError(t, err)
		require.Equal(t, "incomplete_cache_duration_usage", got.Reason)
		snapshot.Channel.Rules = nil
		snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 0.001, SupportsCacheBreakdown: true, CacheCreation5mPrice: 0.002, CacheCreation1hPrice: 0.004}
		got, err = CalculateFinanceUsageCost(in, snapshot)
		require.NoError(t, err)
		require.Equal(t, "incomplete_cache_duration_usage", got.Reason)
		// Aggregate-only pricing does not need a duration split.
		snapshot.ModelPricing.SupportsCacheBreakdown = false
		got, err = CalculateFinanceUsageCost(in, snapshot)
		require.NoError(t, err)
		require.Equal(t, FinanceRecalculationChanged, got.Status)
	}
}

func TestFinanceUsageCostRetainedBaselineDoesNotInventMissingEvidence(t *testing.T) {
	in, snapshot := financeCostFixture()
	frontier := in.CreatedAt.Add(48 * time.Hour)
	snapshot.CoverageFrom = &frontier
	got, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "unproven_aggregate_coverage", got.Reason)
	snapshot.AcceptRetainedBaseline = true
	snapshot.DefaultMultiplier = "2.0000"
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationChanged, got.Status)
	require.Equal(t, "4.2", got.NewCost)
	// Acceptance replaces the historical-coverage gate, not pricing evidence.
	snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(.03)})
	in.MediaType = "image"
	in.ImageCount = 0
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "missing_image_count", got.Reason)
	require.Equal(t, FinanceRecalculationSkipped, got.Status)
	snapshot.MissingReason = "missing_current_account"
	got, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, "missing_current_account", got.Reason)
}
