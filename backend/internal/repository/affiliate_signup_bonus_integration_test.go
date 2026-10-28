//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateSignupBonusMigrationAndIndependentRewards(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)
	u := mustCreateAffiliateWithdrawUser(t, client, "signup-bonus")
	// 模拟非零默认余额，注册赠送必须做增量而非覆盖。
	require.NoError(t, client.User.UpdateOneID(u.ID).SetBalance(7).Exec(txCtx))
	before := querySingleFloat(t, txCtx, client, "SELECT balance::double precision FROM users WHERE id=$1", u.ID)
	require.Equal(t, 1, querySingleInt(t, txCtx, client, `SELECT count(*) FROM pg_indexes WHERE indexname='idx_user_affiliate_ledger_signup_bonus_once' AND indexdef LIKE '%UNIQUE%' AND indexdef LIKE '%invitee_signup_bonus%'`))
	for i := 0; i < 2; i++ {
		applied, err := repo.CreditInviteeSignupBonus(txCtx, u.ID, 5)
		require.NoError(t, err)
		require.Equal(t, i == 0, applied)
		applied, err = repo.CreditInviteeBonus(txCtx, u.ID, 3, nil)
		require.NoError(t, err)
		require.Equal(t, i == 0, applied)
	}
	require.InDelta(t, before+8, querySingleFloat(t, txCtx, client, "SELECT balance::double precision FROM users WHERE id=$1", u.ID), 1e-9)
	require.Equal(t, 1, querySingleInt(t, txCtx, client, `SELECT count(*) FROM user_affiliate_ledger WHERE user_id=$1 AND source_user_id=$1 AND action='invitee_signup_bonus' AND reward_type='invitee_signup_bonus' AND source_order_id IS NULL`, u.ID))
	totals, err := repo.SumRewardTotals(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 5.0, totals.InviteeSignupBonus)
	require.Equal(t, 3.0, totals.InviteeBonus)
	require.Zero(t, totals.Agent)
	require.Zero(t, totals.Invite)
	has, err := repo.HasRewardForSource(txCtx, u.ID, service.AffiliateRewardInviteeSignupBonus)
	require.NoError(t, err)
	require.True(t, has)
	records, total, err := repo.ListAffiliateRebateRecords(txCtx, service.AffiliateRecordFilter{Search: u.Email, Page: 1, PageSize: 20, RewardType: service.AffiliateRewardInviteeSignupBonus})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, u.ID, records[0].InviterID)
	require.Equal(t, u.ID, *records[0].InviteeID)
	gifted, err := NewRedeemCodeRepository(client).GetBalanceHistoryTotals(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 8.0, gifted.TotalGifted)
	require.Zero(t, gifted.TotalRecharged)
	// 管理端余额历史走同一事务客户端，覆盖全部/独立筛选及实收赠送拆分。
	admin := service.NewAdminService(nil, nil, nil, nil, nil, nil, NewRedeemCodeRepository(client), nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, filter := range []string{"", service.AffiliateRewardInviteeSignupBonus} {
		codes, count, historyTotals, err := admin.GetUserBalanceHistory(txCtx, u.ID, 1, 20, filter)
		require.NoError(t, err)
		if filter == "" {
			require.EqualValues(t, 2, count)
		} else {
			require.EqualValues(t, 1, count)
		}
		found := false
		for _, code := range codes {
			if code.Type == service.AffiliateRewardInviteeSignupBonus {
				require.Equal(t, 5.0, code.Value)
				found = true
			}
		}
		require.True(t, found)
		require.Equal(t, 8.0, historyTotals.TotalGifted)
		require.Zero(t, historyTotals.TotalRecharged)
	}
	codes, count, _, err := admin.GetUserBalanceHistory(txCtx, u.ID, 1, 20, service.RedeemTypeAffiliateBalance)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.Equal(t, service.RedeemTypeAffiliateBalance, codes[0].Type)
	// 同一赠送用户的真实充值上限仍由订单决定，两个赠送均不增加上限。
	_, err = repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	pending, err := repo.LockPendingConsumption(txCtx, u.ID, time.Second)
	require.NoError(t, err)
	require.Zero(t, pending.PaidTotal)
	mustInsertPaymentOrder(t, txCtx, client, u.ID, 20, 0, 0, "COMPLETED")
	pending, err = repo.LockPendingConsumption(txCtx, u.ID, time.Second)
	require.NoError(t, err)
	require.Equal(t, 20.0, pending.PaidTotal)
}

