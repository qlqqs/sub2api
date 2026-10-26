package service

// CUSTOM(fork): deterministic preview calculation only. This file never updates
// usage logs, balances, quota counters, payment amounts or settlement ledgers.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	FinanceRecalculationChanged   = "changed"
	FinanceRecalculationUnchanged = "unchanged"
	FinanceRecalculationSkipped   = "skipped"
)

// FinanceUsageCostInput contains only persisted calculation facts. Monetary
// strings must be read from NUMERIC columns directly, not through float64.
// Tokens must have the same meaning as UsageTokens in the online cost path
// (notably non-cached image input tokens, not the raw upstream image total).
// Missing historical inputs are not reconstructed from the current model map.
type FinanceUsageCostInput struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	// PricingAt is a reliable persisted request pricing instant, NOT insertion time.
	PricingAt *time.Time `json:"pricing_at"`
	// Nil means unknown; explicit zero means a proven absence of storage surcharge.
	ImageCacheSplitKnown  bool        `json:"image_cache_split_known"`
	CacheStorageCost      *string     `json:"cache_storage_cost"`
	AccountID             int64       `json:"account_id"`
	GroupID               int64       `json:"group_id"`
	Model                 string      `json:"model"`
	RequestedModel        string      `json:"requested_model"`
	UpstreamModel         string      `json:"upstream_model"`
	ServiceTier           string      `json:"service_tier"`
	ReasoningEffort       string      `json:"reasoning_effort"`
	Tokens                UsageTokens `json:"tokens"`
	ImageCount            int         `json:"image_count"`
	MediaType             string      `json:"media_type"`
	BillingMode           BillingMode `json:"billing_mode"`
	TotalCost             string      `json:"total_cost"`
	AccountStatsCost      *string     `json:"account_stats_cost"`
	AccountRateMultiplier *string     `json:"account_rate_multiplier"`
}

// FinanceCostChannelSnapshot deliberately excludes credentials, user-facing
// prices, routing and feature configuration. Rules retain their original order.
type FinanceCostChannelSnapshot struct {
	ID                         int64                     `json:"id"`
	ApplyPricingToAccountStats bool                      `json:"apply_pricing_to_account_stats"`
	Rules                      []AccountStatsPricingRule `json:"rules"`
}

// FinanceUsageCostSnapshot is loaded once by the preview orchestrator. A nil
// Channel means a VERIFIED absent channel, not a lookup failure. Lookup errors
// must abort preview creation rather than silently use stale cached config.
// ModelPricing is the resolved current model-file/fallback price for Model.
// The account's CURRENT long-context policy is explicit, not inferred from the
// historical customer LongContextBillingApplied flag (a different policy).
type FinanceUsageCostSnapshot struct {
	// Set only by the config loader after full-day retained-baseline reconciliation.
	AcceptRetainedBaseline    bool                        `json:"accept_retained_baseline,omitempty"`
	CoverageFrom              *time.Time                  `json:"coverage_from,omitempty"`
	MissingReason             string                      `json:"missing_reason,omitempty"`
	AccountID                 int64                       `json:"account_id"`
	GroupID                   int64                       `json:"group_id"`
	Model                     string                      `json:"model"`
	Platform                  string                      `json:"platform"`
	DefaultMultiplier         string                      `json:"default_multiplier"`
	ModelRates                []AccountModelCostRule      `json:"model_rates"`
	Channel                   *FinanceCostChannelSnapshot `json:"channel"`
	ModelPricing              *ModelPricing               `json:"model_pricing"`
	LongContextPricingEnabled bool                        `json:"long_context_pricing_enabled"`
}

// FinanceUsageCostResult preserves NULL snapshots and exact aggregate operands.
// Amounts are costs in the EXISTING account-cost unit; there is no FX conversion
// here. Delta is new minus old. A skipped row has no proposed new values.
type FinanceUsageCostResult struct {
	Status                   string  `json:"status"`
	Reason                   string  `json:"reason,omitempty"`
	Source                   string  `json:"source,omitempty"`
	Model                    string  `json:"model"`
	InputFingerprint         string  `json:"input_fingerprint"`
	RuleFingerprint          string  `json:"rule_fingerprint"`
	OldAccountStatsCost      *string `json:"old_account_stats_cost"`
	OldAccountRateMultiplier *string `json:"old_account_rate_multiplier"`
	NewAccountStatsCost      *string `json:"new_account_stats_cost"`
	NewAccountRateMultiplier *string `json:"new_account_rate_multiplier"`
	OldCost                  string  `json:"old_cost,omitempty"`
	NewCost                  string  `json:"new_cost,omitempty"`
	Delta                    string  `json:"delta,omitempty"`
}

