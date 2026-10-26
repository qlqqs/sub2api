//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func financeCostTestRules() []AccountStatsPricingRule {
	price := 0.123
	return []AccountStatsPricingRule{{Name: "cost", GroupIDs: []int64{2, 1}, AccountIDs: []int64{8}, Pricing: []ChannelModelPricing{{Models: []string{"gpt-*"}, BillingMode: BillingModeToken, InputPrice: &price, ReasoningEffortMultipliers: map[string]float64{"high": 2}, Intervals: []PricingInterval{{InputPrice: &price, SortOrder: 4}}}}}}
}

func TestFinanceChannelCostFingerprint(t *testing.T) {
	rules := financeCostTestRules()
	version, err := FinanceChannelCostConfigVersion(false, rules)
	require.NoError(t, err)
	clone := (&Channel{AccountStatsPricingRules: rules}).Clone().AccountStatsPricingRules
	clone[0].ID = 99
	clone[0].ChannelID = 42
	clone[0].UpdatedAt = time.Now()
	clone[0].GroupIDs = []int64{1, 2, 1}
	clone[0].Pricing[0].ID = 188
	clone[0].Pricing[0].CreatedAt = time.Now()
	clone[0].Pricing[0].Intervals[0].ID = 200
	clone[0].Pricing[0].Intervals[0].PricingID = 188
	same, err := FinanceChannelCostConfigVersion(false, clone)
	require.NoError(t, err)
	require.Equal(t, version, same)
	mutations := map[string]func([]AccountStatsPricingRule){
		"scope":       func(r []AccountStatsPricingRule) { r[0].AccountIDs = []int64{9} },
		"order":       func(r []AccountStatsPricingRule) { r[0].SortOrder++ },
		"name":        func(r []AccountStatsPricingRule) { r[0].Name = "other" },
		"platform":    func(r []AccountStatsPricingRule) { r[0].Pricing[0].Platform = "openai" },
		"model":       func(r []AccountStatsPricingRule) { r[0].Pricing[0].Models = []string{"gpt-5"} },
		"reasoning":   func(r []AccountStatsPricingRule) { r[0].Pricing[0].ReasoningEffortMultipliers["high"] = 3 },
		"interval":    func(r []AccountStatsPricingRule) { r[0].Pricing[0].Intervals[0].SortOrder++ },
		"null-v-zero": func(r []AccountStatsPricingRule) { v := 0.0; r[0].Pricing[0].CacheWrite1hPrice = &v },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copy := (&Channel{AccountStatsPricingRules: rules}).Clone().AccountStatsPricingRules
			mutate(copy)
			changed, err := FinanceChannelCostConfigVersion(false, copy)
			require.NoError(t, err)
			require.NotEqual(t, version, changed)
		})
	}
	changed, err := FinanceChannelCostConfigVersion(true, rules)
	require.NoError(t, err)
	require.NotEqual(t, version, changed)
	a, err := FinanceChannelCostConfigVersion(false, nil)
	require.NoError(t, err)
	b, err := FinanceChannelCostConfigVersion(false, []AccountStatsPricingRule{})
	require.NoError(t, err)
	require.Equal(t, a, b)
}

type financeCostRepoStub struct {
	calls int
	input FinanceChannelCostUpdate
	err   error
}

func (r *financeCostRepoStub) Get(context.Context, int64) (*FinanceChannelCostConfig, error) {
	return &FinanceChannelCostConfig{}, r.err
}
func (r *financeCostRepoStub) CompareAndSwap(_ context.Context, id int64, input FinanceChannelCostUpdate) (*FinanceChannelCostConfig, error) {
	r.calls++
	r.input = input
	return &FinanceChannelCostConfig{ChannelID: id}, r.err
}

