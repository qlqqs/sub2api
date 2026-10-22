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
	require.NoError(t, repo.SetAgentLevelManual(txCtx, []int64{u.ID}, 1, 0))
	summary, err = repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 1, summary.AgentLevel)
	require.False(t, summary.AgentLevelLocked)
	require.Equal(t, service.AffiliateAgentLevelSourceManual, summary.AgentLevelSource)
	require.NotNil(t, summary.AgentLevelUpdatedAt)

	// Manual demotion: locked, auto promotion refused.
	require.NoError(t, repo.SetAgentLevelManual(txCtx, []int64{u.ID}, 3, 0))
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

	// 已删除的邀请人不再参与重新评估。
	_, err = client.ExecContext(txCtx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", inviter.ID)
	require.NoError(t, err)
	ids, err = repo.ListInviterIDs(txCtx, inviter.ID-1, 10)
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
	applied, err := repo.AccrueQuota(txCtx, inviter.ID, consumer.ID, 4, 0, service.AffiliateAccrueSource{UsageUntil: &until}, service.AffiliateRewardAgentDirect)
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

// TestAffiliateRepository_BindInviterWritesOwnerSnapshotAndResetsWatermark 验证绑定时写入
// 分润结构快照并把消费水位重置为绑定时刻（补绑老用户时不结算绑定前的消费）。
func TestAffiliateRepository_BindInviterWritesOwnerSnapshotAndResetsWatermark(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	top := mustCreateAffiliateWithdrawUser(t, client, "snap-top")
	agent := mustCreateAffiliateWithdrawUser(t, client, "snap-agent")
	customer := mustCreateAffiliateWithdrawUser(t, client, "snap-customer")
	_, err := repo.EnsureUserAffiliate(txCtx, customer.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(txCtx, "UPDATE user_affiliates SET rebate_settled_at = NOW() - interval '30 days' WHERE user_id = $1", customer.ID)
	require.NoError(t, err)

	level := 2
	_, bound, err := repo.BindInviter(txCtx, customer.ID, agent.ID, service.AffiliateInviterBinding{
		Source: "registration",
		Owner:  service.AffiliateOwnerSnapshot{OwnerID: &agent.ID, OwnerLevel: &level, UplineID: &top.ID},
	})
	require.NoError(t, err)
	require.True(t, bound)

	summary, err := repo.EnsureUserAffiliate(txCtx, customer.ID)
	require.NoError(t, err)
	require.Equal(t, agent.ID, *summary.AgentOwnerID)
	require.Equal(t, 2, *summary.AgentOwnerLevel)
	require.Equal(t, top.ID, *summary.AgentUplineID)
	require.NotNil(t, summary.AgentOwnerBoundAt)
	require.WithinDuration(t, time.Now(), summary.RebateSettledAt, time.Minute)

	count, err := repo.CountOwnedCustomers(txCtx, agent.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// 代理管理：按归属代理筛选，带出快照邮箱；客户列表区分 owner / upline。
	entries, total, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{OwnerID: agent.ID, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, customer.ID, entries[0].UserID)
	require.Equal(t, agent.Email, entries[0].AgentOwnerEmail)
	require.Equal(t, top.Email, entries[0].AgentUplineEmail)
	require.False(t, entries[0].AgentOwnerDeleted)

	customers, total, err := repo.ListAgentCustomers(txCtx, top.ID, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "upline", customers[0].Relation)
	customers, _, err = repo.ListAgentCustomers(txCtx, agent.ID, 1, 10)
	require.NoError(t, err)
	require.Equal(t, "owner", customers[0].Relation)

	// 邀请记录带出邀请人与被邀请人当前的代理等级。
	_, err = client.ExecContext(txCtx, "UPDATE user_affiliates SET agent_level = 2 WHERE user_id = $1", agent.ID)
	require.NoError(t, err)
	invites, _, err := repo.ListAffiliateInviteRecords(txCtx, service.AffiliateRecordFilter{Search: customer.Email, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, invites, 1)
	require.Equal(t, 2, invites[0].InviterLevel)
	require.Equal(t, 3, invites[0].InviteeLevel)

	// 软删除上方一级：快照 ID 保留，节点标记为不活跃，列表显示已删除。
	_, err = client.ExecContext(txCtx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", top.ID)
	require.NoError(t, err)
	nodes, err := repo.GetAffiliateNodes(txCtx, []int64{agent.ID, top.ID})
	require.NoError(t, err)
	require.True(t, nodes[agent.ID].Active)
	require.False(t, nodes[top.ID].Active)
	overview, err := repo.GetAffiliateUserOverview(txCtx, customer.ID)
	require.NoError(t, err)
	require.Equal(t, top.ID, *overview.AgentUplineID)
	require.True(t, overview.AgentUplineDeleted)
}

func TestAffiliateRepository_AgentAppointmentsAndLevelFilter(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	appointer := mustCreateAffiliateWithdrawUser(t, client, "appointer")
	target := mustCreateAffiliateWithdrawUser(t, client, "appointee")
	_, err := repo.GetAffiliateForUpdate(txCtx, target.ID)
	require.NoError(t, err)

	require.NoError(t, repo.SetAgentLevelByAgent(txCtx, target.ID, 2, appointer.ID, false))
	n, err := repo.CountAgentAppointments(txCtx, appointer.ID)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	entries, _, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{Search: target.Email, AgentLevel: 2, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, service.AffiliateAgentLevelSourceAgent, entries[0].AgentLevelSource)
	require.Equal(t, appointer.Email, entries[0].AgentLevelUpdatedByEmail)

	// 管理员覆盖：来源变为 manual，任命计数归零。
	require.NoError(t, repo.SetAgentLevelManual(txCtx, []int64{target.ID}, 2, appointer.ID+1000))
	n, err = repo.CountAgentAppointments(txCtx, appointer.ID)
	require.NoError(t, err)
	require.Zero(t, n)

	require.NoError(t, repo.SetAgentLevelByAgent(txCtx, target.ID, 3, appointer.ID, true))
	summary, err := repo.EnsureUserAffiliate(txCtx, target.ID)
	require.NoError(t, err)
	require.Equal(t, 3, summary.AgentLevel)
	require.True(t, summary.AgentLevelLocked)
	require.Equal(t, appointer.ID, *summary.AgentLevelUpdatedBy)
}

func TestAffiliateRepository_SetAgentOwnerByAdminAndInviterWasAgent(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	top := mustCreateAffiliateWithdrawUser(t, client, "owner-top")
	admin := mustCreateAffiliateWithdrawUser(t, client, "owner-admin")
	agent := mustCreateAffiliateWithdrawUser(t, client, "owner-agent")
	invited := mustCreateAffiliateWithdrawUser(t, client, "owner-invited")
	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("owner-%d", time.Now().UnixNano())})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: agent.ID, Name: "owner", Key: fmt.Sprintf("sk-owner-%d", time.Now().UnixNano())})

	// 绑定写入 inviter_was_agent。
	level := 1
	_, bound, err := repo.BindInviter(txCtx, invited.ID, top.ID, service.AffiliateInviterBinding{
		Source: "registration", InviterWasAgent: true,
		Owner: service.AffiliateOwnerSnapshot{OwnerID: &top.ID, OwnerLevel: &level},
	})
	require.NoError(t, err)
	require.True(t, bound)
	summary, err := repo.EnsureUserAffiliate(txCtx, invited.ID)
	require.NoError(t, err)
	require.True(t, summary.InviterWasAgent)
	require.Nil(t, summary.AgentOwnerSetBy)

	// agent 没有邀请人也没有归属：不在待结算列表里，水位停在很久以前。
	_, err = repo.EnsureUserAffiliate(txCtx, agent.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(txCtx, "UPDATE user_affiliates SET rebate_settled_at = NOW() - interval '30 days' WHERE user_id = $1", agent.ID)
	require.NoError(t, err)
	mustInsertUsageLog(t, txCtx, client, agent.ID, apiKey.ID, account.ID, 10, service.BillingTypeBalance, time.Now().Add(-2*time.Hour))
	pending, err := repo.ListUsersWithPendingConsumption(txCtx, time.Minute, 100)
	require.NoError(t, err)
	require.NotContains(t, pending, agent.ID)

	// 设置上级：写快照与操作人，水位移到当前（不补发设置前的消费），不改邀请关系。
	require.NoError(t, repo.SetAgentOwnerByAdmin(txCtx, agent.ID, &top.ID, admin.ID))
	summary, err = repo.EnsureUserAffiliate(txCtx, agent.ID)
	require.NoError(t, err)
	require.Nil(t, summary.InviterID)
	require.Equal(t, top.ID, *summary.AgentOwnerID)
	require.Equal(t, 1, *summary.AgentOwnerLevel)
	require.Nil(t, summary.AgentUplineID)
	require.Equal(t, admin.ID, *summary.AgentOwnerSetBy)
	require.NotNil(t, summary.AgentOwnerBoundAt)
	require.WithinDuration(t, time.Now(), summary.RebateSettledAt, time.Minute)
	pending, err = repo.ListUsersWithPendingConsumption(txCtx, time.Minute, 100)
	require.NoError(t, err)
	require.NotContains(t, pending, agent.ID, "consumption before the owner was set is not settled")

	// 已有归属时更换不移动水位；之后的消费会进入待结算。
	_, err = client.ExecContext(txCtx, "UPDATE user_affiliates SET rebate_settled_at = NOW() - interval '1 day' WHERE user_id = $1", agent.ID)
	require.NoError(t, err)
	require.NoError(t, repo.SetAgentOwnerByAdmin(txCtx, agent.ID, &top.ID, admin.ID))
	pending, err = repo.ListUsersWithPendingConsumption(txCtx, time.Minute, 100)
	require.NoError(t, err)
	require.Contains(t, pending, agent.ID)

	entries, _, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{OwnerID: top.ID, Page: 1, PageSize: 10})
	require.NoError(t, err)
	var found bool
	for _, e := range entries {
		if e.UserID == agent.ID {
			found = true
			require.Equal(t, admin.Email, e.AgentOwnerSetByEmail)
			require.NotNil(t, e.AgentOwnerBoundAt)
		}
	}
	require.True(t, found)

	// 清除：快照清空。
	require.NoError(t, repo.SetAgentOwnerByAdmin(txCtx, agent.ID, nil, admin.ID))
	summary, err = repo.EnsureUserAffiliate(txCtx, agent.ID)
	require.NoError(t, err)
	require.Nil(t, summary.AgentOwnerID)
	require.Nil(t, summary.AgentOwnerLevel)
	require.Nil(t, summary.AgentOwnerBoundAt)
}
