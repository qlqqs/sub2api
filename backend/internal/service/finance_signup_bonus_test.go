//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFinanceInviteeSignupBonusIsIndependentGift(t *testing.T) {
	at, err := time.ParseInLocation("2006-01-02", "2026-10-27", AffiliateTeamLocation)
	require.NoError(t, err)
	span := FinanceRange{Start: "2026-10-27", End: "2026-10-27", Granularity: "day"}
	data := &FinanceData{
		Users: []FinanceUser{{ID: 1, Balance: fd("8")}},
		Ledger: []FinanceLedger{
			{ID: 1, UserID: 1, At: at, Action: AffiliateRewardInviteeSignupBonus, RewardType: AffiliateRewardInviteeSignupBonus, Amount: fd("5")},
			{ID: 2, UserID: 1, At: at, Action: AffiliateRewardInviteeBonus, RewardType: AffiliateRewardInviteeBonus, Amount: fd("3")},
			// 防御历史异常 action：即使标为 accrue，赠送也不得计分润。
			{ID: 3, UserID: 1, At: at, Action: "accrue", RewardType: AffiliateRewardInviteeSignupBonus, Amount: fd("99")},
		},
	}
	replay := financeReplays(data)[1]
	require.Equal(t, "8", replay.Free.String())
	require.True(t, replay.Paid.IsZero())
	require.True(t, replay.Commission.IsZero())
	require.True(t, replay.Difference.IsZero())
	svc := NewFinanceService(&financeBreakdownRepo{data: data})
	recharge, err := svc.Recharge(context.Background(), span)
	require.NoError(t, err)
	gift := recharge["totals"].(map[string]any)["gift"].(financeAmounts)
	require.Equal(t, "5", gift["invitee_signup_bonus"].String())
	require.Equal(t, "3", gift["invitee_bonus"].String())
	require.Equal(t, "8", gift["total"].String())
	commissions, err := svc.Affiliate(context.Background(), span)
	require.NoError(t, err)
	require.Equal(t, "0", commissions["accrued"].(financeAmounts)["total"].String())
	summary := financeSummary(data, span)
	require.Equal(t, "0", summary["totals"].(map[string]any)["commission"])
}
