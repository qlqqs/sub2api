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

func TestInviteeSignupBonusRegistrationAndFirstRechargeStack(t *testing.T) {
	for _, level := range []int{0, 1, 2} {
		t.Run(string(rune('0'+level)), func(t *testing.T) {
			ctx := context.Background()
			repo := newFakeAffiliateRepo()
			repo.row(1).AgentLevel = level
			svc, settings := newTwoTierService(repo, map[string]string{
				SettingKeyAffiliateInviteeSignupBonus: "5.123456789",
				SettingKeyAffiliateInviteeBonusMode:   AffiliateInviteeBonusModeFixed,
				SettingKeyAffiliateInviteeBonusValue:  "3",
			})
			bind(t, svc, 2, 1)
			require.InDelta(t, 5.12345679, repo.balances[2], 1e-9)
			require.Len(t, repo.ledger, 1)
			e := repo.ledger[0]
			require.Equal(t, int64(2), e.userID)
			require.Equal(t, e.userID, e.sourceUserID)
			require.Equal(t, AffiliateRewardInviteeSignupBonus, e.action)
			require.Equal(t, e.action, e.rewardType)
			// 绑定重放、金额变更均不补发，也不消耗首充资格。
			settings.values[SettingKeyAffiliateInviteeSignupBonus] = "9"
			bind(t, svc, 2, 1)
			require.Len(t, repo.ledger, 1)
			detail, err := svc.GetAffiliateDetail(ctx, 2)
			require.NoError(t, err)
			require.NotNil(t, detail.FirstRechargeBonus)
			require.Equal(t, 9.0, detail.InviteeSignupBonus)
			require.InDelta(t, 5.12345679, detail.RewardTotals.InviteeSignupBonus, 1e-9)
			res, err := svc.ApplyFirstRechargeRewards(ctx, 2, 20, nil)
			require.NoError(t, err)
			require.Equal(t, 3.0, res.InviteeBonus)
			require.InDelta(t, 8.12345679, repo.balances[2], 1e-9)
			res, err = svc.ApplyFirstRechargeRewards(ctx, 2, 20, nil)
			require.NoError(t, err)
			require.Zero(t, res.InviteeBonus)
		})
	}
}

func TestInviteeSignupBonusRegistrationGuards(t *testing.T) {
	for _, tc := range []struct {
		name, enabled, amount, code string
		wantErr                     bool
	}{
		{"总开关关闭", "false", "5", "CODE1", false},
		{"默认关闭", "true", "", "CODE1", false},
		{"金额为零", "true", "0", "CODE1", false},
		{"非法存储金额", "true", "NaN", "CODE1", false},
		{"没有邀请码", "true", "5", "", false},
		{"不存在的邀请码", "true", "5", "MISSING", true},
		{"格式非法", "true", "5", "!", true},
		{"自邀", "true", "5", "CODE2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeAffiliateRepo()
			repo.row(1)
			repo.row(2)
			svc, _ := newTwoTierService(repo, map[string]string{SettingKeyAffiliateEnabled: tc.enabled, SettingKeyAffiliateInviteeSignupBonus: tc.amount})
			err := svc.BindInviterByCode(context.Background(), 2, tc.code)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, repo.ledger)
			require.Zero(t, repo.balances[2])
		})
	}
}

type signupBonusFailRepo struct {
	*fakeAffiliateRepo
	calls int
}