func financeCostFingerprint(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode finance cost snapshot: %w", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func financeCopyString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func financeCostDecimal(value string, scale int32, limit string) (decimal.Decimal, error) {
	// Bound parsing before decimal exponent arithmetic. Values come from SQL,
	// but audit snapshots are still validated before proposing destructive writes.
	if len(value) == 0 || len(value) > 64 || strings.ContainsAny(value, "eE") {
		return decimal.Zero, fmt.Errorf("invalid finance numeric value")
	}
	d, err := decimal.NewFromString(value)
	if err != nil || d.IsNegative() || !d.Equal(d.Round(scale)) || d.GreaterThanOrEqual(decimal.RequireFromString(limit)) {
		return decimal.Zero, fmt.Errorf("finance numeric value outside storage precision")
	}
	return d, nil
}

func financeUsageCostModel(in FinanceUsageCostInput) string {
	if model := strings.TrimSpace(in.UpstreamModel); model != "" {
		return model
	}
	if model := strings.TrimSpace(in.RequestedModel); model != "" {
		return model
	}
	return strings.TrimSpace(in.Model)
}

// CalculateFinanceUsageCost shares the online matchers and calculators, but
// rejects absent/invalid historical evidence before entering their fallback
// paths. It does not call live configuration services or mutate its arguments.
func CalculateFinanceUsageCost(in FinanceUsageCostInput, snapshot FinanceUsageCostSnapshot) (FinanceUsageCostResult, error) {
	out := FinanceUsageCostResult{
		Status: FinanceRecalculationSkipped, Model: financeUsageCostModel(in),
		OldAccountStatsCost:      financeCopyString(in.AccountStatsCost),
		OldAccountRateMultiplier: financeCopyString(in.AccountRateMultiplier),
	}
	var err error
	out.InputFingerprint, err = financeCostFingerprint(in)
	if err != nil {
		return out, err
	}
	out.RuleFingerprint, err = financeCostFingerprint(snapshot)
	if err != nil {
		return out, err
	}
	skip := func(reason string) (FinanceUsageCostResult, error) { out.Reason = reason; return out, nil }
	if in.ID <= 0 || in.AccountID <= 0 || in.CreatedAt.IsZero() || out.Model == "" {
		return skip("missing_usage_identity")
	}
	if snapshot.AccountID != in.AccountID || snapshot.GroupID != in.GroupID || snapshot.Model != out.Model {
		return out, fmt.Errorf("finance cost snapshot does not match usage scope")
	}
	if !snapshot.AcceptRetainedBaseline && snapshot.CoverageFrom != nil && in.CreatedAt.Before(*snapshot.CoverageFrom) {
		return skip("unproven_aggregate_coverage")
	}
	if snapshot.MissingReason != "" {
		return skip(snapshot.MissingReason)
	}
	total, err := financeCostDecimal(in.TotalCost, 10, "10000000000")
	if err != nil {
		return skip("invalid_original_cost")
	}
	oldBase, oldRate := total, decimal.NewFromInt(1)
	if in.AccountStatsCost != nil {
		oldBase, err = financeCostDecimal(*in.AccountStatsCost, 10, "10000000000")
		if err != nil {
			return skip("invalid_original_cost")
		}
	}
	if in.AccountRateMultiplier != nil {
		oldRate, err = financeCostDecimal(*in.AccountRateMultiplier, 4, "1000000")
		if err != nil {
			return skip("invalid_original_multiplier")
		}
	}
	out.OldCost = oldBase.Mul(oldRate).String()
	rate, err := financeCostDecimal(snapshot.DefaultMultiplier, 4, "1000000")
	if err != nil {
		return out, fmt.Errorf("invalid current account multiplier: %w", err)
	}
	rules, err := ValidateAccountModelCostRules(snapshot.ModelRates)
	if err != nil {
		return out, err
	}
	if matched, ok := newAccountModelCostIndex(rules).match(normalizeAccountModelCostName(out.Model)); ok {
		rate = decimal.NewFromFloat(matched)
	}

	// Reuse the exact online priority chain. No channel means the online path
	// uses total_cost, even when a model-file price happens to be available.
	var cost *float64
	useOriginalTotal := false
	out.Source = "original_total_cost"
	if ch := snapshot.Channel; ch != nil {
		var matched *ChannelModelPricing
		for i := range ch.Rules {
			rule := &ch.Rules[i]
			if !matchAccountStatsRule(rule, in.AccountID, in.GroupID) {
				continue
			}
			matched = findPricingForModel(rule.Pricing, snapshot.Platform, strings.ToLower(out.Model))
			if matched != nil {
				break
			}
		}
		if matched != nil {
			// Use the existing save-time validator on a clone: validation may
			// normalize empty time-pricing fields and must not mutate a snapshot.
			if validationErr := validateAccountStatsPricingRules([]AccountStatsPricingRule{{Pricing: []ChannelModelPricing{matched.Clone()}}}); validationErr != nil {
				return skip("invalid_cost_rule")
			}
			if !matched.BillingMode.IsValid() {
				return skip("unsupported_cost_mode")
			}
			if reason := financeValidateCostEvidence(in, matched.BillingMode); reason != "" {
				return skip(reason)
			}
			if matched.BillingMode == "" || matched.BillingMode == BillingModeToken {
				pricing := matched
				t := in.Tokens
				if iv := FindMatchingInterval(matched.Intervals, t.InputTokens+t.OutputTokens+t.CacheCreationTokens+t.CacheReadTokens); iv != nil {
					pricing = &ChannelModelPricing{CacheWrite1hPrice: iv.CacheWrite1hPrice}
				}
				if pricing.CacheWrite1hPrice != nil && !financeCacheBreakdownComplete(t) {
					return skip("incomplete_cache_duration_usage")
				}
			}
			count := 1
			if in.ImageCount > 0 {
				count = in.ImageCount
			}
			cost = calculateStatsCost(matched, in.Tokens, count)
			if cost != nil {
				*cost *= reasoningEffortBillingMultiplier(in.ReasoningEffort, matched.ReasoningEffortMultipliers)
				out.Source = "channel_cost_rule"
			} else {
				// The online path falls through here. A historical edit must not
				// silently replace an explicit unusable rule with another price.
				return skip("unusable_cost_rule")
			}
		} else if ch.ApplyPricingToAccountStats {
			if total.IsPositive() {
				useOriginalTotal = true
				out.Source = "original_customer_base"
			}
		} else if snapshot.ModelPricing != nil {
			if reason := financeValidateCostEvidence(in, BillingModeToken); reason != "" {
				return skip(reason)
			}
			p := snapshot.ModelPricing
			if p.SupportsCacheBreakdown && (p.CacheCreation5mPrice > 0 || p.CacheCreation1hPrice > 0) && !financeCacheBreakdownComplete(in.Tokens) {
				return skip("incomplete_cache_duration_usage")
			}
			pricingAt := in.CreatedAt // irrelevant to non-time-sensitive model policies
			if in.PricingAt != nil && !in.PricingAt.IsZero() {
				pricingAt = *in.PricingAt
			} else if isDeepSeekModel(out.Model) {
				return skip("missing_pricing_time")
			}
			// Frozen price only: neither resolver nor service has live pricing
			// dependencies. The unified calculator still applies tier, effort,
			// official time policy and current account long-context policy.
			bs := &BillingService{}
			resolved := &ResolvedPricing{Mode: BillingModeToken, Source: PricingSourceLiteLLM, BasePricing: snapshot.ModelPricing, SupportsCacheBreakdown: snapshot.ModelPricing.SupportsCacheBreakdown, longContextPricingEnabled: snapshot.LongContextPricingEnabled}
			breakdown, calcErr := bs.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: out.Model, Tokens: in.Tokens, RateMultiplier: 1, ServiceTier: in.ServiceTier, ReasoningEffort: in.ReasoningEffort, PricingAt: pricingAt, Resolver: NewModelPricingResolver(nil, bs), Resolved: resolved})
			if calcErr != nil {
				return out, fmt.Errorf("calculate frozen model cost: %w", calcErr)
			}
			if breakdown == nil || breakdown.TotalCost <= 0 {
				return skip("unusable_model_price")
			}
			cost = &breakdown.TotalCost
			out.Source = "model_price"
		}
	}
	newBase := total
	if useOriginalTotal {
		value := total.StringFixed(10)
		out.NewAccountStatsCost = &value
	}
	if cost != nil {
		if math.IsNaN(*cost) || math.IsInf(*cost, 0) || *cost < 0 || *cost >= 1e10 {
			return skip("invalid_calculated_cost")
		}
		if in.CacheStorageCost == nil {
			return skip("unknown_cache_storage_cost")
		}
		storage, storageErr := financeCostDecimal(*in.CacheStorageCost, 10, "10000000000")
		if storageErr != nil || storage.GreaterThan(total) {
			return skip("invalid_cache_storage_cost")
		}
		newBase = decimal.NewFromFloat(*cost).Add(storage).Round(10)
		if newBase.GreaterThanOrEqual(decimal.RequireFromString("10000000000")) {
			return skip("invalid_calculated_cost")
		}
		value := newBase.StringFixed(10)
		out.NewAccountStatsCost = &value
	}
	newRate := rate.StringFixed(4)
	out.NewAccountRateMultiplier = &newRate
	newCost := newBase.Mul(rate)
	out.NewCost = newCost.String()
	out.Delta = newCost.Sub(oldBase.Mul(oldRate)).String()
	out.Status = FinanceRecalculationChanged
	if financeEqualOptionalDecimal(in.AccountStatsCost, out.NewAccountStatsCost) && financeEqualOptionalDecimal(in.AccountRateMultiplier, out.NewAccountRateMultiplier) {
		out.Status = FinanceRecalculationUnchanged
	}
	return out, nil
}