func TestFinanceChannelCostUpdateValidation(t *testing.T) {
	cases := map[string]func(*FinanceChannelCostUpdate){
		"missing version": func(i *FinanceChannelCostUpdate) { i.ExpectedVersion = "" },
		"empty scope": func(i *FinanceChannelCostUpdate) {
			(*i.AccountStatsPricingRules)[0].GroupIDs = nil
			(*i.AccountStatsPricingRules)[0].AccountIDs = nil
		},
		"empty prices": func(i *FinanceChannelCostUpdate) { (*i.AccountStatsPricingRules)[0].Pricing = nil },
		"negative price": func(i *FinanceChannelCostUpdate) {
			v := -1.0
			(*i.AccountStatsPricingRules)[0].Pricing[0].InputPrice = &v
		},
		"nan": func(i *FinanceChannelCostUpdate) {
			v := math.NaN()
			(*i.AccountStatsPricingRules)[0].Pricing[0].InputPrice = &v
		},
		"fast": func(i *FinanceChannelCostUpdate) {
			v := 2.0
			(*i.AccountStatsPricingRules)[0].Pricing[0].FastMultiplier = &v
		},
		"image input": func(i *FinanceChannelCostUpdate) {
			v := 0.0
			(*i.AccountStatsPricingRules)[0].Pricing[0].ImageInputPrice = &v
		},
		"interval multiplier": func(i *FinanceChannelCostUpdate) {
			v := 2.0
			(*i.AccountStatsPricingRules)[0].Pricing[0].Intervals[0].InputMultiplier = &v
		},
		"time pricing": func(i *FinanceChannelCostUpdate) {
			(*i.AccountStatsPricingRules)[0].Pricing[0].TimePricing = &ChannelTimePricing{Periods: []ChannelTimePricingPeriod{{Multiplier: 2}}}
		},
		"conflict model": func(i *FinanceChannelCostUpdate) {
			(*i.AccountStatsPricingRules)[0].Pricing[0].Models = []string{"gpt-*", "gpt-5"}
		},
		"invalid mode": func(i *FinanceChannelCostUpdate) { (*i.AccountStatsPricingRules)[0].Pricing[0].BillingMode = "bogus" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rules := financeCostTestRules()
			input := FinanceChannelCostUpdate{ExpectedVersion: "v", AccountStatsPricingRules: &rules}
			mutate(&input)
			repo := &financeCostRepoStub{}
			s := NewFinanceChannelCostService(repo, nil)
			_, err := s.Update(context.Background(), 1, input)
			require.Error(t, err)
			require.Zero(t, repo.calls)
		})
	}
	repo := &financeCostRepoStub{}
	s := NewFinanceChannelCostService(repo, nil)
	rules := financeCostTestRules()
	rules[0].SortOrder = 9
	_, err := s.Update(context.Background(), 1, FinanceChannelCostUpdate{ExpectedVersion: "v", AccountStatsPricingRules: &rules})
	require.NoError(t, err)
	require.Equal(t, 9, rules[0].SortOrder)
	require.Equal(t, 0, (*repo.input.AccountStatsPricingRules)[0].SortOrder)
	require.Equal(t, "", (*repo.input.AccountStatsPricingRules)[0].Pricing[0].Platform)
	empty := []AccountStatsPricingRule{}
	_, err = s.Update(context.Background(), 1, FinanceChannelCostUpdate{ExpectedVersion: "v", AccountStatsPricingRules: &empty})
	require.NoError(t, err)
	require.NotNil(t, repo.input.AccountStatsPricingRules)
	require.Empty(t, *repo.input.AccountStatsPricingRules)
	for _, failure := range []error{ErrFinanceCostConfigChanged, errors.New("db unavailable")} {
		repo.err = failure
		_, err = s.Update(context.Background(), 1, FinanceChannelCostUpdate{ExpectedVersion: "v", AccountStatsPricingRules: &empty})
		require.ErrorIs(t, err, failure)
	}
}

func TestFinanceChannelCostInvalidatesExistingChannelCacheOnlyAfterCommit(t *testing.T) {
	ctx := context.Background()
	rebuilds, notifications := 0, 0
	legacy := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { rebuilds++; return nil, nil }}
	bus := &mockChannelCachePubSub{}
	bus.SubscribeUpdates(ctx, func() { notifications++ })
	channels := NewChannelService(legacy, nil, nil, nil, bus)
	repo := &financeCostRepoStub{err: ErrFinanceCostConfigChanged}
	s := NewFinanceChannelCostService(repo, channels)
	yes := true
	input := FinanceChannelCostUpdate{ExpectedVersion: "v", ApplyPricingToAccountStats: &yes}
	_, err := s.Update(ctx, 1, input)
	require.ErrorIs(t, err, ErrFinanceCostConfigChanged)
	require.Zero(t, rebuilds)
	require.Zero(t, notifications)
	repo.err = nil
	_, err = s.Update(ctx, 1, input)
	require.NoError(t, err)
	require.Equal(t, 1, rebuilds)
	require.Equal(t, 1, notifications)
}
