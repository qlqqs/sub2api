//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func financeChannelCostFixture(t *testing.T) (*service.Channel, *channelRepository, service.FinanceChannelCostRepository) {
	t.Helper()
	price, zero := 0.123456789, 0.0
	ch := &service.Channel{Description: "untouched", Features: "[\"feature\"]", FeaturesConfig: map[string]any{"private": "not in cost snapshot"}, ModelMapping: map[string]map[string]string{"openai": {"public": "upstream"}}, ModelPricing: []service.ChannelModelPricing{{Platform: "openai", Models: []string{"public"}, InputPrice: &price}}, AccountStatsPricingRules: []service.AccountStatsPricingRule{{Name: "original", SortOrder: 0, GroupIDs: []int64{}, AccountIDs: []int64{8}, Pricing: []service.ChannelModelPricing{{Platform: "", Models: []string{"gpt-*"}, InputPrice: &price, CacheWrite1hPrice: &price, ImageOutputPrice: &zero, ReasoningEffortMultipliers: map[string]float64{"high": 2}, Intervals: []service.PricingInterval{{InputPrice: &price, CacheWrite1hPrice: &price, SortOrder: 3}}}}}}}
	legacy := newReasoningPricingIntegrationChannel(t, ch)
	return ch, legacy, NewFinanceChannelCostRepository(integrationDB)
}

func TestFinanceChannelCostRoundTripPreservesUnrelatedConfig(t *testing.T) {
	ctx := context.Background()
	ch, legacy, repo := financeChannelCostFixture(t)
	before, err := legacy.GetByID(ctx, ch.ID)
	require.NoError(t, err)
	config, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	s := service.NewFinanceChannelCostService(repo, nil)
	rules := before.AccountStatsPricingRules
	rules[0].Name = "updated"
	yes := true
	result, err := s.Update(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: config.Version, ApplyPricingToAccountStats: &yes, AccountStatsPricingRules: &rules})
	require.NoError(t, err)
	require.NotEqual(t, config.Version, result.Version)
	after, err := legacy.GetByID(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, before.ModelPricing, after.ModelPricing) // Includes unchanged IDs/timestamps/precision.
	require.Equal(t, before.ModelMapping, after.ModelMapping)
	require.Equal(t, before.FeaturesConfig, after.FeaturesConfig)
	require.Equal(t, before.GroupIDs, after.GroupIDs)
	require.Equal(t, before.Features, after.Features)
	require.Equal(t, before.Description, after.Description)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.BillingModelSource, after.BillingModelSource)
	require.Equal(t, after.AccountStatsPricingRules, result.AccountStatsPricingRules)
	require.Empty(t, result.AccountStatsPricingRules[0].Pricing[0].Platform)
	require.Equal(t, before.AccountStatsPricingRules[0].Pricing[0].CacheWrite1hPrice, result.AccountStatsPricingRules[0].Pricing[0].CacheWrite1hPrice)
	require.Equal(t, before.AccountStatsPricingRules[0].Pricing[0].ReasoningEffortMultipliers, result.AccountStatsPricingRules[0].Pricing[0].ReasoningEffortMultipliers)
	require.Equal(t, 3, result.AccountStatsPricingRules[0].Pricing[0].Intervals[0].SortOrder)
	// Changing only the toggle must not replace rules (including their IDs).
	no := false
	toggled, err := s.Update(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: result.Version, ApplyPricingToAccountStats: &no})
	require.NoError(t, err)
	require.Equal(t, result.AccountStatsPricingRules, toggled.AccountStatsPricingRules)
	_, err = s.Update(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: result.Version, ApplyPricingToAccountStats: &yes})
	require.ErrorIs(t, err, service.ErrFinanceCostConfigChanged)
	// Row recreation with identical business values does not spuriously drift the content version.
	same, err := s.Update(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: toggled.Version, AccountStatsPricingRules: &toggled.AccountStatsPricingRules})
	require.NoError(t, err)
	require.Equal(t, toggled.Version, same.Version)
	empty := []service.AccountStatsPricingRule{}
	cleared, err := s.Update(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: same.Version, AccountStatsPricingRules: &empty})
	require.NoError(t, err)
	require.Empty(t, cleared.AccountStatsPricingRules)
	oldRead, err := legacy.GetByID(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, oldRead.AccountStatsPricingRules)
}

