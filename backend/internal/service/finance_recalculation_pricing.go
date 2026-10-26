package service

import (
	"encoding/json"
	"errors"
)

// LockFinancePricing freezes the live price generation through a caller-owned
// database transaction. The isolated reader avoids recursive RLock deadlocks
// when a reload is waiting for the original write lock. No live map is mutated.
func (s *BillingService) LockFinancePricing() (func(string) (*ModelPricing, error), func()) {
	frozen := &BillingService{cfg: s.cfg, fallbackPrices: s.fallbackPrices}
	release := func() {}
	if s.pricingService != nil {
		p := s.pricingService
		p.mu.RLock()
		release = p.mu.RUnlock
		frozen.pricingService = &PricingService{cfg: p.cfg, pricingData: p.pricingData}
	}
	return func(model string) (*ModelPricing, error) {
		price, err := frozen.GetModelPricing(model)
		if errors.Is(err, ErrModelPricingUnavailable) {
			return nil, nil
		}
		if err != nil || price == nil {
			return nil, err
		}
		// Detach maps/slices as well as the top-level price from the live generation.
		data, err := json.Marshal(price)
		if err != nil {
			return nil, err
		}
		var copy ModelPricing
		if err = json.Unmarshal(data, &copy); err != nil {
			return nil, err
		}
		return &copy, nil
	}, release
}

func FinanceRecalculationConfigFingerprint(config FinanceRecalculationConfig) (string, error) {
	return financeCostFingerprint(config)
}

// Current account policy, using the same subscription exception as online cost.
// Only whitelisted account fields are provided by the snapshot repository.
func FinanceAccountLongContextPolicy(platform, accountType, model string, enabled bool) bool {
	account := &Account{Platform: platform, Type: accountType, Extra: map[string]any{openAILongContextBillingEnabledKey: enabled}}
	return accountStatsLongContextPricingEnabled(openAILongContextBillingGate(account)) && !openAISubscriptionLongContextExempt(account, &OpenAIForwardResult{UpstreamModel: model})
}
