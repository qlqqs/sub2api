//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAffiliateGroupRebateFactorsValidationAndRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		factors []AffiliateGroupRebateFactor
		valid   bool
	}{
		{"negative", []AffiliateGroupRebateFactor{{1, -1}}, false},
		{"too high", []AffiliateGroupRebateFactor{{1, 101}}, false},
		{"NaN", []AffiliateGroupRebateFactor{{1, math.NaN()}}, false},
		{"duplicate", []AffiliateGroupRebateFactor{{1, 30}, {1, 100}}, false},
		{"missing", []AffiliateGroupRebateFactor{{999, 100}}, false},
		{"normalized", []AffiliateGroupRebateFactor{{2, 100}, {1, 30.125}, {3, 0}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &settingUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})
			svc.SetDefaultSubscriptionGroupReader(&defaultSubGroupReaderStub{byID: map[int64]*Group{1: {ID: 1, Name: "视频"}, 2: {ID: 2}, 3: {ID: 3}}})
			settings := &SystemSettings{AffiliateGroupRebateFactors: tc.factors}
			err := svc.UpdateSettings(context.Background(), settings)
			if !tc.valid {
				require.Error(t, err)
				require.Nil(t, repo.updates)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, `[{"group_id":1,"factor":30.13},{"group_id":3,"factor":0}]`, repo.updates[SettingKeyAffiliateGroupRebateFactors])
			parsed := svc.parseSettings(repo.updates)
			require.Equal(t, []AffiliateGroupRebateFactor{{1, 30.13}, {3, 0}}, parsed.AffiliateGroupRebateFactors)
			views := svc.AffiliateGroupRebateFactorViews(context.Background(), parsed.AffiliateGroupRebateFactors)
			require.Equal(t, "视频", views[0].GroupName)
		})
	}
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{}))
	require.NotContains(t, repo.updates, SettingKeyAffiliateGroupRebateFactors, "omitted field is not written")
	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{AffiliateGroupRebateFactors: []AffiliateGroupRebateFactor{}}))
	require.Equal(t, "[]", repo.updates[SettingKeyAffiliateGroupRebateFactors])
	require.Empty(t, parseAffiliateGroupRebateFactors(`bad json`))
	require.Empty(t, parseAffiliateGroupRebateFactors(`null`))
	require.Equal(t, []AffiliateGroupRebateFactor{{1, 0}, {2, 30}}, parseAffiliateGroupRebateFactors(`[{"group_id":2,"factor":30},{"group_id":1,"factor":0}]`))
	svc.SetDefaultSubscriptionGroupReader(&defaultSubGroupReaderStub{byID: map[int64]*Group{}})
	require.Empty(t, svc.AffiliateGroupRebateFactorViews(context.Background(), []AffiliateGroupRebateFactor{{999, 30}})[0].GroupName)
}

func TestWeightedEligibleConsumption(t *testing.T) {
	for _, tc := range []struct {
		name                                                string
		raw, weighted, paid, settled, wantRaw, wantWeighted float64
	}{
		{"partial cap", 100, 30, 60, 10, 50, 15},
		{"zero factor", 100, 0, 150, 0, 100, 0},
		{"mixed groups", 80, 39, 100, 0, 80, 39},
		{"zero consumption", 0, 0, 100, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &AffiliatePendingConsumption{DeltaCost: tc.raw, WeightedDeltaCost: tc.weighted, PaidTotal: tc.paid, SettledConsumption: tc.settled}
			raw := eligibleConsumption(p)
			require.Equal(t, tc.wantRaw, raw)
			require.Equal(t, tc.wantWeighted, weightedEligibleConsumption(p, raw))
		})
	}
}

func TestSettleConsumptionRebatesWeightedAndZeroAdvance(t *testing.T) {
	for _, weighted := range []float64{24, 0} {
		svc, repo := newExampleChain(t)
		svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
			SettingKeyAffiliateEnabled: "true", SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
			SettingKeyAffiliateRebateRate: "30", SettingKeyAffiliateAgentLevel2Rate: "20",
		}}, nil)
		now := time.Now()
		repo.pendingUsers = []int64{userU2}
		repo.pending[userU2] = &AffiliatePendingConsumption{UserID: userU2, SettledAt: now.Add(-time.Hour), UntilAt: now, DeltaCost: 80, WeightedDeltaCost: weighted, PaidTotal: 50}
		result, err := svc.SettleConsumptionRebates(context.Background(), 0)
		require.NoError(t, err)
		require.Equal(t, 50.0, result.Consumption)
		require.Equal(t, 50.0, repo.advances[userU2])
		require.InDelta(t, weighted*50/80*.3, result.Rebate, 1e-8)
		require.Equal(t, now, repo.pending[userU2].SettledAt)
	}
}

func TestFillAffiliateTeamRebateFactors(t *testing.T) {
	one, two, other := int64(1), int64(2), int64(99)
	items := []AffiliateTeamRequest{{GroupID: &one}, {GroupID: &two}, {GroupID: &other}, {}}
	fillAffiliateTeamRebateFactors(items, []AffiliateGroupRebateFactor{{1, 30}, {2, 0}})
	require.Equal(t, []float64{30, 0, 100, 100}, []float64{items[0].RebateFactor, items[1].RebateFactor, items[2].RebateFactor, items[3].RebateFactor})
}
