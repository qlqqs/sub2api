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
