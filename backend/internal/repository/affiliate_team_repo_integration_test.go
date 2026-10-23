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

// teamFixture 构造 design 第 3 节的范围场景：
//
//	B 一级代理；A、Z 为 B 名下的当前二级（Z 无客户）；H 曾是 B 的二级，已降级，老客户 H1 的上方一级仍是 B。
//	B1 B 的直属客户；B2 由 B1（普通用户）邀请、归属 B；A1 A 的客户（上方一级 B）；
//	X 是 B 成为代理前邀请的人，归属其他代理 O，不在 B 的范围内。
type teamFixture struct {
	B, A, Z, H, B1, B2, A1, H1, X, O *service.User
	apiKeyID, accountID              int64
}

func newTeamFixture(t *testing.T, ctx context.Context, client *dbent.Client) *teamFixture {
	t.Helper()
	f := &teamFixture{}
	mk := func(label, nickname string) *service.User {
		return mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("team-%s-%d@example.com", label, time.Now().UnixNano()),
			Username:     nickname,
			PasswordHash: "hash",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			Concurrency:  5,
		})
	}
	f.B, f.A, f.Z, f.H = mk("b", "老B"), mk("a", "小王"), mk("z", ""), mk("h", "老赵")
	f.B1, f.B2, f.A1, f.H1 = mk("b1", "张三"), mk("b2", ""), mk("a1", "李四"), mk("h1", "")
	f.X, f.O = mk("x", "路人"), mk("o", "")

	repo := NewAffiliateRepository(client, integrationDB)
	for _, u := range []*service.User{f.B, f.A, f.Z, f.H, f.B1, f.B2, f.A1, f.H1, f.X, f.O} {
		_, err := repo.EnsureUserAffiliate(ctx, u.ID)
		require.NoError(t, err)
	}
	set := func(user *service.User, level int, inviter, owner, upline *service.User) {
		var inviterID, ownerID, uplineID any
		if inviter != nil {
			inviterID = inviter.ID
		}
		if owner != nil {
			ownerID = owner.ID
		}
		if upline != nil {
			uplineID = upline.ID
		}
		_, err := client.ExecContext(ctx, `
UPDATE user_affiliates
SET agent_level = $2, inviter_id = $3, agent_owner_id = $4, agent_upline_id = $5,
    agent_owner_bound_at = CASE WHEN $4::bigint IS NULL THEN NULL ELSE NOW() END
WHERE user_id = $1`, user.ID, level, inviterID, ownerID, uplineID)
		require.NoError(t, err)
	}
	set(f.B, 1, nil, nil, nil)
	set(f.O, 1, nil, nil, nil)
	set(f.A, 2, f.B, f.B, nil)
	set(f.Z, 2, f.B, f.B, nil)
	set(f.H, 3, f.B, f.B, nil)
	set(f.B1, 3, f.B, f.B, nil)
	set(f.B2, 3, f.B1, f.B, nil)
	set(f.A1, 3, f.A, f.A, f.B)
	set(f.H1, 3, f.H, f.H, f.B)
	set(f.X, 3, f.B, f.O, nil)

	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("team-%d", time.Now().UnixNano())})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: f.B1.ID, Name: "team", Key: fmt.Sprintf("sk-team-%d", time.Now().UnixNano())})
	f.apiKeyID, f.accountID = apiKey.ID, account.ID
	return f
}

func (f *teamFixture) usage(t *testing.T, ctx context.Context, client *dbent.Client, user *service.User, model string, cost float64, billingType int8, at time.Time) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
INSERT INTO usage_logs (user_id, api_key_id, account_id, model, requested_model, input_tokens, output_tokens,
                        cache_read_tokens, total_cost, actual_cost, billing_type, created_at)
VALUES ($1, $2, $3, $4 || '-upstream', $4, 100, 10, 1000, $5, $5, $6, $7)`,
		user.ID, f.apiKeyID, f.accountID, model, cost, billingType, at)
	require.NoError(t, err)
}

func teamLedger(t *testing.T, ctx context.Context, client *dbent.Client, beneficiary, source *service.User, rewardType string, amount float64, at time.Time) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, reward_type, created_at, updated_at)
VALUES ($1, 'accrue', $2, $3, $4, $5, $5)`, beneficiary.ID, amount, source.ID, rewardType, at)
	require.NoError(t, err)
}

