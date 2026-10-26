package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// FinanceChannelCostConfigVersion is a content version of ONLY the editor's
// cost switch and custom cost rules. Names and first-match order are included;
// row IDs, parent IDs and timestamps are excluded because rule replacement
// recreates them. nil/empty collections are equivalent; scope IDs are sets.
// JSON map keys are sorted by encoding/json. Pointer prices preserve NULL vs 0.
// Recalculation guards can reuse this for custom cost configuration, but MUST
// additionally snapshot channel status/group membership and main model pricing
// when ApplyPricingToAccountStats enables that fallback (and account/group/model
// file pricing independently). This is not a version of an entire channel.
func FinanceChannelCostConfigVersion(apply bool, rules []AccountStatsPricingRule) (string, error) {
	data, err := CanonicalFinanceChannelCostConfig(apply, rules)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "fcc1:" + hex.EncodeToString(sum[:]), nil
}

// CanonicalFinanceChannelCostConfig produces a credential-free, explicit field
// whitelist suitable for persistence. It never marshals Channel itself.
func CanonicalFinanceChannelCostConfig(apply bool, rules []AccountStatsPricingRule) ([]byte, error) {
	type interval struct {
		MinTokens                                                                                    int    `json:"min_tokens"`
		MaxTokens                                                                                    *int   `json:"max_tokens"`
		TierLabel                                                                                    string `json:"tier_label"`
		InputPrice, OutputPrice, CacheWritePrice, CacheWrite1hPrice, CacheReadPrice, PerRequestPrice *float64
		InputMultiplier, OutputMultiplier, CacheWriteMultiplier, CacheReadMultiplier                 *float64
		SortOrder                                                                                    int `json:"sort_order"`
	}
	type pricing struct {
		Platform                                                                           string      `json:"platform"`
		Models                                                                             []string    `json:"models"`
		BillingMode                                                                        BillingMode `json:"billing_mode"`
		InputPrice, OutputPrice, CacheWritePrice, CacheWrite1hPrice, CacheReadPrice        *float64
		FastMultiplier, FlexMultiplier, ImageInputPrice, ImageOutputPrice, PerRequestPrice *float64
		ReasoningEffortMultipliers                                                         map[string]float64  `json:"reasoning_effort_multipliers,omitempty"`
		Intervals                                                                          []interval          `json:"intervals"`
		TimePricing                                                                        *ChannelTimePricing `json:"time_pricing,omitempty"`
	}
	type rule struct {
		Name                 string `json:"name"`
		GroupIDs, AccountIDs []int64
		SortOrder            int       `json:"sort_order"`
		Pricing              []pricing `json:"pricing"`
	}
	value := struct {
		Apply bool   `json:"apply_pricing_to_account_stats"`
		Rules []rule `json:"account_stats_pricing_rules"`
	}{Apply: apply, Rules: make([]rule, 0, len(rules))}
	ordered := append([]AccountStatsPricingRule(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SortOrder < ordered[j].SortOrder })
	for _, r := range ordered {
		cr := rule{Name: r.Name, GroupIDs: financeCostScopeIDs(r.GroupIDs), AccountIDs: financeCostScopeIDs(r.AccountIDs), SortOrder: r.SortOrder, Pricing: make([]pricing, 0, len(r.Pricing))}
		for _, p := range r.Pricing {
			mode := p.BillingMode
			if mode == "" {
				mode = BillingModeToken
			}
			cp := pricing{Platform: p.Platform, Models: append([]string{}, p.Models...), BillingMode: mode,
				InputPrice: p.InputPrice, OutputPrice: p.OutputPrice, CacheWritePrice: p.CacheWritePrice, CacheWrite1hPrice: p.CacheWrite1hPrice, CacheReadPrice: p.CacheReadPrice,
				FastMultiplier: p.FastMultiplier, FlexMultiplier: p.FlexMultiplier, ImageInputPrice: p.ImageInputPrice, ImageOutputPrice: p.ImageOutputPrice, PerRequestPrice: p.PerRequestPrice,
				ReasoningEffortMultipliers: p.ReasoningEffortMultipliers, Intervals: make([]interval, 0, len(p.Intervals)), TimePricing: p.TimePricing}
			ivs := append([]PricingInterval(nil), p.Intervals...)
			sort.SliceStable(ivs, func(i, j int) bool { return ivs[i].SortOrder < ivs[j].SortOrder })
			for _, iv := range ivs {
				cp.Intervals = append(cp.Intervals, interval{MinTokens: iv.MinTokens, MaxTokens: iv.MaxTokens, TierLabel: iv.TierLabel,
					InputPrice: iv.InputPrice, OutputPrice: iv.OutputPrice, CacheWritePrice: iv.CacheWritePrice, CacheWrite1hPrice: iv.CacheWrite1hPrice, CacheReadPrice: iv.CacheReadPrice, PerRequestPrice: iv.PerRequestPrice,
					InputMultiplier: iv.InputMultiplier, OutputMultiplier: iv.OutputMultiplier, CacheWriteMultiplier: iv.CacheWriteMultiplier, CacheReadMultiplier: iv.CacheReadMultiplier, SortOrder: iv.SortOrder})
			}
			cr.Pricing = append(cr.Pricing, cp)
		}
		value.Rules = append(value.Rules, cr)
	}
	return json.Marshal(value)
}

func financeCostScopeIDs(ids []int64) []int64 {
	result := append([]int64{}, ids...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	n := 0
	for _, id := range result {
		if n == 0 || result[n-1] != id {
			result[n] = id
			n++
		}
	}
	return result[:n]
}
