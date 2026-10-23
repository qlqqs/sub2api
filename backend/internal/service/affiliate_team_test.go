//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeAffiliateTeamRepo struct {
	state     AffiliateTeamViewerState
	subAgents map[int64]*AffiliateTeamAgentSubject
	customers map[int64]*AffiliateTeamCustomerSubject
	trend     []AffiliateTeamTrendPoint
	models    []AffiliateTeamModelShare
	customer  []AffiliateTeamCustomerEntry
	lastQuery AffiliateTeamListQuery
	subject   AffiliateTeamViewerSubject
	platform  AffiliateTeamPlatformAggregates
	owners    []AffiliateTeamOwnerEntry
}

func (f *fakeAffiliateTeamRepo) GetViewerState(context.Context, int64) (AffiliateTeamViewerState, error) {
	return f.state, nil
}
func (f *fakeAffiliateTeamRepo) CountTeam(context.Context, int64) (AffiliateTeamCounts, error) {
	return AffiliateTeamCounts{DirectCustomers: 3, SubAgents: int64(len(f.subAgents))}, nil
}
func (f *fakeAffiliateTeamRepo) GetSubAgent(_ context.Context, _ int64, agentID int64) (*AffiliateTeamAgentSubject, error) {
	if a, ok := f.subAgents[agentID]; ok {
		copied := *a
		return &copied, nil
	}
	return nil, nil
}
func (f *fakeAffiliateTeamRepo) GetCustomer(_ context.Context, _ int64, customerID int64) (*AffiliateTeamCustomerSubject, error) {
	if c, ok := f.customers[customerID]; ok {
		copied := *c
		return &copied, nil
	}
	return nil, nil
}
func (f *fakeAffiliateTeamRepo) GetAggregates(context.Context, AffiliateTeamScope, AffiliateTeamRange) (AffiliateTeamAggregates, error) {
	return AffiliateTeamAggregates{Summary: AffiliateTeamSummary{Cost: 10}, Trend: f.trend, Models: f.models}, nil
}
func (f *fakeAffiliateTeamRepo) GetCustomerFilters(context.Context, int64, AffiliateTeamRange) (AffiliateTeamFilters, error) {
	return AffiliateTeamFilters{Models: []string{"m"}, Groups: []AffiliateTeamGroup{}}, nil
}
func (f *fakeAffiliateTeamRepo) ListCustomers(_ context.Context, _ int64, _ AffiliateTeamRange, q AffiliateTeamListQuery) ([]AffiliateTeamCustomerEntry, int64, error) {
	f.lastQuery = q
	return append([]AffiliateTeamCustomerEntry(nil), f.customer...), int64(len(f.customer)), nil
}
func (f *fakeAffiliateTeamRepo) ListAgents(_ context.Context, _ int64, _ AffiliateTeamRange, q AffiliateTeamListQuery) ([]AffiliateTeamAgentEntry, int64, error) {
	f.lastQuery = q
	return nil, 0, nil
}
func (f *fakeAffiliateTeamRepo) ListRequests(_ context.Context, _ int64, _ AffiliateTeamRange, q AffiliateTeamListQuery) ([]AffiliateTeamRequest, int64, error) {
	f.lastQuery = q
	return nil, 0, nil
}
func (f *fakeAffiliateTeamRepo) GetViewerSubject(context.Context, int64) (AffiliateTeamViewerSubject, error) {
	subject := f.subject
	if subject.Upline != nil {
		upline := *subject.Upline
		subject.Upline = &upline
	}
	return subject, nil
}
func (f *fakeAffiliateTeamRepo) GetPlatformAggregates(context.Context, AffiliateTeamRange) (AffiliateTeamPlatformAggregates, error) {
	return f.platform, nil
}
func (f *fakeAffiliateTeamRepo) ListTeamOwners(_ context.Context, _ AffiliateTeamRange, q AffiliateTeamListQuery) ([]AffiliateTeamOwnerEntry, int64, error) {
	f.lastQuery = q
	return append([]AffiliateTeamOwnerEntry(nil), f.owners...), int64(len(f.owners)), nil
}

func newAffiliateTeamTestService(repo *fakeAffiliateTeamRepo, enabled bool) *AffiliateTeamService {
	values := map[string]string{SettingKeyAffiliateSettleIntervalMinutes: "30"}
	if enabled {
		values[SettingKeyAffiliateEnabled] = "true"
	}
	return NewAffiliateTeamService(repo, NewSettingService(&twoTierSettingRepo{values: values}, nil))
}