func TestAffiliateTeamRepository_ScopesFollowOwnerSnapshot(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	f := newTeamFixture(t, txCtx, client)
	repo := NewAffiliateTeamRepository(client)

	now := time.Now()
	rng, err := service.ParseAffiliateTeamRange("", "", now)
	require.NoError(t, err)

	f.usage(t, txCtx, client, f.B1, "claude", 1.0, service.BillingTypeBalance, now.Add(-2*time.Hour))
	f.usage(t, txCtx, client, f.B1, "gpt", 2.0, service.BillingTypeBalance, now.Add(-time.Hour))
	f.usage(t, txCtx, client, f.B1, "claude", 9.0, service.BillingTypeSubscription, now.Add(-time.Hour))
	f.usage(t, txCtx, client, f.B1, "claude", 7.0, service.BillingTypeBalance, now.AddDate(0, 0, -30))
	f.usage(t, txCtx, client, f.A, "claude", 0.5, service.BillingTypeBalance, now.Add(-time.Hour))
	f.usage(t, txCtx, client, f.A1, "claude", 3.0, service.BillingTypeBalance, now.Add(-time.Hour))
	f.usage(t, txCtx, client, f.H1, "gpt", 4.0, service.BillingTypeBalance, now.Add(-time.Hour))
	f.usage(t, txCtx, client, f.X, "claude", 50.0, service.BillingTypeBalance, now.Add(-time.Hour))

	teamLedger(t, txCtx, client, f.B, f.B1, service.AffiliateRewardAgentDirect, 0.6, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.B1, service.AffiliateRewardAgentDirect, 5.0, now.AddDate(0, 0, -30))
	teamLedger(t, txCtx, client, f.B, f.B1, service.AffiliateRewardInvitePercent, 9.9, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.A, service.AffiliateRewardAgentDirect, 0.1, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.A1, service.AffiliateRewardAgentDiff, 0.3, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.H1, service.AffiliateRewardAgentDiff, 0.4, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.A, f.A1, service.AffiliateRewardAgentDirect, 0.6, now.Add(-30*time.Minute))

	t.Run("viewer state", func(t *testing.T) {
		state, err := repo.GetViewerState(txCtx, f.H.ID)
		require.NoError(t, err)
		require.Equal(t, 3, state.AgentLevel)
		require.True(t, state.HasOwnedCustomers, "降级者仍有老客户")
		state, err = repo.GetViewerState(txCtx, f.B1.ID)
		require.NoError(t, err)
		require.False(t, state.HasOwnedCustomers || state.HasUplineCustomer)
	})

	t.Run("counts and summary", func(t *testing.T) {
		counts, err := repo.CountTeam(txCtx, f.B.ID)
		require.NoError(t, err)
		require.Equal(t, service.AffiliateTeamCounts{DirectCustomers: 5, SubAgents: 3}, counts)

		agg, err := repo.GetAggregates(txCtx, service.AffiliateTeamScope{ViewerID: f.B.ID}, rng)
		require.NoError(t, err)
		sum := agg.Summary
		require.InDelta(t, 10.5, sum.Cost, 1e-9)
		require.InDelta(t, 3.5, sum.DirectCost, 1e-9)
		require.InDelta(t, 7.0, sum.SubCost, 1e-9)
		require.InDelta(t, 1.4, sum.Earnings, 1e-9, "只计代理分润且只计范围内")
		require.Equal(t, int64(4), sum.ActiveCustomers)
		require.Equal(t, int64(7), sum.Customers)
		require.Equal(t, int64(5), sum.Requests)
		require.Equal(t, int64(5*1110), sum.Tokens)

		// AC4：与直接对 usage_logs 汇总一致。
		var raw float64
		rawRows, err := client.QueryContext(txCtx, `
SELECT COALESCE(SUM(ul.actual_cost), 0)::double precision
FROM usage_logs ul
JOIN user_affiliates ua ON ua.user_id = ul.user_id
WHERE (ua.agent_owner_id = $1 OR ua.agent_upline_id = $1)
  AND ul.billing_type = 0 AND ul.created_at >= $2 AND ul.created_at < $3`, f.B.ID, rng.Start, rng.End)
		require.NoError(t, err)
		require.True(t, rawRows.Next())
		require.NoError(t, rawRows.Scan(&raw))
		require.NoError(t, rawRows.Close())
		require.InDelta(t, raw, sum.Cost, 1e-9)

		var trendCost, trendEarnings float64
		for _, p := range agg.Trend {
			trendCost += p.Cost
			trendEarnings += p.Earnings
		}
		require.InDelta(t, 10.5, trendCost, 1e-9)
		require.InDelta(t, 1.4, trendEarnings, 1e-9)

		require.Equal(t, []service.AffiliateTeamModelShare{{Model: "gpt", Cost: 6}, {Model: "claude", Cost: 4.5}}, agg.Models)

		// 区间内无消费：客户数仍来自范围，其余为 0。
		empty, err := service.ParseAffiliateTeamRange("2020-01-01", "2020-01-07", now)
		require.NoError(t, err)
		agg, err = repo.GetAggregates(txCtx, service.AffiliateTeamScope{ViewerID: f.B.ID}, empty)
		require.NoError(t, err)
		require.Equal(t, service.AffiliateTeamSummary{Customers: 7}, agg.Summary)
		require.Empty(t, agg.Trend)
	})

	t.Run("my customers", func(t *testing.T) {
		items, total, err := repo.ListCustomers(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{
			SortBy: service.AffiliateTeamSortCost, SortDesc: true, Page: 1, PageSize: 20,
		})
		require.NoError(t, err)
		require.Equal(t, int64(5), total)
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.User.UserID)
		}
		require.ElementsMatch(t, []int64{f.A.ID, f.Z.ID, f.H.ID, f.B1.ID, f.B2.ID}, ids)
		require.Equal(t, f.B1.ID, items[0].User.UserID)
		require.InDelta(t, 3.0, items[0].Cost, 1e-9)
		require.Equal(t, int64(2), items[0].Requests)
		require.InDelta(t, 0.6, items[0].Earnings, 1e-9)
		require.InDelta(t, 3.0/3.5, items[0].CostShare, 1e-9)
		require.NotNil(t, items[0].LastUsedAt)
		require.Equal(t, "张三", items[0].User.Nickname)
		require.Equal(t, f.A.ID, items[1].User.UserID)
		require.Equal(t, 2, items[1].AgentLevel)

		// 搜索：昵称模糊、邮箱完整匹配；邮箱前缀不命中。
		items, total, err = repo.ListCustomers(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{Search: "张", Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, f.B1.ID, items[0].User.UserID)
		_, total, err = repo.ListCustomers(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{Search: f.B2.Email, Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		_, total, err = repo.ListCustomers(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{Search: f.B2.Email[:8], Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost})
		require.NoError(t, err)
		require.Zero(t, total)
		_, total, err = repo.ListCustomers(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{Search: "%", Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost})
		require.NoError(t, err)
		require.Zero(t, total, "LIKE 通配符需转义")

		// 二级代理 A 只看到自己的客户。
		items, total, err = repo.ListCustomers(txCtx, f.A.ID, rng, service.AffiliateTeamListQuery{Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost, SortDesc: true})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, f.A1.ID, items[0].User.UserID)
		require.InDelta(t, 0.6, items[0].Earnings, 1e-9)
	})

	t.Run("sub agents", func(t *testing.T) {
		items, total, err := repo.ListAgents(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{
			SortBy: service.AffiliateTeamSortTeamCost, SortDesc: true, Page: 1, PageSize: 20,
		})
		require.NoError(t, err)
		require.Equal(t, int64(3), total)
		require.Len(t, items, 3)
		require.Equal(t, f.H.ID, items[0].User.UserID)
		require.True(t, items[0].Historical)
		require.InDelta(t, 4.0, items[0].TeamCost, 1e-9)
		require.InDelta(t, 0.4, items[0].Earnings, 1e-9)
		require.Equal(t, int64(1), items[0].Customers)
		require.Equal(t, int64(1), items[0].ActiveCustomers)
		require.Equal(t, f.A.ID, items[1].User.UserID)
		require.False(t, items[1].Historical)
		require.InDelta(t, 3.0, items[1].TeamCost, 1e-9, "A 本人消费不计入其团队")
		require.InDelta(t, 3.0/7.0, items[1].CostShare, 1e-9)
		require.Equal(t, f.Z.ID, items[2].User.UserID)
		require.Zero(t, items[2].Customers)

		team, total, err := repo.ListCustomers(txCtx, f.B.ID, rng, service.AffiliateTeamListQuery{AgentID: f.A.ID, Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, f.A1.ID, team[0].User.UserID)
		require.InDelta(t, 0.3, team[0].Earnings, 1e-9)

		agg, err := repo.GetAggregates(txCtx, service.AffiliateTeamScope{ViewerID: f.B.ID, AgentID: f.A.ID}, rng)
		require.NoError(t, err)
		require.InDelta(t, 3.0, agg.Summary.Cost, 1e-9)
		require.InDelta(t, 0.3, agg.Summary.Earnings, 1e-9)
		require.InDelta(t, 3.0, agg.Summary.SubCost, 1e-9)
		require.Equal(t, int64(1), agg.Summary.Customers)
	})

	t.Run("targets outside scope are not found", func(t *testing.T) {
		agent, err := repo.GetSubAgent(txCtx, f.B.ID, f.H.ID)
		require.NoError(t, err)
		require.NotNil(t, agent)
		require.True(t, agent.Historical)
		for _, id := range []int64{f.X.ID, f.A1.ID, f.O.ID, f.B.ID} {
			agent, err = repo.GetSubAgent(txCtx, f.B.ID, id)
			require.NoError(t, err)
			require.Nil(t, agent, "user %d", id)
		}
		agent, err = repo.GetSubAgent(txCtx, f.O.ID, f.A.ID)
		require.NoError(t, err)
		require.Nil(t, agent)

		customer, err := repo.GetCustomer(txCtx, f.B.ID, f.A1.ID)
		require.NoError(t, err)
		require.Equal(t, "upline", customer.Relation)
		require.Equal(t, f.A.ID, customer.OwnerAgent.UserID)
		require.False(t, customer.IsSubAgent)
		customer, err = repo.GetCustomer(txCtx, f.B.ID, f.A.ID)
		require.NoError(t, err)
		require.Equal(t, "owner", customer.Relation)
		require.Nil(t, customer.OwnerAgent)
		require.True(t, customer.IsSubAgent)
		customer, err = repo.GetCustomer(txCtx, f.B.ID, f.B1.ID)
		require.NoError(t, err)
		require.InDelta(t, 5.6, customer.LifetimeEarnings, 1e-9)
		for _, id := range []int64{f.X.ID, f.O.ID, f.B.ID} {
			customer, err = repo.GetCustomer(txCtx, f.B.ID, id)
			require.NoError(t, err)
			require.Nil(t, customer, "user %d", id)
		}
		customer, err = repo.GetCustomer(txCtx, f.A.ID, f.H1.ID)
		require.NoError(t, err)
		require.Nil(t, customer)
	})

	t.Run("customer requests and filters", func(t *testing.T) {
		items, total, err := repo.ListRequests(txCtx, f.B1.ID, rng, service.AffiliateTeamListQuery{Page: 1, PageSize: 20})
		require.NoError(t, err)
		require.Equal(t, int64(2), total, "订阅消费与区间外的记录不出现")
		require.Equal(t, "gpt", items[0].Model, "按时间倒序，模型为用户请求的模型")
		require.Equal(t, int64(1000), items[0].CacheReadTokens)
		require.InDelta(t, 2.0, items[0].Cost, 1e-9)

		items, total, err = repo.ListRequests(txCtx, f.B1.ID, rng, service.AffiliateTeamListQuery{Page: 1, PageSize: 20, Model: "claude"})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, "claude", items[0].Model)
		zero := int64(0)
		_, total, err = repo.ListRequests(txCtx, f.B1.ID, rng, service.AffiliateTeamListQuery{Page: 1, PageSize: 20, GroupID: &zero})
		require.NoError(t, err)
		require.Equal(t, int64(2), total)

		filters, err := repo.GetCustomerFilters(txCtx, f.B1.ID, rng)
		require.NoError(t, err)
		require.Equal(t, []string{"claude", "gpt"}, filters.Models)
		require.Equal(t, []service.AffiliateTeamGroup{{ID: 0, Name: ""}}, filters.Groups)
	})
}