func TestAffiliateSignupBonusConcurrentReplayAndRollback(t *testing.T) {
	ctx := context.Background()
	// 并发需要真实提交的用户，不使用只对单事务可见的 testEntTx。
	u := mustCreateAffiliateWithdrawUser(t, integrationEntClient, fmt.Sprintf("signup-race-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		_, _ = integrationEntClient.ExecContext(ctx, "DELETE FROM user_affiliate_ledger WHERE user_id=$1", u.ID)
		_, _ = integrationEntClient.ExecContext(ctx, "DELETE FROM users WHERE id=$1", u.ID)
	})
	repo := NewAffiliateRepository(integrationEntClient, integrationDB)
	before := querySingleFloat(t, ctx, integrationEntClient, "SELECT balance::double precision FROM users WHERE id=$1", u.ID)
	var appliedCount atomic.Int32
	var wg sync.WaitGroup
	errors := make(chan error, 12)
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			applied, err := repo.CreditInviteeSignupBonus(ctx, u.ID, 5)
			if err != nil {
				errors <- err
				return
			}
			if applied {
				appliedCount.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, appliedCount.Load())
	require.InDelta(t, before+5, querySingleFloat(t, ctx, integrationEntClient, "SELECT balance::double precision FROM users WHERE id=$1", u.ID), 1e-9)
	require.Equal(t, 1, querySingleInt(t, ctx, integrationEntClient, `SELECT count(*) FROM user_affiliate_ledger WHERE user_id=$1 AND reward_type='invitee_signup_bonus'`, u.ID))

	other := mustCreateAffiliateWithdrawUser(t, integrationEntClient, "signup-rollback")
	t.Cleanup(func() { _, _ = integrationEntClient.ExecContext(ctx, "DELETE FROM users WHERE id=$1", other.ID) })
	tx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := dbent.NewTxContext(ctx, tx)
	applied, err := repo.CreditInviteeSignupBonus(txCtx, other.ID, 5)
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, 1, querySingleInt(t, txCtx, tx.Client(), `SELECT count(*) FROM user_affiliate_ledger WHERE user_id=$1`, other.ID))
	require.NoError(t, tx.Rollback())
	require.Equal(t, 0, querySingleInt(t, ctx, integrationEntClient, `SELECT count(*) FROM user_affiliate_ledger WHERE user_id=$1`, other.ID))
	require.Equal(t, other.Balance, querySingleFloat(t, ctx, integrationEntClient, "SELECT balance::double precision FROM users WHERE id=$1", other.ID))
	applied, err = repo.CreditInviteeSignupBonus(ctx, other.ID, 5)
	require.NoError(t, err)
	require.True(t, applied)
	_, err = integrationEntClient.ExecContext(ctx, "DELETE FROM user_affiliate_ledger WHERE user_id=$1", other.ID)
	require.NoError(t, err)
}

func TestAffiliateSignupBonusBalanceFailureRollsBackLedger(t *testing.T) {
	ctx := context.Background()
	u := mustCreateAffiliateWithdrawUser(t, integrationEntClient, "signup-balance-failure")
	t.Cleanup(func() { _, _ = integrationEntClient.ExecContext(ctx, "DELETE FROM users WHERE id=$1", u.ID) })
	tx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := dbent.NewTxContext(ctx, tx)
	// 临时触发器只拒绝本用例用户，且随事务回滚，不影响其它用例或容器。
	_, err = tx.Client().ExecContext(txCtx, fmt.Sprintf(`
CREATE FUNCTION pg_temp.reject_signup_balance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '模拟余额更新失败'; END $$;
CREATE TRIGGER reject_signup_balance BEFORE UPDATE OF balance ON users
FOR EACH ROW WHEN (NEW.id = %d) EXECUTE FUNCTION pg_temp.reject_signup_balance()`, u.ID))
	require.NoError(t, err)
	repo := NewAffiliateRepository(integrationEntClient, integrationDB)
	applied, err := repo.CreditInviteeSignupBonus(txCtx, u.ID, 5)
	require.Error(t, err)
	require.False(t, applied)
	// 保存点已恢复外层事务，继续写入并提交，注册主体不受失败赠送影响。
	require.Equal(t, 0, querySingleInt(t, txCtx, tx.Client(), "SELECT count(*) FROM user_affiliate_ledger WHERE user_id=$1", u.ID))
	_, err = tx.Client().ExecContext(txCtx, "DROP TRIGGER reject_signup_balance ON users")
	require.NoError(t, err)
	require.NoError(t, tx.Client().User.UpdateOneID(u.ID).SetUsername("注册仍可提交").Exec(txCtx))
	require.NoError(t, tx.Commit())
	require.Equal(t, 0, querySingleInt(t, ctx, integrationEntClient, "SELECT count(*) FROM user_affiliate_ledger WHERE user_id=$1", u.ID))
	persisted, err := integrationEntClient.User.Get(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "注册仍可提交", persisted.Username)
	require.Equal(t, u.Balance, querySingleFloat(t, ctx, integrationEntClient, "SELECT balance::double precision FROM users WHERE id=$1", u.ID))
}
