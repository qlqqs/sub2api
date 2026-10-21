//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func mustInsertUsageLog(t *testing.T, ctx context.Context, client *dbent.Client, userID, apiKeyID, accountID int64, actualCost float64, billingType int8, createdAt time.Time) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
INSERT INTO usage_logs (user_id, api_key_id, account_id, model, input_tokens, output_tokens, total_cost, actual_cost, billing_type, created_at)
VALUES ($1, $2, $3, 'gpt-test', 1, 1, $4, $4, $5, $6)`, userID, apiKeyID, accountID, actualCost, billingType, createdAt.UTC())
	require.NoError(t, err)
}

func mustInsertPaymentOrder(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, amount, bonus, refund float64, status string) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
INSERT INTO payment_orders (user_id, amount, pay_amount, bonus_amount, refund_amount, order_type, status, out_trade_no, expires_at, created_at, updated_at)
VALUES ($1, $2, $2, $3, $4, 'balance', $5, $6, NOW() + interval '1 hour', NOW(), NOW())`,
		userID, amount, bonus, refund, status, fmt.Sprintf("aff-agent-%d-%d", userID, time.Now().UnixNano()))
	require.NoError(t, err)
}

func TestAffiliateRepository_AgentLevelDefaultsAndManualLockRules(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateAffiliateWithdrawUser(t, client, "agent-level")
	summary, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, service.AffiliateAgentLevelDefault, summary.AgentLevel)
	require.False(t, summary.AgentLevelLocked)
	require.Equal(t, service.AffiliateAgentLevelSourceDefault, summary.AgentLevelSource)
	require.Nil(t, summary.AgentLevelUpdatedAt)
	require.False(t, summary.RebateSettledAt.IsZero())

	// Manual promotion: not locked.
	require.NoError(t, repo.SetAgentLevelManual(txCtx, []int64{u.ID}, 1))
	summary, err = repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 1, summary.AgentLevel)
	require.False(t, summary.AgentLevelLocked)
	require.Equal(t, service.AffiliateAgentLevelSourceManual, summary.AgentLevelSource)
	require.NotNil(t, summary.AgentLevelUpdatedAt)

	// Manual demotion: locked, auto promotion refused.
	require.NoError(t, repo.SetAgentLevelManual(txCtx, []int64{u.ID}, 3))
	summary, err = repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 3, summary.AgentLevel)
	require.True(t, summary.AgentLevelLocked)
	promoted, err := repo.PromoteAgentLevelAuto(txCtx, u.ID, 2)
	require.NoError(t, err)
	require.False(t, promoted)

	// Unlock then auto promote; auto never demotes.
	require.NoError(t, repo.SetAgentLevelLocked(txCtx, []int64{u.ID}, false))
	promoted, err = repo.PromoteAgentLevelAuto(txCtx, u.ID, 2)
	require.NoError(t, err)
	require.True(t, promoted)
	promoted, err = repo.PromoteAgentLevelAuto(txCtx, u.ID, 3)
	require.NoError(t, err)
	require.False(t, promoted)
	summary, err = repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 2, summary.AgentLevel)
	require.Equal(t, service.AffiliateAgentLevelSourceAuto, summary.AgentLevelSource)

	// Non-default level shows up in the custom-user list and the overview.
	entries, _, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{Search: u.Email, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, 2, entries[0].AgentLevel)
	require.Equal(t, service.AffiliateAgentLevelSourceAuto, entries[0].AgentLevelSource)
	overview, err := repo.GetAffiliateUserOverview(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 2, overview.AgentLevel)
}

func TestAffiliateRepository_CountQualifiedInviteesAndListInviterIDs(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateAffiliateWithdrawUser(t, client, "count-inviter")
	paid := mustCreateAffiliateWithdrawUser(t, client, "count-paid")
	bonusOnly := mustCreateAffiliateWithdrawUser(t, client, "count-bonus")
	free := mustCreateAffiliateWithdrawUser(t, client, "count-free")
	for _, invitee := range []*service.User{paid, bonusOnly, free} {
		_, bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID, service.AffiliateInviterBinding{Source: "registration"})
		require.NoError(t, err)
		require.True(t, bound)
	}
	mustInsertPaymentOrder(t, txCtx, client, paid.ID, 10, 0, 0, "COMPLETED")
	mustInsertPaymentOrder(t, txCtx, client, bonusOnly.ID, 5, 5, 0, "COMPLETED")
	mustInsertPaymentOrder(t, txCtx, client, free.ID, 10, 0, 0, "PENDING")

	all, err := repo.CountQualifiedInvitees(txCtx, inviter.ID, false)
	require.NoError(t, err)
	require.Equal(t, 3, all)
	recharged, err := repo.CountQualifiedInvitees(txCtx, inviter.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, recharged)

	ids, err := repo.ListInviterIDs(txCtx, inviter.ID-1, 10)
	require.NoError(t, err)
	require.Contains(t, ids, inviter.ID)
	ids, err = repo.ListInviterIDs(txCtx, inviter.ID, 10)
	require.NoError(t, err)
	require.NotContains(t, ids, inviter.ID)
}

