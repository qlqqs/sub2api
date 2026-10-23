//go:build integration

package repository

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// 汇总页的行与该代理自己的团队摘要逐项相等；汇总条与直接对明细的 SQL 一致。
// 集成库被其他测试提交的数据共享，全站口径一律与同一事务内的原始 SQL 对比，收录范围用本次的昵称标记隔离。
func TestAffiliateTeamAdminRepository_OwnersMatchViewerAggregates(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	f := newTeamFixture(t, txCtx, client)
	repo := NewAffiliateTeamRepository(client)

	tag := fmt.Sprintf("adm%d", time.Now().UnixNano())
	fixture := []*service.User{f.B, f.A, f.Z, f.H, f.B1, f.B2, f.A1, f.H1, f.X, f.O}
	ids := make([]int64, 0, len(fixture))
	for _, u := range fixture {
		ids = append(ids, u.ID)
	}
	_, err := client.ExecContext(txCtx, `UPDATE users SET username = $1 || '-' || username WHERE id = ANY($2)`, tag, pq.Array(ids))
	require.NoError(t, err)

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
	f.usage(t, txCtx, client, f.O, "claude", 6.0, service.BillingTypeBalance, now.Add(-time.Hour)) // 代理本人不是渠道客户

	teamLedger(t, txCtx, client, f.B, f.B1, service.AffiliateRewardAgentDirect, 0.6, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.B1, service.AffiliateRewardAgentDirect, 5.0, now.AddDate(0, 0, -30))
	teamLedger(t, txCtx, client, f.B, f.B1, service.AffiliateRewardInvitePercent, 9.9, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.A, service.AffiliateRewardAgentDirect, 0.1, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.A1, service.AffiliateRewardAgentDiff, 0.3, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.B, f.H1, service.AffiliateRewardAgentDiff, 0.4, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.A, f.A1, service.AffiliateRewardAgentDirect, 0.6, now.Add(-30*time.Minute))
	// X 已改挂到 O：改挂前给 B 的分润只计入汇总条，不计入 B 的行。
	teamLedger(t, txCtx, client, f.B, f.X, service.AffiliateRewardAgentDirect, 0.7, now.Add(-30*time.Minute))
	teamLedger(t, txCtx, client, f.O, f.X, service.AffiliateRewardAgentDirect, 5.0, now.Add(-20*time.Minute))

	list := func(query service.AffiliateTeamListQuery) ([]service.AffiliateTeamOwnerEntry, int64) {
		t.Helper()
		if query.Page == 0 {
			query.Page, query.PageSize = 1, 50
		}
		if query.SortBy == "" {
			query.SortBy, query.SortDesc = service.AffiliateTeamSortTeamCost, true
		}
		query.AdminSearch = true
		items, total, err := repo.ListTeamOwners(txCtx, rng, query)
		require.NoError(t, err)
		return items, total
	}
	userIDs := func(items []service.AffiliateTeamOwnerEntry) []int64 {
		out := make([]int64, 0, len(items))
		for _, it := range items {
			out = append(out, it.User.UserID)
		}
		return out
	}

	t.Run("rows equal each agent's own summary", func(t *testing.T) {
		items, total := list(service.AffiliateTeamListQuery{Search: tag})
		require.Equal(t, int64(5), total, "B、O、A、Z、H；客户与普通用户不收")
		require.Equal(t, []int64{f.O.ID, f.B.ID, f.H.ID, f.A.ID, f.Z.ID}, userIDs(items), "按团队消费降序")

		for _, it := range items {
			agg, err := repo.GetAggregates(txCtx, service.AffiliateTeamScope{ViewerID: it.User.UserID}, rng)
			require.NoError(t, err)
			require.InDelta(t, agg.Summary.Cost, it.TeamCost, 1e-9, "user %d", it.User.UserID)
			require.InDelta(t, agg.Summary.Earnings, it.Earnings, 1e-9, "user %d", it.User.UserID)
			require.Equal(t, agg.Summary.ActiveCustomers, it.ActiveCustomers, "user %d", it.User.UserID)
			require.Equal(t, agg.Summary.Customers, it.Customers, "user %d", it.User.UserID)
		}

		byID := make(map[int64]service.AffiliateTeamOwnerEntry, len(items))
		for _, it := range items {
			byID[it.User.UserID] = it
		}
		b := byID[f.B.ID]
		require.Equal(t, 1, b.AgentLevel)
		require.Nil(t, b.Upline)
		require.InDelta(t, 10.5, b.TeamCost, 1e-9, "一级包含其二级团队")
		require.InDelta(t, 1.4, b.Earnings, 1e-9, "已改挂走的 X 不计入")
		require.Equal(t, int64(7), b.Customers)
		require.Equal(t, int64(4), b.ActiveCustomers)
		a := byID[f.A.ID]
		require.Equal(t, 2, a.AgentLevel)
		require.Equal(t, f.B.ID, a.Upline.UserID)
		require.Equal(t, f.B.Email, a.Upline.Email)
		require.InDelta(t, 3.0, a.TeamCost, 1e-9, "A 本人消费不计入其团队")
		require.Equal(t, 3, byID[f.H.ID].AgentLevel)
		require.Nil(t, byID[f.H.ID].Upline, "已降级不显示上级")
		require.Equal(t, f.A.Email, a.User.Email, "管理端原文邮箱")

		channel := sumRawChannelCost(t, txCtx, client, rng)
		require.InDelta(t, b.TeamCost/channel, b.CostShare, 1e-9, "占比以汇总条消费为分母")
	})

	t.Run("level filter, search and sort", func(t *testing.T) {
		items, _ := list(service.AffiliateTeamListQuery{Search: tag, Level: service.AffiliateTeamLevel1})
		require.ElementsMatch(t, []int64{f.B.ID, f.O.ID}, userIDs(items))
		items, _ = list(service.AffiliateTeamListQuery{Search: tag, Level: service.AffiliateTeamLevel2})
		require.ElementsMatch(t, []int64{f.A.ID, f.Z.ID}, userIDs(items))
		items, _ = list(service.AffiliateTeamListQuery{Search: tag, Level: service.AffiliateTeamLevelDowngraded})
		require.Equal(t, []int64{f.H.ID}, userIDs(items))

		items, _ = list(service.AffiliateTeamListQuery{Search: tag + "-小"})
		require.Equal(t, []int64{f.A.ID}, userIDs(items), "昵称片段")
		items, _ = list(service.AffiliateTeamListQuery{Search: f.H.Email[:len(f.H.Email)-6]})
		require.Equal(t, []int64{f.H.ID}, userIDs(items), "邮箱片段")
		// 纯数字同时模糊匹配邮箱 / 昵称里的数字，所以只断言目标在结果中；精确性见 deleted owners。
		items, _ = list(service.AffiliateTeamListQuery{Search: strconv.FormatInt(f.O.ID, 10)})
		require.Contains(t, userIDs(items), f.O.ID, "ID 匹配")
		_, total := list(service.AffiliateTeamListQuery{Search: tag + "%"})
		require.Zero(t, total, "LIKE 通配符需转义")
		_, total = list(service.AffiliateTeamListQuery{Search: tag + "_"})
		require.Zero(t, total)

		items, _ = list(service.AffiliateTeamListQuery{Search: tag, SortBy: service.AffiliateTeamSortEarnings, SortDesc: true})
		require.Equal(t, f.O.ID, items[0].User.UserID)
		require.InDelta(t, 5.0, items[0].Earnings, 1e-9)
		items, _ = list(service.AffiliateTeamListQuery{Search: tag, SortBy: service.AffiliateTeamSortActiveCustomers, SortDesc: false})
		require.Equal(t, f.Z.ID, items[0].User.UserID)

		items, total = list(service.AffiliateTeamListQuery{Search: tag, Page: 2, PageSize: 2})
		require.Equal(t, int64(5), total)
		require.Equal(t, []int64{f.H.ID, f.A.ID}, userIDs(items))
	})

	t.Run("deleted owners", func(t *testing.T) {
		_, err := client.ExecContext(txCtx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, f.Z.ID)
		require.NoError(t, err)
		items, _ := list(service.AffiliateTeamListQuery{Search: tag})
		require.NotContains(t, userIDs(items), f.Z.ID, "已删除账号不参与昵称 / 邮箱搜索")
		// 已删除账号不参与模糊匹配，能否找到只取决于 ID 精确匹配。
		items, _ = list(service.AffiliateTeamListQuery{Search: strconv.FormatInt(f.Z.ID, 10)})
		var z *service.AffiliateTeamOwnerEntry
		for i := range items {
			if items[i].User.UserID == f.Z.ID {
				z = &items[i]
			}
		}
		require.NotNil(t, z)
		require.True(t, z.User.Deleted)
		require.Equal(t, 2, z.AgentLevel)
		items, _ = list(service.AffiliateTeamListQuery{Search: strconv.FormatInt(f.Z.ID, 10) + "0"})
		require.NotContains(t, userIDs(items), f.Z.ID, "ID 不做前缀匹配")
	})

	t.Run("viewer subject", func(t *testing.T) {
		subject, err := repo.GetViewerSubject(txCtx, f.B.ID)
		require.NoError(t, err)
		require.Equal(t, f.B.Email, subject.User.Email)
		require.Nil(t, subject.Upline)
		require.InDelta(t, 7.1, subject.LifetimeEarnings, 1e-9, "全部历史的代理分润，不含邀请返利")
		subject, err = repo.GetViewerSubject(txCtx, f.A.ID)
		require.NoError(t, err)
		require.Equal(t, f.B.ID, subject.Upline.UserID)
		subject, err = repo.GetViewerSubject(txCtx, f.H.ID)
		require.NoError(t, err)
		require.Nil(t, subject.Upline, "已降级不显示上级")
		subject, err = repo.GetViewerSubject(txCtx, -1)
		require.NoError(t, err)
		require.True(t, subject.User.Deleted, "不存在的用户按已删除展示")
	})

	t.Run("admin customer search", func(t *testing.T) {
		search := func(viewerID int64, q string) []int64 {
			t.Helper()
			items, _, err := repo.ListCustomers(txCtx, viewerID, rng, service.AffiliateTeamListQuery{
				Search: q, AdminSearch: true, Page: 1, PageSize: 20, SortBy: service.AffiliateTeamSortCost,
			})
			require.NoError(t, err)
			out := make([]int64, 0, len(items))
			for _, it := range items {
				out = append(out, it.User.UserID)
			}
			return out
		}
		require.Equal(t, []int64{f.B2.ID}, search(f.B.ID, f.B2.Email[:10]), "邮箱片段")
		require.Equal(t, []int64{f.B1.ID}, search(f.B.ID, tag+"-张"))
		require.Contains(t, search(f.B.ID, strconv.FormatInt(f.H.ID, 10)), f.H.ID)
		require.NotContains(t, search(f.B.ID, strconv.FormatInt(f.X.ID, 10)), f.X.ID, "ID 搜索仍限于该代理的客户")
		require.Empty(t, search(f.B.ID, "%"))
	})

	t.Run("platform aggregates match raw SQL", func(t *testing.T) {
		agg, err := repo.GetPlatformAggregates(txCtx, rng)
		require.NoError(t, err)

		var (
			platformCost, earnings          float64
			active, customers               int64
			all, level1, level2, downgraded int64
		)
		require.InDelta(t, sumRawChannelCost(t, txCtx, client, rng), agg.Summary.Cost, 1e-6, "渠道客户的消费，每个客户只算一次")
		scanRaw(t, txCtx, client, `
SELECT COALESCE(SUM(actual_cost), 0)::double precision FROM usage_logs
WHERE billing_type = 0 AND created_at >= $1 AND created_at < $2`, []any{rng.Start, rng.End}, &platformCost)
		require.InDelta(t, platformCost, agg.Summary.PlatformCost, 1e-6)
		scanRaw(t, txCtx, client, `
SELECT COUNT(*) FROM (
    SELECT ul.user_id FROM usage_logs ul JOIN user_affiliates ua ON ua.user_id = ul.user_id
    WHERE ua.agent_owner_id IS NOT NULL AND ul.billing_type = 0 AND ul.created_at >= $1 AND ul.created_at < $2
    GROUP BY ul.user_id HAVING SUM(ul.actual_cost) > 0
) a`, []any{rng.Start, rng.End}, &active)
		require.Equal(t, active, agg.Summary.ActiveCustomers)
		scanRaw(t, txCtx, client, `SELECT COUNT(*) FROM user_affiliates WHERE agent_owner_id IS NOT NULL`, nil, &customers)
		require.Equal(t, customers, agg.Summary.Customers)
		scanRaw(t, txCtx, client, `
SELECT COALESCE(SUM(amount), 0)::double precision FROM user_affiliate_ledger
WHERE reward_type IN ('agent_direct', 'agent_diff') AND created_at >= $1 AND created_at < $2`, []any{rng.Start, rng.End}, &earnings)
		require.InDelta(t, earnings, agg.Summary.Earnings, 1e-6, "分润不限来源")
		scanRaw(t, txCtx, client, `
WITH owners AS (
    SELECT user_id FROM user_affiliates WHERE agent_level IN (1, 2)
    UNION SELECT agent_owner_id FROM user_affiliates WHERE agent_owner_id IS NOT NULL
    UNION SELECT agent_upline_id FROM user_affiliates WHERE agent_upline_id IS NOT NULL
)
SELECT COUNT(*),
       COUNT(*) FILTER (WHERE ua.agent_level = 1),
       COUNT(*) FILTER (WHERE ua.agent_level = 2),
       COUNT(*) FILTER (WHERE COALESCE(ua.agent_level, 3) NOT IN (1, 2))
FROM owners o LEFT JOIN user_affiliates ua ON ua.user_id = o.user_id`, nil, &all, &level1, &level2, &downgraded)
		require.Equal(t, service.AffiliateTeamOwnerCounts{All: all, Level1: level1, Level2: level2, Downgraded: downgraded}, agg.Counts)

		var trendCost, trendEarnings, modelCost float64
		for _, p := range agg.Trend {
			trendCost += p.Cost
			trendEarnings += p.Earnings
		}
		for _, m := range agg.Models {
			modelCost += m.Cost
		}
		require.InDelta(t, agg.Summary.Cost, trendCost, 1e-6)
		require.InDelta(t, agg.Summary.Earnings, trendEarnings, 1e-6)
		require.InDelta(t, agg.Summary.Cost, modelCost, 1e-6)
	})
}

func sumRawChannelCost(t *testing.T, ctx context.Context, client *dbent.Client, rng service.AffiliateTeamRange) float64 {
	t.Helper()
	var cost float64
	scanRaw(t, ctx, client, `
SELECT COALESCE(SUM(ul.actual_cost), 0)::double precision
FROM usage_logs ul JOIN user_affiliates ua ON ua.user_id = ul.user_id
WHERE ua.agent_owner_id IS NOT NULL AND ul.billing_type = 0 AND ul.created_at >= $1 AND ul.created_at < $2`,
		[]any{rng.Start, rng.End}, &cost)
	return cost
}

func scanRaw(t *testing.T, ctx context.Context, client *dbent.Client, query string, args []any, dest ...any) {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(dest...))
	require.NoError(t, rows.Err())
}
