//go:build unit

package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSignupBonusSettingAudit(t *testing.T) {
	before := &service.SystemSettings{}
	after := &service.SystemSettings{AffiliateInviteeSignupBonus: 5}
	changed := diffSettings(before, after, nil, nil, UpdateSettingsRequest{})
	require.Equal(t, []string{"affiliate_invitee_signup_bonus"}, changed)
	require.Empty(t, diffSettings(after, after, nil, nil, UpdateSettingsRequest{}))
}

func TestSignupSourceSettingRoundTripAndPartialUpdate(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyAffiliateInviteeSignupBonusSource: "agent",
	})
	rec := doUpdateSettings(t, h, map[string]any{"affiliate_invitee_signup_bonus": 10}, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "agent", repo.values[service.SettingKeyAffiliateInviteeSignupBonusSource])
	require.Contains(t, rec.Body.String(), `"affiliate_invitee_signup_bonus_source":"agent"`)
	for _, source := range []string{"user", "all", "agent"} {
		rec = doUpdateSettings(t, h, map[string]any{"affiliate_invitee_signup_bonus_source": source}, nil)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, source, repo.values[service.SettingKeyAffiliateInviteeSignupBonusSource])
	}
	for _, source := range []string{"", "invalid"} {
		rec = doUpdateSettings(t, h, map[string]any{"affiliate_invitee_signup_bonus_source": source}, nil)
		require.Equal(t, 400, rec.Code)
		require.Equal(t, "agent", repo.values[service.SettingKeyAffiliateInviteeSignupBonusSource])
	}
	before := &service.SystemSettings{AffiliateInviteeSignupBonusSource: "all"}
	after := &service.SystemSettings{AffiliateInviteeSignupBonusSource: "agent"}
	require.Equal(t, []string{"affiliate_invitee_signup_bonus_source"}, diffSettings(before, after, nil, nil, UpdateSettingsRequest{}))
}