func financeEqualOptionalDecimal(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	x, ex := decimal.NewFromString(*a)
	y, ey := decimal.NewFromString(*b)
	return ex == nil && ey == nil && x.Equal(y)
}

func financeCacheBreakdownComplete(tokens UsageTokens) bool {
	return tokens.CacheCreation5mTokens+tokens.CacheCreation1hTokens == tokens.CacheCreationTokens
}

func financeValidateCostEvidence(in FinanceUsageCostInput, mode BillingMode) string {
	t := in.Tokens
	for _, count := range []int{t.InputTokens, t.OutputTokens, t.CacheCreationTokens, t.CacheReadTokens, t.CacheCreation5mTokens, t.CacheCreation1hTokens, t.ImageInputTokens, t.ImageOutputTokens, t.ImageCacheReadTokens, in.ImageCount} {
		if count < 0 || count > math.MaxInt32 {
			return "invalid_usage_counts"
		}
	}
	if in.MediaType == "video" || mode == BillingModeVideo {
		return "unsupported_video_cost_evidence"
	}
	if mode == BillingModeImage || mode == BillingModePerRequest {
		if (mode == BillingModeImage || in.MediaType == "image" || in.BillingMode == BillingModeImage) && in.ImageCount == 0 {
			return "missing_image_count"
		}
		return ""
	}
	if t.InputTokens == 0 && t.OutputTokens == 0 && t.CacheCreationTokens == 0 && t.CacheReadTokens == 0 {
		return "missing_token_usage"
	}
	if !in.ImageCacheSplitKnown && t.ImageInputTokens > 0 && t.CacheReadTokens > 0 {
		return "missing_image_cache_split"
	}
	if t.ImageInputTokens > t.InputTokens || t.ImageOutputTokens > t.OutputTokens || t.ImageCacheReadTokens > t.CacheReadTokens {
		return "inconsistent_image_tokens"
	}
	if t.CacheCreation5mTokens+t.CacheCreation1hTokens > t.CacheCreationTokens {
		return "inconsistent_cache_tokens"
	}
	return ""
}