func (r *signupBonusFailRepo) CreditInviteeSignupBonus(context.Context, int64, float64) (bool, error) {
	r.calls++
	return false, errors.New("模拟入账失败")
}
func TestInviteeSignupBonusCreditFailureDoesNotBlockRegistration(t *testing.T) {
	base := newFakeAffiliateRepo()
	base.row(1)
	svc, _ := newTwoTierService(base, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
	repo := &signupBonusFailRepo{fakeAffiliateRepo: base}
	svc.repo = repo
	require.NoError(t, svc.BindInviterByCode(context.Background(), 2, "CODE1"))
	require.Equal(t, int64(1), *base.row(2).InviterID)
	require.Equal(t, 1, repo.calls)
	// 已绑定后不补发失败的赠送，也不影响后续登录。
	require.NoError(t, svc.BindInviterByCode(context.Background(), 2, "CODE1"))
	require.Equal(t, 1, repo.calls)
}

func TestInviteeSignupBonusAdminBindNeverGrants(t *testing.T) {
	repo := newFakeAffiliateRepo()
	repo.row(1)
	svc, _ := newTwoTierService(repo, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
	_, err := svc.AdminBindInviter(context.Background(), 2, 1, 99, "测试管理员补绑")
	require.NoError(t, err)
	require.Empty(t, repo.ledger)
	require.Zero(t, repo.balances[2])
}

func TestInviteeSignupBonusOAuthRegistration(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAffiliateRepo()
	repo.row(1)
	affiliate, _ := newTwoTierService(repo, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
	auth := newEmailOAuthAutoAuthService(&userRepoStub{nextID: 88}, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil)
	auth.affiliateService = affiliate
	u, err := auth.createEmailOAuthUser(ctx, "signup-bonus@example.com", "注册赠送", "github", "", "CODE1")
	require.NoError(t, err)
	require.Equal(t, 5.0, repo.balances[u.ID])
	auth.bindOAuthAffiliate(ctx, u, "CODE1")
	require.Len(t, repo.ledger, 1)
}

func TestInviteeSignupBonusSettingsAndPublic(t *testing.T) {
	ctx := context.Background()
	svc := newSettingServiceForPlatformThresholdTest(nil)
	for _, v := range []float64{-1, math.NaN(), math.Inf(1)} {
		_, err := svc.buildSystemSettingsUpdates(ctx, &SystemSettings{AffiliateInviteeSignupBonus: v})
		require.Error(t, err)
	}
	updates, err := svc.buildSystemSettingsUpdates(ctx, &SystemSettings{AffiliateInviteeSignupBonus: 5.123456789})
	require.NoError(t, err)
	require.Equal(t, "5.12345679", updates[SettingKeyAffiliateInviteeSignupBonus])
	for _, enabled := range []bool{true, false} {
		public := &PublicSettings{AffiliateEnabled: enabled}
		applyPublicInviteeBonus(public, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
		if enabled {
			require.Equal(t, 5.0, public.AffiliateInviteeSignupBonus)
		} else {
			require.Zero(t, public.AffiliateInviteeSignupBonus)
		}
		require.Equal(t, AffiliateInviteeBonusModeOff, public.AffiliateInviteeBonusMode)
	}
	config := parseAffiliateRewardConfig(map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
	settings := &SystemSettings{}
	config.applyToSettings(settings)
	require.Equal(t, 5.0, settings.AffiliateInviteeSignupBonus)
	require.Equal(t, 5.0, affiliateRewardConfigFromSettings(settings).InviteeSignupBonus)
}

func TestInviteeSignupBonusConsumptionCapUnchanged(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAffiliateRepo()
	repo.row(1)
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateInviteeSignupBonus: "5", SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
		SettingKeyAffiliateInviteRewardMode: AffiliateInviteRewardModePercent, SettingKeyAffiliateInviteRewardRate: "10",
	})
	bind(t, svc, 2, 1)
	now := time.Now()
	repo.pendingUsers = []int64{2}
	repo.pending[2] = &AffiliatePendingConsumption{UserID: 2, SettledAt: now.Add(-time.Hour), UntilAt: now, DeltaCost: 5, PaidTotal: 0}
	res, err := svc.SettleConsumptionRebates(ctx, time.Second)
	require.NoError(t, err)
	require.Zero(t, res.Rebate)
	require.Zero(t, res.Consumption)
	require.Len(t, repo.ledger, 1)
}