func mustTeamRange(t *testing.T, start, end string) AffiliateTeamRange {
	t.Helper()
	rng, err := ParseAffiliateTeamRange(start, end, time.Now())
	require.NoError(t, err)
	return rng
}

func TestAffiliateTeamAccessMatrix(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		state   AffiliateTeamViewerState
		access  bool
	}{
		{name: "level-1 agent", enabled: true, state: AffiliateTeamViewerState{AgentLevel: 1}, access: true},
		{name: "level-2 agent", enabled: true, state: AffiliateTeamViewerState{AgentLevel: 2}, access: true},
		{name: "downgraded with owned customers", enabled: true, state: AffiliateTeamViewerState{AgentLevel: 3, HasOwnedCustomers: true}, access: true},
		{name: "downgraded with upline customers", enabled: true, state: AffiliateTeamViewerState{AgentLevel: 3, HasUplineCustomer: true}, access: true},
		{name: "normal user", enabled: true, state: AffiliateTeamViewerState{AgentLevel: 3}, access: false},
		{name: "affiliate disabled", enabled: false, state: AffiliateTeamViewerState{AgentLevel: 1}, access: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newAffiliateTeamTestService(&fakeAffiliateTeamRepo{state: tc.state}, tc.enabled)
			access, err := svc.HasTeamAccess(context.Background(), 7)
			require.NoError(t, err)
			require.Equal(t, tc.access, access)

			_, err = svc.Overview(context.Background(), 7, 0, 0, mustTeamRange(t, "", ""))
			if tc.access {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrAffiliateTeamForbidden)
			}
		})
	}
}

func TestAffiliateTeamOverviewMarksDowngradedViewer(t *testing.T) {
	svc := newAffiliateTeamTestService(&fakeAffiliateTeamRepo{state: AffiliateTeamViewerState{AgentLevel: 3, HasOwnedCustomers: true}}, true)
	out, err := svc.Overview(context.Background(), 7, 0, 0, mustTeamRange(t, "", ""))
	require.NoError(t, err)
	require.True(t, out.Viewer.Downgraded)
	require.Equal(t, 30, out.Viewer.SettleIntervalMinutes)
	require.NotNil(t, out.Counts)
	require.Nil(t, out.Agent)
	require.Nil(t, out.Customer)
}

func TestParseAffiliateTeamRange(t *testing.T) {
	now := time.Date(2026, 10, 22, 17, 0, 0, 0, time.UTC) // 北京时间 10-23 01:00
	rng, err := ParseAffiliateTeamRange("", "", now)
	require.NoError(t, err)
	require.Equal(t, "2026-10-17", rng.StartDate)
	require.Equal(t, "2026-10-23", rng.EndDate)
	require.True(t, rng.Start.Equal(time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)))
	require.True(t, rng.End.Equal(time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)))

	_, err = ParseAffiliateTeamRange("2026-07-25", "2026-10-22", now) // 90 天
	require.NoError(t, err)
	_, err = ParseAffiliateTeamRange("2026-07-24", "2026-10-22", now) // 91 天
	require.ErrorIs(t, err, ErrAffiliateTeamInvalidRange)
	_, err = ParseAffiliateTeamRange("2026-10-22", "2026-10-21", now)
	require.ErrorIs(t, err, ErrAffiliateTeamInvalidRange)
	_, err = ParseAffiliateTeamRange("2026/10/01", "2026-10-21", now)
	require.ErrorIs(t, err, ErrAffiliateTeamInvalidRange)
}

func TestAffiliateTeamOverviewFillsTrendAndMergesModels(t *testing.T) {
	repo := &fakeAffiliateTeamRepo{
		state: AffiliateTeamViewerState{AgentLevel: 1},
		trend: []AffiliateTeamTrendPoint{{Date: "2026-10-02", Cost: 5, Earnings: 1}},
		models: []AffiliateTeamModelShare{
			{Model: "a", Cost: 40}, {Model: "b", Cost: 20}, {Model: "c", Cost: 10},
			{Model: "d", Cost: 10}, {Model: "e", Cost: 10}, {Model: "f", Cost: 6}, {Model: "g", Cost: 4},
		},
	}
	svc := newAffiliateTeamTestService(repo, true)
	out, err := svc.Overview(context.Background(), 7, 0, 0, mustTeamRange(t, "2026-10-01", "2026-10-03"))
	require.NoError(t, err)

	require.Equal(t, []AffiliateTeamTrendPoint{
		{Date: "2026-10-01"},
		{Date: "2026-10-02", Cost: 5, Earnings: 1},
		{Date: "2026-10-03"},
	}, out.Trend)

	require.Len(t, out.Models, 6)
	require.Equal(t, "a", out.Models[0].Model)
	require.InDelta(t, 0.4, out.Models[0].Share, 1e-9)
	require.Equal(t, "", out.Models[5].Model, "其余模型合并为「其他」")
	require.InDelta(t, 10.0, out.Models[5].Cost, 1e-9)
	require.InDelta(t, 0.1, out.Models[5].Share, 1e-9)
}

