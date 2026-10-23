//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateTeamAdminAccessIgnoresFeatureFlag(t *testing.T) {
	cases := []struct {
		name   string
		state  AffiliateTeamViewerState
		access bool
	}{
		{name: "level-1 agent", state: AffiliateTeamViewerState{AgentLevel: 1}, access: true},
		{name: "level-2 agent", state: AffiliateTeamViewerState{AgentLevel: 2}, access: true},
		{name: "downgraded with owned customers", state: AffiliateTeamViewerState{AgentLevel: 3, HasOwnedCustomers: true}, access: true},
		{name: "downgraded with upline customers", state: AffiliateTeamViewerState{AgentLevel: 3, HasUplineCustomer: true}, access: true},
		{name: "normal user", state: AffiliateTeamViewerState{AgentLevel: 3}, access: false},
	}
	for _, tc := range cases {
		for _, enabled := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/affiliate_enabled=%v", tc.name, enabled), func(t *testing.T) {
				svc := newAffiliateTeamTestService(&fakeAffiliateTeamRepo{state: tc.state}, enabled)
				ctx := context.Background()
				rng := mustTeamRange(t, "", "")

				_, err := svc.AdminViewerOverview(ctx, 7, 0, 0, rng)
				_, _, customersErr := svc.AdminViewerCustomers(ctx, 7, rng, AffiliateTeamListQuery{})
				_, _, agentsErr := svc.AdminViewerAgents(ctx, 7, rng, AffiliateTeamListQuery{})
				for _, got := range []error{err, customersErr, agentsErr} {
					if tc.access {
						require.NoError(t, got)
					} else {
						// 管理端：没有团队的用户与不存在的 ID 一样是 404，而不是代理端的 403。
						require.ErrorIs(t, got, ErrAffiliateTeamNotFound)
					}
				}
			})
		}
	}
}

