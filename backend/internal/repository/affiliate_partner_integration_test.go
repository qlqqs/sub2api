//go:build integration

package repository

// CUSTOM(fork): 合伙人写入、区域取数和结算在真实 PostgreSQL 上验证。
import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type partnerFixture struct {
	ctx          context.Context
	repo         service.PartnerRepository
	aff          service.AffiliateRepository
	svc          *service.PartnerService
	users        []*service.User
	account, key int64
	at           time.Time
}

func newPartnerFixture(t *testing.T) *partnerFixture {
	t.Helper()
	c := testEntClient(t)
	f := &partnerFixture{ctx: context.Background(), at: time.Date(2026, 2, 2, 12, 0, 0, 0, service.AffiliateTeamLocation)}
	f.repo = NewPartnerRepository(integrationDB, c)
	f.aff = NewAffiliateRepository(c, integrationDB)
	f.svc = service.NewPartnerService(f.repo, nil, nil)
	t.Cleanup(func() {
		ids := []int64{}
		for _, u := range f.users {
			ids = append(ids, u.ID)
		}
		for _, query := range []string{
			`DELETE FROM affiliate_partner_statements WHERE partner_user_id=ANY($1)`,
			`DELETE FROM affiliate_partner_adjustments WHERE partner_user_id=ANY($1)`,
			`DELETE FROM affiliate_partner_rates WHERE partner_user_id=ANY($1)`,
			`DELETE FROM affiliate_partner_agents WHERE partner_user_id=ANY($1)`,
			`DELETE FROM affiliate_partners WHERE user_id=ANY($1)`,
			`DELETE FROM finance_expenses WHERE created_by=ANY($1)`,
			`DELETE FROM usage_logs WHERE user_id=ANY($1)`,
			`DELETE FROM usage_user_daily_rollups WHERE user_id=ANY($1)`,
			`DELETE FROM user_affiliate_ledger WHERE user_id=ANY($1) OR source_user_id=ANY($1)`,
			`DELETE FROM redeem_codes WHERE used_by=ANY($1)`,
			`DELETE FROM payment_orders WHERE user_id=ANY($1)`,
			`DELETE FROM api_keys WHERE user_id=ANY($1)`,
			`DELETE FROM user_affiliates WHERE user_id=ANY($1)`,
			`DELETE FROM users WHERE id=ANY($1)`,
		} {
			_, err := integrationDB.ExecContext(f.ctx, query, pq.Array(ids))
			require.NoError(t, err)
		}
		_, err := integrationDB.ExecContext(f.ctx, `DELETE FROM accounts WHERE id=$1`, f.account)
		require.NoError(t, err)
	})
	for i := 0; i < 6; i++ {
		u := mustCreateUser(t, c, &service.User{Email: fmt.Sprintf("partner-%d-%d@test.local", time.Now().UnixNano(), i), CreatedAt: f.at.AddDate(0, -1, 0)})
		f.users = append(f.users, u)
		_, err := f.aff.EnsureUserAffiliate(f.ctx, u.ID)
		require.NoError(t, err)
	}
	p, a, customer, manual, auto := f.users[0].ID, f.users[1].ID, f.users[2].ID, f.users[4].ID, f.users[5].ID
	require.NoError(t, f.repo.Create(f.ctx, p, "测试区域", decimal.RequireFromString("0.4"), p))
	_, err := integrationDB.ExecContext(f.ctx, `UPDATE affiliate_partners SET created_at=$2 WHERE user_id=$1`, p, f.at.Add(-2*time.Hour))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(f.ctx, `UPDATE affiliate_partner_rates SET effective_month='2026-02-01' WHERE partner_user_id=$1`, p)
	require.NoError(t, err)
	require.NoError(t, f.aff.SetAgentLevelManual(f.ctx, []int64{a}, 1, p))
	require.NoError(t, f.repo.Attach(f.ctx, p, a, p))
	_, err = integrationDB.ExecContext(f.ctx, `UPDATE affiliate_partner_agents SET attached_at=$2 WHERE agent_user_id=$1`, a, f.at)
	require.NoError(t, err)
	for _, v := range []struct{ user, owner int64 }{{customer, a}, {manual, p}, {auto, a}} {
		_, err = integrationDB.ExecContext(f.ctx, `UPDATE user_affiliates SET agent_owner_id=$2,agent_owner_bound_at=$3 WHERE user_id=$1`, v.user, v.owner, f.at.Add(-24*time.Hour))
		require.NoError(t, err)
	}
	account := mustCreateAccount(t, c, &service.Account{Name: fmt.Sprintf("partner-%d", time.Now().UnixNano())})
	key := mustCreateApiKey(t, c, &service.APIKey{UserID: customer, Name: "partner", Key: fmt.Sprintf("sk-partner-%d", time.Now().UnixNano())})
	f.account, f.key = account.ID, key.ID
	return f
}
func (f *partnerFixture) usage(t *testing.T, user int64, actual, stats string, billing int, at time.Time) {
	t.Helper()
	_, err := integrationDB.ExecContext(f.ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,requested_model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,billing_type,created_at) VALUES($1,$2,$3,'vendor-model','public-model',100,$4,$5,0.5,$6,$7)`, user, f.key, f.account, actual, stats, billing, at)
	require.NoError(t, err)
}
func TestPartnerCreateAttachAndAutomaticPromotionAtomic(t *testing.T) {
	f := newPartnerFixture(t)
	p, a, manual, auto := f.users[0].ID, f.users[1].ID, f.users[4].ID, f.users[5].ID
	require.ErrorIs(t, f.repo.Create(f.ctx, p, "重复", decimal.Zero, p), service.ErrPartnerExists)
	require.ErrorIs(t, f.repo.Create(f.ctx, 9223372036854775807, "缺失", decimal.Zero, p), service.ErrUserNotFound)
	require.ErrorIs(t, f.repo.Create(f.ctx, a, "挂靠者", decimal.Zero, p), service.ErrPartnerAttached)
	require.ErrorIs(t, f.repo.Attach(f.ctx, p, a, p), service.ErrPartnerAttached)
	require.ErrorIs(t, f.repo.Attach(f.ctx, p, p, p), service.ErrPartnerAgentIsPartner)
	require.ErrorIs(t, f.repo.Attach(f.ctx, p, manual, p), service.ErrPartnerAgentLevel)
	require.ErrorIs(t, f.aff.SetAgentLevelManual(f.ctx, []int64{manual, p}, 2, p), service.ErrPartnerLevelFixed)
	require.ErrorIs(t, f.aff.SetAgentLevelLocked(f.ctx, []int64{p}, false), service.ErrPartnerLevelFixed)
	nodes, err := f.aff.GetAffiliateNodes(f.ctx, []int64{p, manual})
	require.NoError(t, err)
	require.True(t, nodes[p].Partner)
	require.True(t, nodes[p].Active)
	require.Equal(t, 3, nodes[manual].AgentLevel)
	failed := errors.New("回滚挂靠")
	err = f.aff.WithTx(f.ctx, func(ctx context.Context) error {
		require.NoError(t, f.aff.SetAgentLevelManual(ctx, []int64{manual}, 1, p))
		return failed
	})
	require.ErrorIs(t, err, failed)
	state, err := f.repo.Read(f.ctx)
	require.NoError(t, err)
	require.NotContains(t, state.Attachments, manual)
	require.Equal(t, 3, state.Snapshots[manual].AgentLevel)
	affsvc := service.ProvideAffiliateService(f.aff, nil, nil, nil, f.repo)
	attached, err := affsvc.AdminSetAgentLevelWithAttachments(f.ctx, []int64{manual}, 1, p)
	require.NoError(t, err)
	require.Len(t, attached, 1)
	require.Equal(t, p, attached[0].Partner.ID)
	promoted, err := f.aff.PromoteAgentLevelAuto(f.ctx, auto, 1)
	require.NoError(t, err)
	require.True(t, promoted)
	state, err = f.repo.Read(f.ctx)
	require.NoError(t, err)
	require.Equal(t, p, state.Attachments[manual].PartnerID)
	require.Equal(t, "promote", state.Attachments[auto].Source)
	require.Nil(t, state.Attachments[auto].AttachedByEmail)
	require.Equal(t, a, *state.Snapshots[auto].OwnerID, "升级不改已有快照")
	rows, total, err := f.aff.ListUsersWithCustomSettings(f.ctx, service.AffiliateAdminFilter{PartnerID: p, Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, p, row.Partner.UserID)
		require.Equal(t, row.UserID == p, row.IsPartner)
	}
}

func TestPartnerLoadCostsLedgerAndFrozenConcurrentStatement(t *testing.T) {
	f := newPartnerFixture(t)
	p, customer := f.users[0].ID, f.users[2].ID
	f.usage(t, customer, "10", "20", 0, f.at.Add(-time.Hour))
	f.usage(t, customer, "30", "4", 0, f.at.Add(time.Hour))
	f.usage(t, customer, "9", "6", 1, f.at.Add(time.Hour))
	f.usage(t, customer, "8", "8", 2, f.at.Add(time.Hour))
	c := testEntClient(t)
	code := fmt.Sprintf("PARTNER-%d", time.Now().UnixNano())
	_, err := c.PaymentOrder.Create().SetUserID(customer).SetUserEmail(f.users[2].Email).SetUserName("").SetAmount(100).SetBonusAmount(0).SetPayAmount(100).SetRechargeCode(code).SetOutTradeNo(code).SetPaymentType("wxpay").SetPaymentTradeNo("").SetClientIP("127.0.0.1").SetSrcHost("localhost").SetStatus("COMPLETED").SetOrderType("balance").SetChannelFeeRate("0").SetCompletedAt(f.at.Add(-24 * time.Hour)).SetExpiresAt(f.at.Add(time.Hour)).Save(f.ctx)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(f.ctx, `UPDATE users SET balance=60 WHERE id=$1`, customer)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(f.ctx, `INSERT INTO user_affiliate_ledger(user_id,source_user_id,action,amount,base_amount,reward_type,created_at) VALUES($1,$2,'accrue',0.7,100,'agent_direct',$3),($1,$2,'accrue',0.21,1.4,'agent_direct',$4),($1,$2,'accrue',0.5,3,'invite_fixed',$4),($1,$2,'accrue',0.4,10,NULL,$4),($1,$2,'accrue',5,10,'invitee_bonus',$4),($1,$2,'transfer',9,10,NULL,$4),($1,$2,'accrue',6,10,'unknown',$4)`, f.users[3].ID, customer, f.at.Add(-time.Hour), f.at.Add(time.Hour))
	require.NoError(t, err)
	finance := service.NewFinanceService(NewFinanceRepository(integrationDB))
	expense, err := finance.SaveExpense(f.ctx, 0, p, service.FinanceExpenseInput{OccurredOn: "2026-02-02", Category: "other", Amount: "3.00", AmortizeDays: 1, PartnerUserID: &p, Note: "区域费用"})
	require.NoError(t, err)
	require.Equal(t, "测试区域", *expense.PartnerRegionName)
	missing := int64(9223372036854775807)
	_, err = finance.SaveExpense(f.ctx, 0, p, service.FinanceExpenseInput{OccurredOn: "2026-02-02", Category: "other", Amount: "1", AmortizeDays: 1, PartnerUserID: &missing})
	require.ErrorContains(t, err, "FINANCE_EXPENSE_PARTNER_INVALID")
	_, err = finance.SaveExpense(f.ctx, 0, p, service.FinanceExpenseInput{OccurredOn: "2026-02-02", Category: "other", Amount: "8", AmortizeDays: 1})
	require.NoError(t, err)
	data, err := f.repo.LoadMonths(f.ctx, []int64{p}, "2026-02", "2026-02")
	require.NoError(t, err)
	require.Empty(t, data.Finance.Costs, "共享财务读取不重复扫描成本")
	require.Equal(t, "40", data.StartDays[customer].Total.String())
	require.Equal(t, "30", data.StartDays[customer].After.String())
	require.Len(t, data.Usage, 1)
	require.Equal(t, "9", data.Usage[0].Cost.String())
	require.Equal(t, "public-model", data.Usage[0].Model)
	adjustment, err := f.svc.AddAdjustment(f.ctx, p, "2026-02", decimal.NewFromInt(1), "修正", p)
	require.NoError(t, err)
	draft, err := f.svc.Statement(f.ctx, p, "2026-02")
	require.NoError(t, err)
	require.Equal(t, "30", draft.Totals.Consumption)
	require.Equal(t, "30", draft.Totals.Revenue)
	require.Equal(t, "1.11", draft.Totals.Commission)
	require.Equal(t, "3", draft.Totals.RegionExpense)
	require.Equal(t, "17.89", draft.Totals.Profit)
	require.Equal(t, "7.16", draft.Totals.ShareAmount)
	span, err := service.ParseFinanceRange("2026-02-02", "2026-02-02", "day", f.at)
	require.NoError(t, err)
	summary, err := finance.Summary(f.ctx, span)
	require.NoError(t, err)
	require.Equal(t, "11", summary["totals"].(map[string]any)["expenses"], "专项支出全站只计一次")
	_, err = f.svc.Confirm(f.ctx, p, "2026-03", p)
	require.ErrorContains(t, err, "PARTNER_PREVIOUS_UNCONFIRMED")
	_, err = f.svc.Confirm(f.ctx, p, time.Now().In(service.AffiliateTeamLocation).Format("2006-01"), p)
	require.ErrorContains(t, err, "PARTNER_MONTH_NOT_ENDED")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, e := f.svc.Confirm(f.ctx, p, "2026-02", p); errs <- e }()
	}
	close(start)
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else {
			require.ErrorIs(t, e, service.ErrPartnerMonthConfirmed)
			conflict++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)
	frozen, err := f.svc.Statement(f.ctx, p, "2026-02")
	require.NoError(t, err)
	require.NotNil(t, frozen.Totals.ConfirmedAt)
	var rawStatus string
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `SELECT detail->'totals'->>'status' FROM affiliate_partner_statements WHERE partner_user_id=$1 AND month='2026-02-01'`, p).Scan(&rawStatus))
	require.Equal(t, "confirmed", rawStatus)
	_, err = integrationDB.ExecContext(f.ctx, `UPDATE usage_logs SET account_stats_cost=999 WHERE user_id=$1`, customer)
	require.NoError(t, err)
	after, err := f.svc.Statement(f.ctx, p, "2026-02")
	require.NoError(t, err)
	require.Equal(t, frozen, after)
	_, err = f.svc.AddAdjustment(f.ctx, p, "2026-02", decimal.NewFromInt(1), "冻结后", p)
	require.ErrorIs(t, err, service.ErrPartnerMonthConfirmed)
	require.ErrorIs(t, f.svc.DeleteAdjustment(f.ctx, p, adjustment.ID), service.ErrPartnerMonthConfirmed)
	paid, err := f.svc.Pay(f.ctx, p, "2026-02", p, "2026-03-03", "本机转账记录")
	require.NoError(t, err)
	require.Equal(t, "paid", paid.Totals.Status)
	require.Equal(t, "7.16", paid.Totals.ShareAmount)
	_, err = f.svc.Pay(f.ctx, p, "2026-02", p, "", "重复")
	require.ErrorContains(t, err, "PARTNER_MONTH_PAID")
	rate := decimal.RequireFromString("0.35")
	detail, err := f.svc.Update(f.ctx, p, nil, &rate, p)
	require.NoError(t, err)
	require.Equal(t, "0.4000", detail.ShareRate)
	require.Equal(t, "0.3500", *detail.NextShareRate)
	rate = decimal.RequireFromString("0.4")
	detail, err = f.svc.Update(f.ctx, p, nil, &rate, p)
	require.NoError(t, err)
	require.Nil(t, detail.NextShareRate)
	// 固定成本账号只清零成本，保留区域余额消耗。
	var old string
	oldErr := integrationDB.QueryRowContext(f.ctx, `SELECT value FROM settings WHERE key=$1`, service.FinanceFixedAccountsKey).Scan(&old)
	t.Cleanup(func() {
		if oldErr == nil {
			_, _ = integrationDB.ExecContext(f.ctx, `UPDATE settings SET value=$2 WHERE key=$1`, service.FinanceFixedAccountsKey, old)
		} else {
			_, _ = integrationDB.ExecContext(f.ctx, `DELETE FROM settings WHERE key=$1`, service.FinanceFixedAccountsKey)
		}
	})
	_, err = finance.SaveSettings(f.ctx, []int64{f.account})
	require.NoError(t, err)
	data, err = f.repo.LoadMonths(f.ctx, []int64{p}, "2026-02", "2026-02")
	require.NoError(t, err)
	require.True(t, data.Usage[0].Cost.IsZero())
	require.Equal(t, "30", data.Usage[0].Consumption.String())
}

func TestPartnerRegionAggregatesEqualMembers(t *testing.T) {
	f := newPartnerFixture(t)
	p, a, customer, outside := f.users[0].ID, f.users[1].ID, f.users[2].ID, f.users[3].ID
	f.usage(t, p, "2", "1", 0, f.at)
	f.usage(t, a, "3", "1", 0, f.at)
	f.usage(t, customer, "5", "1", 0, f.at)
	f.usage(t, outside, "90", "1", 0, f.at)
	state, err := f.repo.Read(f.ctx)
	require.NoError(t, err)
	ids := []int64{}
	for id, m := range service.ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots) {
		if m.PartnerID == p {
			ids = append(ids, id)
		}
	}
	rng, err := service.ParseAffiliateTeamRange("2026-02-02", "2026-02-02", f.at)
	require.NoError(t, err)
	repo := NewAffiliateTeamRepository(testEntClient(t))
	agg, err := repo.GetAggregates(f.ctx, service.AffiliateTeamScope{ViewerID: p, MemberIDs: ids}, rng)
	require.NoError(t, err)
	require.Equal(t, 10.0, agg.Summary.Cost)
	require.EqualValues(t, 3, agg.Summary.Requests)
	require.Zero(t, agg.Summary.Earnings)
	rows, total, err := repo.ListCustomers(f.ctx, p, rng, service.AffiliateTeamListQuery{MemberIDs: ids, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	sum := 0.0
	for _, row := range rows {
		require.NotEqual(t, outside, row.User.UserID)
		sum += row.Cost
	}
	require.Equal(t, agg.Summary.Cost, sum)
}

// CUSTOM(fork): 客户详情只读取当前区域内该客户的用量，不回退到区域汇总。
type partnerTeamIntegrationSettings struct{ service.SettingRepository }

func (partnerTeamIntegrationSettings) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyAffiliateEnabled {
		return "true", nil
	}
	return "", service.ErrSettingNotFound
}

func TestPartnerTeamOverviewCustomerDataIsolation(t *testing.T) {
	f := newPartnerFixture(t)
	p, agent, customer, outside := f.users[0].ID, f.users[1].ID, f.users[2].ID, f.users[3].ID
	f.usage(t, customer, "5", "1", 0, f.at)
	for _, row := range []struct {
		user  int64
		model string
		cost  float64
		at    time.Time
	}{
		{agent, "agent-only", 7, f.at},
		{outside, "outside-only", 90, f.at},
		{customer, "outside-date-range", 100, f.at.AddDate(0, 0, 1)},
	} {
		_, err := integrationDB.ExecContext(f.ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,requested_model,total_cost,actual_cost,billing_type,created_at) VALUES($1,$2,$3,$4,$4,$5,$5,0,$6)`, row.user, f.key, f.account, row.model, row.cost, row.at)
		require.NoError(t, err)
	}
	settings := service.NewSettingService(partnerTeamIntegrationSettings{}, nil)
	team := service.NewAffiliateTeamService(NewAffiliateTeamRepository(testEntClient(t)), settings)
	affiliate := service.NewAffiliateService(f.aff, settings, nil, nil)
	svc := service.NewPartnerService(f.repo, affiliate, team)
	rng, err := service.ParseAffiliateTeamRange("2026-02-01", "2026-02-02", f.at)
	require.NoError(t, err)

	v, err := svc.TeamOverview(f.ctx, p, customer, rng)
	require.NoError(t, err)
	require.NotNil(t, v.Customer)
	require.Equal(t, customer, v.Customer.User.UserID)
	require.Equal(t, "upline", v.Customer.Relation)
	require.NotNil(t, v.Customer.OwnerAgent)
	require.Equal(t, agent, v.Customer.OwnerAgent.UserID)
	require.Contains(t, v.Customer.User.Email, "*")
	require.Contains(t, v.Customer.OwnerAgent.Email, "*")
	require.NotEqual(t, f.users[2].Email, v.Customer.User.Email)
	require.NotEqual(t, f.users[1].Email, v.Customer.OwnerAgent.Email)
	require.Equal(t, service.PartnerTeamSummary{Cost: 5, SubCost: 5, ActiveCustomers: 1, Customers: 1, Requests: 1}, v.Summary)
	require.Equal(t, []service.PartnerTeamTrend{{Date: "2026-02-01"}, {Date: "2026-02-02", Cost: 5}}, v.Trend)
	require.Equal(t, []service.AffiliateTeamModelShare{{Model: "public-model", Cost: 5, Share: 1}}, v.Models)
	require.NotNil(t, v.Filters)
	require.Equal(t, []string{"public-model"}, v.Filters.Models)

	_, err = svc.TeamOverview(f.ctx, p, outside, rng)
	require.ErrorIs(t, err, service.ErrPartnerForbidden)
	region, err := svc.TeamOverview(f.ctx, p, 0, rng)
	require.NoError(t, err)
	require.Nil(t, region.Customer)
	require.Nil(t, region.Filters)
	require.Equal(t, 12.0, region.Summary.Cost)
	require.EqualValues(t, 5, region.Summary.Customers)
}