// FinancePaymentFeeInput keeps the NULL-vs-zero distinction used by finance
// reports. PayAmount is the original cash received; refund amounts are NOT
// subtracted from the fee basis (same contract as FinanceService.Recharge).
type FinancePaymentFeeInput struct {
	OrderID         int64   `json:"order_id"`
	ProviderID      int64   `json:"provider_id"`
	PayAmount       string  `json:"pay_amount"`
	OriginalFeeRate *string `json:"original_fee_rate"`
	CurrentFeeRate  string  `json:"current_fee_rate"`
}

type FinancePaymentFeeResult struct {
	Status           string  `json:"status"`
	InputFingerprint string  `json:"input_fingerprint"`
	OriginalFeeRate  *string `json:"original_fee_rate"`
	NewFeeRate       string  `json:"new_fee_rate"`
	OldFee           string  `json:"old_fee"`
	NewFee           string  `json:"new_fee"`
	Delta            string  `json:"delta"`
}

func CalculateFinancePaymentFee(in FinancePaymentFeeInput) (FinancePaymentFeeResult, error) {
	out := FinancePaymentFeeResult{OriginalFeeRate: financeCopyString(in.OriginalFeeRate)}
	if in.OrderID <= 0 || in.ProviderID <= 0 || strings.TrimSpace(in.CurrentFeeRate) == "" {
		return out, fmt.Errorf("missing payment fee calculation facts")
	}
	pay, err := financeCostDecimal(in.PayAmount, 8, "1000000000000")
	if err != nil {
		return out, err
	}
	out.NewFeeRate, err = ValidateChannelFeeRate(in.CurrentFeeRate)
	if err != nil {
		return out, err
	}
	current := decimal.RequireFromString(out.NewFeeRate)
	old := current
	if in.OriginalFeeRate != nil {
		if strings.TrimSpace(*in.OriginalFeeRate) == "" {
			return out, fmt.Errorf("invalid original payment fee rate")
		}
		raw, parseErr := ValidateChannelFeeRate(*in.OriginalFeeRate)
		if parseErr != nil {
			return out, parseErr
		}
		old = decimal.RequireFromString(raw)
	}
	out.OldFee, out.NewFee = pay.Mul(old).String(), pay.Mul(current).String()
	out.Delta = pay.Mul(current.Sub(old)).String()
	out.Status = FinanceRecalculationChanged
	if in.OriginalFeeRate != nil && old.Equal(current) {
		out.Status = FinanceRecalculationUnchanged
	}
	out.InputFingerprint, err = financeCostFingerprint(in)
	return out, err
}