func TestFinanceChannelCostConcurrentCAS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, _, repo := financeChannelCostFixture(t)
	initial, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			yes := true
			_, err := repo.CompareAndSwap(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: initial.Version, ApplyPricingToAccountStats: &yes})
			done <- err
		}()
	}
	close(start)
	first, second := <-done, <-done
	if first == nil {
		require.ErrorIs(t, second, service.ErrFinanceCostConfigChanged)
	} else {
		require.ErrorIs(t, first, service.ErrFinanceCostConfigChanged)
		require.NoError(t, second)
	}
}

func TestFinanceChannelCostLocksBeforeReadingVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, _, repo := financeChannelCostFixture(t)
	initial, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var pid int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	_, err = tx.ExecContext(ctx, `UPDATE channels SET apply_pricing_to_account_stats=TRUE WHERE id=$1`, ch.ID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		no := false
		_, err := repo.CompareAndSwap(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: initial.Version, ApplyPricingToAccountStats: &no})
		done <- err
	}()
	// Observe the actual PostgreSQL wait, rather than assume timing via Sleep.
	require.Eventually(t, func() bool {
		var count int
		err := integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))`, pid).Scan(&count)
		return err == nil && count > 0
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit())
	require.ErrorIs(t, <-done, service.ErrFinanceCostConfigChanged)
	current, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.True(t, current.ApplyPricingToAccountStats)
}

func TestFinanceChannelCostRollbackAndMissing(t *testing.T) {
	ctx := context.Background()
	ch, _, repo := financeChannelCostFixture(t)
	before, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	bad := []service.AccountStatsPricingRule{{Name: strings.Repeat("x", 101), GroupIDs: []int64{}, AccountIDs: []int64{8}}}
	yes := true
	_, err = repo.CompareAndSwap(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: before.Version, ApplyPricingToAccountStats: &yes, AccountStatsPricingRules: &bad})
	require.Error(t, err)
	after, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, err = repo.Get(ctx, -1)
	require.ErrorIs(t, err, service.ErrChannelNotFound)
	_, err = repo.CompareAndSwap(ctx, -1, service.FinanceChannelCostUpdate{ExpectedVersion: before.Version, ApplyPricingToAccountStats: &yes})
	require.ErrorIs(t, err, service.ErrChannelNotFound)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = repo.CompareAndSwap(canceled, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: before.Version, ApplyPricingToAccountStats: &yes})
	require.ErrorIs(t, err, context.Canceled)
	after, err = repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestFinanceChannelCostDetectsLegacyEditsButNotUnrelatedFields(t *testing.T) {
	ctx := context.Background()
	ch, legacy, repo := financeChannelCostFixture(t)
	before, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	old, err := legacy.GetByID(ctx, ch.ID)
	require.NoError(t, err)
	old.Description = "cosmetic edit"
	require.NoError(t, legacy.Update(ctx, old))
	unchanged, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, before.Version, unchanged.Version)
	old, err = legacy.GetByID(ctx, ch.ID)
	require.NoError(t, err)
	price := 0.9
	old.AccountStatsPricingRules[0].Pricing[0].CacheWrite1hPrice = &price
	require.NoError(t, legacy.Update(ctx, old))
	changed, err := repo.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.NotEqual(t, before.Version, changed.Version)
	yes := true
	_, err = repo.CompareAndSwap(ctx, ch.ID, service.FinanceChannelCostUpdate{ExpectedVersion: before.Version, ApplyPricingToAccountStats: &yes})
	require.ErrorIs(t, err, service.ErrFinanceCostConfigChanged)
}
