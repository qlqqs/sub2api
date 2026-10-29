//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignupSourceMatrix(t *testing.T) {
	for _, source := range []string{"all", "agent", "user"} {
		for _, level := range []int{AffiliateAgentLevelDefault, AffiliateAgentLevelTop, AffiliateAgentLevel2} {
			t.Run(fmt.Sprintf("%s/level%d", source, level), func(t *testing.T) {
				ctx := context.Background()
				repo := newFakeAffiliateRepo()
				repo.row(1).AgentLevel = level
				svc, settings := newTwoTierService(repo, map[string]string{
					SettingKeyAffiliateInviteeSignupBonus:       "5",
					SettingKeyAffiliateInviteeSignupBonusSource: source,
				})
				want := 5.0
				if source == "agent" && level == AffiliateAgentLevelDefault || source == "user" && level != AffiliateAgentLevelDefault {
					want = 0
				}
				amount, err := svc.PreviewInviteeSignupBonus(ctx, " code1 ")
				require.NoError(t, err)
				require.Equal(t, want, amount)
				require.Len(t, repo.rows, 1, "preview must not create an affiliate profile")
				require.Empty(t, repo.ledger)
				detail, err := svc.GetAffiliateDetail(ctx, 1)
				require.NoError(t, err)
				require.Equal(t, want, detail.InviteeSignupBonus)
				bind(t, svc, 2, 1)
				require.Equal(t, want, repo.balances[2])
				settings.values[SettingKeyAffiliateInviteeSignupBonus] = "10"
				amount, err = svc.PreviewInviteeSignupBonus(ctx, "CODE1")
				require.NoError(t, err)
				require.Equal(t, want*2, amount)
				bind(t, svc, 2, 1)
				require.Equal(t, want, repo.balances[2], "old registration is never re-credited")
				detail, err = svc.GetAffiliateDetail(ctx, 2)
				require.NoError(t, err)
				require.Equal(t, want, detail.RewardTotals.InviteeSignupBonus)
				bind(t, svc, 3, 1)
				require.Equal(t, want*2, repo.balances[3])
			})
		}
	}
}

func TestSignupSourceUsesDirectInviterSnapshot(t *testing.T) {
	for _, source := range []string{"agent", "user"} {
		t.Run(source, func(t *testing.T) {
			repo := newFakeAffiliateRepo()
			repo.row(10).AgentLevel = AffiliateAgentLevelTop
			owner := int64(10)
			repo.row(1).AgentOwnerID = &owner
			repo.inviteeCount = 5
			svc, _ := newTwoTierService(repo, map[string]string{
				SettingKeyAffiliateInviteeSignupBonus: "5", SettingKeyAffiliateInviteeSignupBonusSource: source,
				SettingKeyAffiliateAgentUpgradeMode: AffiliateAgentUpgradeModeAuto, SettingKeyAffiliateAgentLevel2Threshold: "5",
			})
			amount, err := svc.PreviewInviteeSignupBonus(context.Background(), "CODE1")
			require.NoError(t, err)
			want := 0.0
			if source == "user" {
				want = 5
			}
			require.Equal(t, want, amount)
			bind(t, svc, 2, 1)
			require.Equal(t, want, repo.balances[2])
			require.Equal(t, owner, *repo.row(2).AgentOwnerID)
			require.False(t, repo.row(2).InviterWasAgent)
			require.Equal(t, AffiliateAgentLevel2, repo.row(1).AgentLevel)
		})
	}
}

func TestSignupSourcePreviewGuards(t *testing.T) {
	repo := newFakeAffiliateRepo()
	repo.row(1)
	svc, settings := newTwoTierService(repo, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
	for _, code := range []string{"", "!", "MISSING"} {
		amount, err := svc.PreviewInviteeSignupBonus(context.Background(), code)
		require.NoError(t, err)
		require.Zero(t, amount)
	}
	repo.deleted[1] = true
	amount, err := svc.PreviewInviteeSignupBonus(context.Background(), "CODE1")
	require.NoError(t, err)
	require.Zero(t, amount)
	repo.deleted[1] = false
	settings.values[SettingKeyAffiliateEnabled] = "false"
	amount, err = svc.PreviewInviteeSignupBonus(context.Background(), "CODE1")
	require.NoError(t, err)
	require.Zero(t, amount)
	require.Empty(t, repo.ledger)
}

type signupPreviewFailRepo struct{ *fakeAffiliateRepo }

func (r *signupPreviewFailRepo) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	return nil, errors.New("unavailable")
}
func TestSignupSourcePreviewFailure(t *testing.T) {
	repo := newFakeAffiliateRepo()
	svc, _ := newTwoTierService(repo, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5"})
	svc.repo = &signupPreviewFailRepo{repo}
	amount, err := svc.PreviewInviteeSignupBonus(context.Background(), "CODE1")
	require.Error(t, err)
	require.Zero(t, amount)
}

func TestSignupSourceConfig(t *testing.T) {
	ctx := context.Background()
	svc := newSettingServiceForPlatformThresholdTest(nil)
	require.Equal(t, AffiliateSignupSourceAll, parseAffiliateRewardConfig(nil).InviteeSignupBonusSource)
	for _, source := range []string{"all", "agent", "user"} {
		settings := &SystemSettings{AffiliateInviteeSignupBonusSource: source}
		updates, err := svc.buildSystemSettingsUpdates(ctx, settings)
		require.NoError(t, err)
		require.Equal(t, source, updates[SettingKeyAffiliateInviteeSignupBonusSource])
		var out SystemSettings
		parseAffiliateRewardConfig(updates).applyToSettings(&out)
		require.Equal(t, source, out.AffiliateInviteeSignupBonusSource)
	}
	_, err := svc.buildSystemSettingsUpdates(ctx, &SystemSettings{AffiliateInviteeSignupBonusSource: "invalid"})
	require.Error(t, err)
}

func TestSignupSourceOAuth(t *testing.T) {
	for _, source := range []string{"agent", "user"} {
		t.Run(source, func(t *testing.T) {
			repo := newFakeAffiliateRepo()
			repo.row(1).AgentLevel = AffiliateAgentLevelTop
			affiliate, _ := newTwoTierService(repo, map[string]string{SettingKeyAffiliateInviteeSignupBonus: "5", SettingKeyAffiliateInviteeSignupBonusSource: source})
			auth := newEmailOAuthAutoAuthService(&userRepoStub{nextID: 88}, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil)
			auth.affiliateService = affiliate
			u, err := auth.createEmailOAuthUser(context.Background(), "source@example.com", "source", "github", "", "CODE1")
			require.NoError(t, err)
			want := 0.0
			if source == "agent" {
				want = 5
			}
			require.Equal(t, want, repo.balances[u.ID])
		})
	}
}
