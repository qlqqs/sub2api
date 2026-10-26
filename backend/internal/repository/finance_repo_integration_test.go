//go:build integration

package repository

// CUSTOM(fork): testcontainers PostgreSQL 上验证实际迁移、decimal 取数和事务回滚。
import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestAccountModelCostRepository(t *testing.T) {
	ctx := context.Background()
	a := mustCreateAccount(t, testEntClient(t), &service.Account{Name: fmt.Sprintf("finance-cost-%d", time.Now().UnixNano())})
	r := NewAccountModelCostRepository(integrationDB)
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, a.ID) })
	require.NoError(t, r.ReplaceForAccount(ctx, a.ID, []service.AccountModelCostRule{{Model: "zero", RateMultiplier: 0}, {Model: "gpt-*", RateMultiplier: 0.1234}}))
	rules, err := r.ListByAccount(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	require.Equal(t, "gpt-*", rules[0].Model)
	require.Equal(t, 0.1234, rules[0].RateMultiplier)
	err = r.ReplaceForAccount(ctx, a.ID, []service.AccountModelCostRule{{Model: "duplicate", RateMultiplier: 1}, {Model: "duplicate", RateMultiplier: 2}})
	require.Error(t, err)
	rules, err = r.ListByAccount(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, rules, 2, "失败替换回滚，不清空原规则")
	require.Error(t, r.ReplaceForAccount(ctx, a.ID, []service.AccountModelCostRule{{Model: "negative", RateMultiplier: -1}}))
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET deleted_at=NOW() WHERE id=$1`, a.ID)
	require.NoError(t, err)
	all, err := r.ListAll(ctx)
	require.NoError(t, err)
	require.NotContains(t, all, a.ID)
	require.ErrorIs(t, r.ReplaceForAccount(ctx, a.ID, nil), service.ErrAccountNotFound)
}
func TestFinanceRepositorySnapshotExpensesAndReports(t *testing.T) {
	ctx := context.Background()
	c := testEntClient(t)
	suffix := time.Now().UnixNano()
	u := mustCreateUser(t, c, &service.User{Email: fmt.Sprintf("finance-%d@test.local", suffix), Balance: 10})
	a := mustCreateAccount(t, c, &service.Account{Name: fmt.Sprintf("finance-%d", suffix)})
	key := mustCreateApiKey(t, c, &service.APIKey{UserID: u.ID, Name: "finance", Key: fmt.Sprintf("sk-finance-%d", suffix)})
	r := NewFinanceRepository(integrationDB)
	svc := service.NewFinanceService(r)
	at := time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC) // 北京 2 月 1 日
	p, err := c.PaymentProviderInstance.Create().SetProviderKey("easypay").SetConfig("{}").SetChannelFeeRate("0.0320").Save(ctx)
	require.NoError(t, err)
	code := fmt.Sprintf("FIN-%d", suffix)
	o, err := c.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName("").SetAmount(12).SetBonusAmount(2).SetPayAmount(10).SetRechargeCode(code).SetOutTradeNo("S2A-" + code).SetPaymentType("wxpay").SetPaymentTradeNo("").SetClientIP("127.0.0.1").SetSrcHost("localhost").SetStatus("COMPLETED").SetOrderType("balance").SetCompletedAt(at).SetExpiresAt(at.Add(time.Hour)).SetProviderInstanceID(fmt.Sprint(p.ID)).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM finance_expenses WHERE created_by=$1`, `DELETE FROM usage_logs WHERE user_id=$1`, `DELETE FROM user_affiliate_ledger WHERE user_id=$1`, `DELETE FROM redeem_codes WHERE used_by=$1`, `DELETE FROM payment_orders WHERE user_id=$1`, `DELETE FROM api_keys WHERE user_id=$1`, `DELETE FROM users WHERE id=$1`} {
			_, _ = integrationDB.ExecContext(ctx, q, u.ID)
		}
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, a.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM payment_provider_instances WHERE id=$1`, p.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM settings WHERE key=$1`, service.FinanceFixedAccountsKey)
	})
	// 支付兑换码不能重复算免费入账；管理员入账另算。
	_, err = c.RedeemCode.Create().SetCode(code).SetType("balance").SetValue(12).SetStatus("used").SetUsedBy(u.ID).SetUsedAt(at).Save(ctx)
	require.NoError(t, err)
	_, err = c.RedeemCode.Create().SetCode(fmt.Sprintf("ADM-%d", suffix)).SetType("admin_balance").SetValue(3).SetStatus("used").SetUsedBy(u.ID).SetUsedAt(at).Save(ctx)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,requested_model,input_tokens,output_tokens,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,billing_type,created_at,image_count) VALUES($1,$2,$3,'upstream','requested',1,1,100,5,0.1,0.3,0,$4,1),($1,$2,$3,'upstream','requested',1,1,100,2,0.2,0.5,1,$4,0)`, u.ID, key.ID, a.ID, at)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount,reward_type,created_at) VALUES($1,'accrue',0.5,'agent_direct',$2),($1,'transfer',1,NULL,$2)`, u.ID, at)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET balance=11 WHERE id=$1`, u.ID)
	require.NoError(t, err) // 12+3+1-5
	span, err := service.ParseFinanceRange("2026-02-01", "2026-02-02", "day", at)
	require.NoError(t, err)
	d, err := r.Load(ctx, span)
	require.NoError(t, err)
	require.Len(t, d.Costs, 1)
	require.True(t, decimal.RequireFromString("0.13").Equal(d.Costs[0].Cost))
	require.True(t, decimal.RequireFromString("5").Equal(d.Costs[0].Consumption))
	require.True(t, decimal.RequireFromString("2").Equal(d.Costs[0].Subscription))
	require.Equal(t, "2026-02-01", d.Costs[0].Day)
	require.Len(t, d.Orders, 1)
	require.True(t, d.Orders[0].LegacyFee)
	require.True(t, d.Orders[0].FeeRate.Equal(decimal.RequireFromString("0.032")))
	var consume, free decimal.Decimal
	for _, e := range d.Events {
		if e.Event.Kind == "consume" {
			consume = consume.Add(e.Event.Amount)
		}
		if e.Event.Kind == "free" {
			free = free.Add(e.Event.Amount)
		}
	}
	require.Equal(t, "5", consume.String())
	require.Equal(t, "3", free.String())
	summary, err := svc.Summary(ctx, span)
	require.NoError(t, err)
	totals := summary["totals"].(map[string]any)
	require.Equal(t, "0", totals["revenue"])
	require.Equal(t, "5", totals["free_consumed"])
	require.Equal(t, "0.13", totals["account_cost"])
	require.Equal(t, "0.5", totals["commission"])
	items, total, err := svc.Reconcile(ctx, 1, 50, false, u.Email)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "0", items[0]["difference"])
	expense, err := svc.SaveExpense(ctx, 0, u.ID, service.FinanceExpenseInput{OccurredOn: "2026-01-31", Category: "upstream_subscription", Amount: "10.00", AmortizeDays: 3, AccountID: &a.ID, Note: "跨月"})
	require.NoError(t, err)
	require.Equal(t, "3.33333333", expense.DailyAmount)
	// 列表按发生日，报表按摊销覆盖日。
	list, n, err := svc.ListExpenses(ctx, span, "", 1, 50)
	require.NoError(t, err)
	require.Empty(t, list)
	require.Zero(t, n)
	summary, err = svc.Summary(ctx, span)
	require.NoError(t, err)
	require.Equal(t, "6.66666667", summary["totals"].(map[string]any)["expenses"])
	_, err = svc.SaveSettings(ctx, []int64{a.ID})
	require.NoError(t, err)
	summary, err = svc.Summary(ctx, span)
	require.NoError(t, err)
	require.Equal(t, "0", summary["totals"].(map[string]any)["account_cost"])
	_, err = svc.SaveSettings(ctx, []int64{9223372036854775807})
	require.Error(t, err)
	require.NoError(t, svc.DeleteExpense(ctx, expense.ID))
	require.ErrorIs(t, svc.DeleteExpense(ctx, expense.ID), service.ErrFinanceExpenseNotFound)
	// 显式 0 扣回与 NULL 历史兜底不同；退款现金按退款日而不是下单日统计。
	_, err = c.PaymentOrder.UpdateOneID(o.ID).SetStatus("PARTIALLY_REFUNDED").SetRefundAt(at.AddDate(0, 0, 1)).SetRefundAmount(1).SetRefundBalanceDeducted("0").SetChannelFeeRate("0.01").Save(ctx)
	require.NoError(t, err)
	d, err = r.Load(ctx, span)
	require.NoError(t, err)
	require.False(t, d.Orders[0].LegacyFee)
	require.True(t, d.Orders[0].RefundDeducted.IsZero())
	require.Equal(t, "0.01", d.Orders[0].FeeRate.String())
	recharge, err := svc.Recharge(ctx, span)
	require.NoError(t, err)
	require.Equal(t, "8.9", recharge["totals"].(map[string]any)["net_cash"])
	require.Equal(t, "S2A-"+code, recharge["orders"].([]map[string]any)[0]["out_trade_no"])
	affiliate, err := svc.Affiliate(ctx, span)
	require.NoError(t, err)
	require.Equal(t, "1", affiliate["transferred"])
	breakdown, err := svc.Breakdown(ctx, span, "account", 200)
	require.NoError(t, err)
	require.Equal(t, "fixed", breakdown["rows"].([]map[string]any)[0]["cost_mode"])
}

func TestFinanceMigrationNumericColumnsAndConstraints(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		table, column    string
		precision, scale int
		nullable         string
	}{{"payment_provider_instances", "channel_fee_rate", 6, 4, "NO"}, {"payment_orders", "channel_fee_rate", 6, 4, "YES"}, {"payment_orders", "refund_balance_deducted", 20, 8, "YES"}, {"finance_expenses", "amount", 20, 2, "NO"}} {
		var typ, nullable string
		var precision, scale int
		err := integrationDB.QueryRowContext(ctx, `SELECT data_type,numeric_precision,numeric_scale,is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`, tc.table, tc.column).Scan(&typ, &precision, &scale, &nullable)
		require.NoError(t, err)
		require.Equal(t, "numeric", typ)
		require.Equal(t, tc.precision, precision)
		require.Equal(t, tc.scale, scale)
		require.Equal(t, tc.nullable, nullable)
	}
	for _, query := range []string{`INSERT INTO finance_expenses(occurred_on,category,amount) VALUES(CURRENT_DATE,'other',0)`, `INSERT INTO finance_expenses(occurred_on,category,amount,amortize_days) VALUES(CURRENT_DATE,'other',1,3661)`, `INSERT INTO finance_expenses(occurred_on,category,amount) VALUES(CURRENT_DATE,'unknown',1)`} {
		_, err := integrationDB.ExecContext(ctx, query)
		require.Error(t, err)
	}
}

// CUSTOM(fork): 当前统计规则按账号或任一所属分组匹配，不看用量分组、图片占比或是否开启渠道售价应用。
func TestFinanceCostModeCurrentPricingConfiguration(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	suffix := time.Now().UnixNano()
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("finance-mode-%d@test.local", suffix)})
	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("finance-mode-%d", suffix)})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Name: "finance-mode", Key: fmt.Sprintf("sk-finance-mode-%d", suffix)})
	groups := []int64{}
	for i := 0; i < 3; i++ {
		g := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("finance-mode-%d-%d", suffix, i), RateMultiplier: 1})
		groups = append(groups, g.ID)
	}
	var channel, rule int64
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM finance_expenses WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM channels WHERE id=$1`, channel)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE user_id=$1`, user.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE id=$1`, key.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
		for _, id := range groups {
			_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, id)
		}
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	})
	for _, id := range groups[:2] {
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account.ID, id)
		require.NoError(t, err)
	}
	at := time.Date(2026, 10, 25, 12, 0, 0, 0, service.AffiliateTeamLocation)
	// 用量无分组、只有图片，但默认成本仍按 total_cost × 账号倍率。
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,account_rate_multiplier,image_count,created_at) VALUES($1,$2,$3,'image-model',2,5,0.3,10,$4)`, user.ID, key.ID, account.ID, at)
	require.NoError(t, err)
	repo := NewFinanceRepository(integrationDB)
	svc := service.NewFinanceService(repo)
	span, err := service.ParseFinanceRange("2026-10-25", "2026-10-25", "day", at)
	require.NoError(t, err)
	check := func(want string) {
		t.Helper()
		out, err := svc.Breakdown(ctx, span, "account", 200)
		require.NoError(t, err)
		for _, row := range out["rows"].([]map[string]any) {
			if row["key"] == fmt.Sprint(account.ID) {
				require.Equal(t, want, row["cost_mode"])
				require.Equal(t, "0.6", row["account_cost"], "标签变化不重算快照成本")
				return
			}
		}
		t.Fatal("缺少测试账号")
	}
	check("rate")
	require.NoError(t, NewAccountModelCostRepository(integrationDB).ReplaceForAccount(ctx, account.ID, []service.AccountModelCostRule{{Model: "image-model", RateMultiplier: 0.1}}))
	check("model_rate")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO channels(name,apply_pricing_to_account_stats) VALUES($1,FALSE) RETURNING id`, fmt.Sprintf("finance-mode-%d", suffix)).Scan(&channel))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO channel_account_stats_pricing_rules(channel_id,account_ids) VALUES($1,ARRAY[$2::bigint]) RETURNING id`, channel, account.ID).Scan(&rule))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO channel_account_stats_model_pricing(rule_id,billing_mode) VALUES($1,'token')`, rule)
	require.NoError(t, err)
	check("model_rate")
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_account_stats_model_pricing SET billing_mode='image' WHERE rule_id=$1`, rule)
	require.NoError(t, err)
	check("per_image")
	// 账号直接匹配去掉，改为第二个所属分组匹配；不依赖用量中的 group_id。
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_account_stats_pricing_rules SET account_ids='{}',group_ids=ARRAY[$2::bigint] WHERE id=$1`, rule, groups[1])
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_account_stats_model_pricing SET billing_mode='per_request' WHERE rule_id=$1`, rule)
	require.NoError(t, err)
	check("per_image")
	// 同样的按次定价若只覆盖未所属分组，就不能标记该账号。
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_account_stats_pricing_rules SET group_ids=ARRAY[$2::bigint] WHERE id=$1`, rule, groups[2])
	require.NoError(t, err)
	check("model_rate")
	require.NoError(t, NewAccountModelCostRepository(integrationDB).ReplaceForAccount(ctx, account.ID, nil))
	check("rate")

	// CUSTOM(fork): 删除用量后仅靠关联支出出行，取数仍带完整当前配置与软删除标记。
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE user_id=$1`, user.ID)
	require.NoError(t, err)
	expense, err := repo.SaveExpense(ctx, 0, user.ID, service.FinanceExpenseInput{OccurredOn: "2026-10-24", Category: "other", Amount: "10", AmortizeDays: 3, AccountID: &account.ID})
	require.NoError(t, err)
	checkExpense := func(mode, label string) {
		t.Helper()
		out, err := svc.Breakdown(ctx, span, "account", 1000)
		require.NoError(t, err)
		for _, row := range out["rows"].([]map[string]any) {
			if row["key"] != fmt.Sprint(account.ID) {
				continue
			}
			require.Equal(t, mode, row["cost_mode"])
			require.Equal(t, label, row["label"])
			require.Equal(t, "3.33333333", row["expense_cost"])
			require.Equal(t, "-3.33333333", row["gross_profit"])
			require.Equal(t, "0", row["account_cost"])
			require.Equal(t, int64(0), row["requests"])
			return
		}
		t.Fatal("缺少无用量的支出关联账号")
	}
	checkExpense("rate", account.Name)
	require.NoError(t, NewAccountModelCostRepository(integrationDB).ReplaceForAccount(ctx, account.ID, []service.AccountModelCostRule{{Model: "image-model", RateMultiplier: 0.1}}))
	checkExpense("model_rate", account.Name)
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_account_stats_pricing_rules SET group_ids=ARRAY[$2::bigint] WHERE id=$1`, rule, groups[1])
	require.NoError(t, err)
	checkExpense("per_image", account.Name)
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_account_stats_pricing_rules SET group_ids='{}',account_ids=ARRAY[$2::bigint] WHERE id=$1`, rule, account.ID)
	require.NoError(t, err)
	checkExpense("per_image", account.Name)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET deleted_at=NOW() WHERE id=$1`, account.ID)
	require.NoError(t, err)
	checkExpense("per_image", account.Name+"（已删除）")
	require.NoError(t, repo.DeleteExpense(ctx, expense.ID))
	data, err := repo.Load(ctx, span)
	require.NoError(t, err)
	for _, a := range data.ExpenseAccounts {
		require.NotEqual(t, account.ID, a.ID)
	}
	out, err := svc.Breakdown(ctx, span, "account", 1000)
	require.NoError(t, err)
	for _, row := range out["rows"].([]map[string]any) {
		require.NotEqual(t, fmt.Sprint(account.ID), row["key"])
	}
}