func TestAffiliateRepository_ConsumptionSettlementWatermarkAndCap(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateAffiliateWithdrawUser(t, client, "settle-inviter")
	consumer := mustCreateAffiliateWithdrawUser(t, client, "settle-consumer")
	_, bound, err := repo.BindInviter(txCtx, consumer.ID, inviter.ID, service.AffiliateInviterBinding{Source: "registration"})
	require.NoError(t, err)
	require.True(t, bound)
	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("settle-%d", time.Now().UnixNano())})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: consumer.ID, Name: "settle", Key: fmt.Sprintf("sk-settle-%d", time.Now().UnixNano())})

	// Watermark starts at profile creation, so push it back to include our rows.
	_, err = client.ExecContext(txCtx, "UPDATE user_affiliates SET rebate_settled_at = NOW() - interval '1 day' WHERE user_id = $1", consumer.ID)
	require.NoError(t, err)

	now := time.Now()
	mustInsertUsageLog(t, txCtx, client, consumer.ID, apiKey.ID, account.ID, 30, service.BillingTypeBalance, now.Add(-2*time.Hour))
	mustInsertUsageLog(t, txCtx, client, consumer.ID, apiKey.ID, account.ID, 20, service.BillingTypeBalance, now.Add(-time.Hour))
	mustInsertUsageLog(t, txCtx, client, consumer.ID, apiKey.ID, account.ID, 999, service.BillingTypeSubscription, now.Add(-time.Hour))
	// Inside the lag window: must not be counted yet.
	mustInsertUsageLog(t, txCtx, client, consumer.ID, apiKey.ID, account.ID, 7, service.BillingTypeBalance, now.Add(-10*time.Second))
	// Real recharge: 100 credited incl. 20 bonus, plus a fully refunded 50 and a pending refund request that still counts.
	mustInsertPaymentOrder(t, txCtx, client, consumer.ID, 100, 20, 0, "COMPLETED")
	mustInsertPaymentOrder(t, txCtx, client, consumer.ID, 50, 0, 50, "REFUNDED")
	mustInsertPaymentOrder(t, txCtx, client, consumer.ID, 10, 0, 10, "REFUND_REQUESTED")

	ids, err := repo.ListUsersWithPendingConsumption(txCtx, time.Minute, 100)
	require.NoError(t, err)
	require.Contains(t, ids, consumer.ID)
	require.NotContains(t, ids, inviter.ID)

	pending, err := repo.LockPendingConsumption(txCtx, consumer.ID, time.Minute)
	require.NoError(t, err)
	require.InDelta(t, 50, pending.DeltaCost, 1e-6)
	require.InDelta(t, 90, pending.PaidTotal, 1e-6)
	require.Zero(t, pending.SettledConsumption)
	require.True(t, pending.UntilAt.After(pending.SettledAt))

	until := pending.UntilAt
	applied, err := repo.AccrueQuota(txCtx, inviter.ID, consumer.ID, 4, 0, service.AffiliateAccrueSource{UsageUntil: &until}, 1)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, repo.AdvanceConsumptionWatermark(txCtx, consumer.ID, until, 50))

	// Second pass sees nothing new (the 7 is still inside the lag window).
	pending, err = repo.LockPendingConsumption(txCtx, consumer.ID, time.Minute)
	require.NoError(t, err)
	require.Zero(t, pending.DeltaCost)
	require.InDelta(t, 50, pending.SettledConsumption, 1e-6)
	ids, err = repo.ListUsersWithPendingConsumption(txCtx, time.Minute, 100)
	require.NoError(t, err)
	require.NotContains(t, ids, consumer.ID)

	// With zero lag the in-window row becomes visible.
	pending, err = repo.LockPendingConsumption(txCtx, consumer.ID, 0)
	require.NoError(t, err)
	require.InDelta(t, 7, pending.DeltaCost, 1e-6)

	rows, err := client.QueryContext(txCtx, "SELECT source_usage_until, source_order_id FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'accrue'", inviter.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var storedUntil time.Time
	var orderID *int64
	require.NoError(t, rows.Scan(&storedUntil, &orderID))
	require.WithinDuration(t, until, storedUntil, time.Millisecond)
	require.Nil(t, orderID)
	require.NoError(t, rows.Close())

	_, err = repo.LockPendingConsumption(txCtx, 0, time.Minute)
	require.ErrorIs(t, err, service.ErrUserNotFound)
}