func TestAffiliateTeamScopedViewsRejectOutOfScopeTargets(t *testing.T) {
	repo := &fakeAffiliateTeamRepo{
		state:     AffiliateTeamViewerState{AgentLevel: 1},
		subAgents: map[int64]*AffiliateTeamAgentSubject{20: {User: AffiliateTeamUser{UserID: 20, Email: "agent@example.com"}}},
		customers: map[int64]*AffiliateTeamCustomerSubject{
			30: {User: AffiliateTeamUser{UserID: 30, Nickname: " 张三 ", Email: "zhangsan@example.com"}, Relation: "upline",
				OwnerAgent: &AffiliateTeamUser{UserID: 20, Email: "agent@example.com"}},
		},
	}
	svc := newAffiliateTeamTestService(repo, true)
	ctx := context.Background()
	rng := mustTeamRange(t, "", "")

	_, err := svc.Overview(ctx, 7, 99, 0, rng)
	require.ErrorIs(t, err, ErrAffiliateTeamNotFound)
	_, err = svc.Overview(ctx, 7, 0, 99, rng)
	require.ErrorIs(t, err, ErrAffiliateTeamNotFound)
	_, _, err = svc.ListCustomers(ctx, 7, rng, AffiliateTeamListQuery{AgentID: 99})
	require.ErrorIs(t, err, ErrAffiliateTeamNotFound)
	_, _, err = svc.ListRequests(ctx, 7, 99, rng, AffiliateTeamListQuery{})
	require.ErrorIs(t, err, ErrAffiliateTeamNotFound)
	_, err = svc.Overview(ctx, 7, 20, 30, rng)
	require.Error(t, err)

	out, err := svc.Overview(ctx, 7, 0, 30, rng)
	require.NoError(t, err)
	require.Equal(t, "张三", out.Customer.User.Nickname)
	require.NotEqual(t, "zhangsan@example.com", out.Customer.User.Email)
	require.Contains(t, out.Customer.User.Email, "***")
	require.Contains(t, out.Customer.OwnerAgent.Email, "***")
	require.NotNil(t, out.Filters)

	out, err = svc.Overview(ctx, 7, 20, 0, rng)
	require.NoError(t, err)
	require.Contains(t, out.Agent.User.Email, "***")
}

func TestAffiliateTeamListNormalizesQueryAndHidesDeletedUsers(t *testing.T) {
	repo := &fakeAffiliateTeamRepo{
		state: AffiliateTeamViewerState{AgentLevel: 2},
		customer: []AffiliateTeamCustomerEntry{
			{User: AffiliateTeamUser{UserID: 1, Nickname: "gone", Email: "gone@example.com", Deleted: true}, AgentLevel: 0},
		},
	}
	svc := newAffiliateTeamTestService(repo, true)
	items, _, err := svc.ListCustomers(context.Background(), 7, mustTeamRange(t, "", ""),
		AffiliateTeamListQuery{SortBy: "email; DROP TABLE users", PageSize: 500, Search: "  bob  "})
	require.NoError(t, err)
	require.Equal(t, AffiliateTeamSortCost, repo.lastQuery.SortBy)
	require.Equal(t, 100, repo.lastQuery.PageSize)
	require.Equal(t, 1, repo.lastQuery.Page)
	require.Equal(t, "bob", repo.lastQuery.Search)
	require.Equal(t, AffiliateTeamUser{UserID: 1, Deleted: true}, items[0].User)
	require.Equal(t, AffiliateAgentLevelDefault, items[0].AgentLevel)

	_, _, err = svc.ListAgents(context.Background(), 7, mustTeamRange(t, "", ""), AffiliateTeamListQuery{SortBy: AffiliateTeamSortEarnings})
	require.NoError(t, err)
	require.Equal(t, AffiliateTeamSortEarnings, repo.lastQuery.SortBy)
	_, _, err = svc.ListAgents(context.Background(), 7, mustTeamRange(t, "", ""), AffiliateTeamListQuery{SortBy: AffiliateTeamSortCost})
	require.NoError(t, err)
	require.Equal(t, AffiliateTeamSortTeamCost, repo.lastQuery.SortBy)
}