func TestAffiliateRepository_ListInviteesSearchAndPagination(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	f := newTeamFixture(t, txCtx, client)
	repo := NewAffiliateRepository(client, integrationDB)

	// B 的直属邀请人：A、Z、H、B1、X（X 归属别人，但仍是 B 邀请的）。
	items, total, err := repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, items, 2)
	page3, _, err := repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Page: 3, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page3, 1)

	items, total, err = repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Search: "小", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, f.A.ID, items[0].UserID)
	require.Equal(t, "小王", items[0].Username)

	_, total, err = repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Search: f.X.Email, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "完整邮箱精确匹配")
	_, total, err = repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Search: f.X.Email[:8], Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Zero(t, total, "邮箱前缀不匹配")
	_, total, err = repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Search: "_", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Zero(t, total, "LIKE 通配符需转义")

	// 已删除的邀请人仍在列表里，带出删除标记，服务层据此不再提供任命 / 取消。
	_, err = client.ExecContext(txCtx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", f.Z.ID)
	require.NoError(t, err)
	items, total, err = repo.ListInvitees(txCtx, f.B.ID, service.AffiliateInviteeQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	for _, item := range items {
		require.Equal(t, item.UserID == f.Z.ID, item.Deleted, "user %d", item.UserID)
	}
}