func TestAffiliateTeamAdminViewerShowsRawEmailAndSubject(t *testing.T) {
	repo := &fakeAffiliateTeamRepo{
		state:     AffiliateTeamViewerState{AgentLevel: 2},
		subAgents: map[int64]*AffiliateTeamAgentSubject{20: {User: AffiliateTeamUser{UserID: 20, Email: "agent@example.com"}}},
		customers: map[int64]*AffiliateTeamCustomerSubject{
			30: {User: AffiliateTeamUser{UserID: 30, Nickname: " 张三 ", Email: "zhangsan@example.com"}, Relation: "upline",
				OwnerAgent: &AffiliateTeamUser{UserID: 20, Email: "agent@example.com"}},
		},
		customer: []AffiliateTeamCustomerEntry{
			{User: AffiliateTeamUser{UserID: 1, Nickname: "bob", Email: "bob@example.com"}},
			{User: AffiliateTeamUser{UserID: 2, Nickname: "gone", Email: "gone@example.com", Deleted: true}},
		},
		subject: AffiliateTeamViewerSubject{
			User:             AffiliateTeamUser{UserID: 7, Nickname: " 小王 ", Email: "wang@example.com"},
			Upline:           &AffiliateTeamUser{UserID: 5, Nickname: "张总", Email: "boss@example.com"},
			LifetimeEarnings: 88,
		},
	}
	svc := newAffiliateTeamTestService(repo, true)
	ctx := context.Background()
	rng := mustTeamRange(t, "", "")

	for _, scope := range []struct{ agentID, customerID int64 }{{0, 0}, {20, 0}, {0, 30}} {
		out, err := svc.AdminViewerOverview(ctx, 7, scope.agentID, scope.customerID, rng)
		require.NoError(t, err)
		require.NotNil(t, out.Subject, "各层都带被查看的代理")
		require.Equal(t, AffiliateTeamUser{UserID: 7, Nickname: "小王", Email: "wang@example.com"}, out.Subject.User)
		require.Equal(t, "boss@example.com", out.Subject.Upline.Email)
		require.InDelta(t, 88.0, out.Subject.LifetimeEarnings, 1e-9)
		require.Equal(t, 2, out.Viewer.AgentLevel)
		switch {
		case scope.agentID > 0:
			require.Equal(t, "agent@example.com", out.Agent.User.Email)
		case scope.customerID > 0:
			require.Equal(t, "zhangsan@example.com", out.Customer.User.Email)
			require.Equal(t, "agent@example.com", out.Customer.OwnerAgent.Email)
		}
	}

	items, _, err := svc.AdminViewerCustomers(ctx, 7, rng, AffiliateTeamListQuery{Search: " 42 "})
	require.NoError(t, err)
	require.True(t, repo.lastQuery.AdminSearch)
	require.Equal(t, "42", repo.lastQuery.Search)
	require.Equal(t, "bob@example.com", items[0].User.Email)
	require.Equal(t, AffiliateTeamUser{UserID: 2, Deleted: true}, items[1].User, "已删除账号仍只留 ID")

	// 代理端：不带 subject（JSON 里也没有该键），邮箱脱敏，搜索不是管理端模式。
	out, err := svc.Overview(ctx, 7, 0, 0, rng)
	require.NoError(t, err)
	require.Nil(t, out.Subject)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"subject"`)
	items, _, err = svc.ListCustomers(ctx, 7, rng, AffiliateTeamListQuery{Search: "bob"})
	require.NoError(t, err)
	require.False(t, repo.lastQuery.AdminSearch)
	require.Contains(t, items[0].User.Email, "***")
}

func TestAffiliateTeamAdminListOwnersNormalizesQuery(t *testing.T) {
	repo := &fakeAffiliateTeamRepo{
		owners: []AffiliateTeamOwnerEntry{
			{User: AffiliateTeamUser{UserID: 1, Nickname: " 张三 ", Email: "zhang@example.com"}, AgentLevel: 1},
			{User: AffiliateTeamUser{UserID: 2, Nickname: "小王", Email: "wang@example.com"}, AgentLevel: 2,
				Upline: &AffiliateTeamUser{UserID: 1, Nickname: "张三", Email: "zhang@example.com"}},
			{User: AffiliateTeamUser{UserID: 3, Nickname: "gone", Email: "gone@example.com", Deleted: true}, AgentLevel: 0,
				Upline: &AffiliateTeamUser{UserID: 9, Nickname: "x", Email: "x@example.com", Deleted: true}},
		},
	}
	svc := newAffiliateTeamTestService(repo, false)
	ctx := context.Background()
	rng := mustTeamRange(t, "", "")

	items, total, err := svc.AdminListOwners(ctx, rng, AffiliateTeamListQuery{Level: "bogus", SortBy: AffiliateTeamSortCost, PageSize: 500, Search: "  zh  "})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Equal(t, AffiliateTeamLevelAll, repo.lastQuery.Level)
	require.Equal(t, AffiliateTeamSortTeamCost, repo.lastQuery.SortBy)
	require.Equal(t, 100, repo.lastQuery.PageSize)
	require.Equal(t, "zh", repo.lastQuery.Search)
	require.True(t, repo.lastQuery.AdminSearch)

	require.Equal(t, AffiliateTeamUser{UserID: 1, Nickname: "张三", Email: "zhang@example.com"}, items[0].User)
	require.Equal(t, "zhang@example.com", items[1].Upline.Email)
	require.Equal(t, AffiliateTeamUser{UserID: 3, Deleted: true}, items[2].User)
	require.Equal(t, AffiliateAgentLevelDefault, items[2].AgentLevel)
	require.Equal(t, &AffiliateTeamUser{UserID: 9, Deleted: true}, items[2].Upline)

	for _, level := range []string{AffiliateTeamLevel1, AffiliateTeamLevel2, AffiliateTeamLevelDowngraded} {
		_, _, err = svc.AdminListOwners(ctx, rng, AffiliateTeamListQuery{Level: level, SortBy: AffiliateTeamSortEarnings})
		require.NoError(t, err)
		require.Equal(t, level, repo.lastQuery.Level)
		require.Equal(t, AffiliateTeamSortEarnings, repo.lastQuery.SortBy)
	}
}

func TestAffiliateTeamAdminOverviewFillsTrendAndMergesModels(t *testing.T) {
	repo := &fakeAffiliateTeamRepo{
		platform: AffiliateTeamPlatformAggregates{
			Counts:  AffiliateTeamOwnerCounts{All: 4, Level1: 1, Level2: 2, Downgraded: 1},
			Summary: AffiliateTeamPlatformSummary{Cost: 100, PlatformCost: 250, Earnings: 20, ActiveCustomers: 3, Customers: 9},
			Trend:   []AffiliateTeamTrendPoint{{Date: "2026-10-02", Cost: 100, Earnings: 20}},
			Models: []AffiliateTeamModelShare{
				{Model: "a", Cost: 40}, {Model: "b", Cost: 20}, {Model: "c", Cost: 10},
				{Model: "d", Cost: 10}, {Model: "e", Cost: 10}, {Model: "f", Cost: 6}, {Model: "g", Cost: 4},
			},
		},
	}
	svc := newAffiliateTeamTestService(repo, false)
	out, err := svc.AdminOverview(context.Background(), mustTeamRange(t, "2026-10-01", "2026-10-03"))
	require.NoError(t, err)
	require.Equal(t, 30, out.SettleIntervalMinutes)
	require.Equal(t, repo.platform.Counts, out.Counts)
	require.Equal(t, repo.platform.Summary, out.Summary)
	require.Equal(t, []AffiliateTeamTrendPoint{
		{Date: "2026-10-01"},
		{Date: "2026-10-02", Cost: 100, Earnings: 20},
		{Date: "2026-10-03"},
	}, out.Trend)
	require.Len(t, out.Models, 6)
	require.Equal(t, "", out.Models[5].Model)
	require.InDelta(t, 0.1, out.Models[5].Share, 1e-9)
}
