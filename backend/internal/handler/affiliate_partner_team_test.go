//go:build unit

package handler

// CUSTOM(fork): 区域客户总览的权限、脱敏和查询范围回归。
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type partnerTeamStateRepo struct {
	service.PartnerRepository
	state *service.PartnerState
}

func (r *partnerTeamStateRepo) Read(context.Context) (*service.PartnerState, error) {
	return r.state, nil
}

type partnerTeamOverviewRepo struct {
	service.AffiliateTeamRepository
	scopes                  []service.AffiliateTeamScope
	filterIDs               []int64
	ranges                  []service.AffiliateTeamRange
	filterErr, aggregateErr error
}

func (r *partnerTeamOverviewRepo) GetCustomerFilters(_ context.Context, id int64, rng service.AffiliateTeamRange) (service.AffiliateTeamFilters, error) {
	r.filterIDs = append(r.filterIDs, id)
	r.ranges = append(r.ranges, rng)
	return service.AffiliateTeamFilters{Models: []string{"customer-model"}, Groups: []service.AffiliateTeamGroup{{ID: 4, Name: "客户分组"}}}, r.filterErr
}
func (r *partnerTeamOverviewRepo) GetAggregates(_ context.Context, scope service.AffiliateTeamScope, rng service.AffiliateTeamRange) (service.AffiliateTeamAggregates, error) {
	r.scopes = append(r.scopes, scope)
	r.ranges = append(r.ranges, rng)
	return service.AffiliateTeamAggregates{
		Summary: service.AffiliateTeamSummary{Cost: 5, SubCost: 5, ActiveCustomers: 1, Customers: int64(len(scope.MemberIDs)), Requests: 2, Tokens: 30, Earnings: 999},
		Trend:   []service.AffiliateTeamTrendPoint{{Date: "2026-02-02", Cost: 5, Earnings: 999}},
		Models:  []service.AffiliateTeamModelShare{{Model: "customer-model", Cost: 5}},
	}, r.aggregateErr
}

func newPartnerTeamOverviewRouter(t *testing.T) (*gin.Engine, *partnerTeamOverviewRepo, *service.PartnerState) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	p, a, secondary, other := int64(1), int64(10), int64(11), int64(2)
	boundAt := time.Date(2026, 1, 1, 0, 0, 0, 0, service.AffiliateTeamLocation)
	state := &service.PartnerState{
		Partners:    map[int64]service.Partner{p: {UserID: p}, other: {UserID: other}},
		Attachments: map[int64]service.PartnerAttachment{a: {AgentID: a, PartnerID: p}},
		Snapshots: map[int64]service.PartnerSnapshot{
			p:         {User: service.PartnerUserRef{ID: p, Email: "partner@example.com"}, AgentLevel: 1},
			other:     {User: service.PartnerUserRef{ID: other}, AgentLevel: 1},
			a:         {User: service.PartnerUserRef{ID: a, Username: "  渠道经理  ", Email: "agent@example.com"}, AgentLevel: 1},
			secondary: {User: service.PartnerUserRef{ID: secondary, Username: "分销代理", Email: "secondary@example.com"}, AgentLevel: 2, OwnerID: &a},
			20:        {User: service.PartnerUserRef{ID: 20, Username: "  区域客户  ", Email: "customer@example.com"}, AgentLevel: 3, OwnerID: &a, BoundAt: &boundAt},
			21:        {User: service.PartnerUserRef{ID: 21, Username: "间接客户", Email: "indirect@example.com"}, AgentLevel: 3, OwnerID: &secondary, UplineID: &a},
			22:        {User: service.PartnerUserRef{ID: 22, Username: "直属代理"}, AgentLevel: 2, OwnerID: &p},
			30:        {User: service.PartnerUserRef{ID: 30}, OwnerID: &other},
		},
	}
	settings := service.NewSettingService(&oauthCaptchaSettingRepo{values: map[string]string{service.SettingKeyAffiliateEnabled: "true", service.SettingKeyAffiliateSettleIntervalMinutes: "30"}}, nil)
	teamRepo := &partnerTeamOverviewRepo{}
	team := service.NewAffiliateTeamService(teamRepo, settings)
	affiliate := service.NewAffiliateService(nil, settings, nil, nil)
	svc := service.NewPartnerService(&partnerTeamStateRepo{state: state}, affiliate, team)
	h := NewAffiliatePartnerHandler(svc, team)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: p}) })
	router.GET("/api/v1/user/aff/partner/team/overview", h.TeamOverview)
	return router, teamRepo, state
}

func TestPartnerTeamOverviewCustomerScope(t *testing.T) {
	for _, tc := range []struct {
		name, query, relation string
		user, owner           int64
		subAgent              bool
	}{
		{"attached agent customer", "20", "upline", 20, 10, false},
		{"upline customer", "21", "upline", 21, 11, false},
		{"direct secondary agent", "22", "owner", 22, 0, true},
		{"attached agent itself", "10", "owner", 10, 0, false},
		{"partner itself", "1", "owner", 1, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, repo, state := newPartnerTeamOverviewRouter(t)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/user/aff/partner/team/overview?start_date=2026-02-01&end_date=2026-02-02&user_id="+tc.query, nil))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var body struct {
				Code int
				Data service.PartnerTeamOverview
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Zero(t, body.Code)
			v := body.Data
			require.NotNil(t, v.Customer)
			require.Equal(t, tc.user, v.Customer.User.UserID)
			require.Equal(t, tc.relation, v.Customer.Relation)
			require.Equal(t, tc.subAgent, v.Customer.IsSubAgent)
			if bound := state.Snapshots[tc.user].BoundAt; bound != nil {
				require.NotNil(t, v.Customer.BoundAt)
				require.True(t, bound.Equal(*v.Customer.BoundAt))
			} else {
				require.Nil(t, v.Customer.BoundAt)
			}
			if email := state.User(tc.user).Email; email != "" {
				require.Contains(t, v.Customer.User.Email, "*")
				require.NotContains(t, w.Body.String(), email)
			}
			if tc.owner > 0 {
				require.NotNil(t, v.Customer.OwnerAgent)
				require.Equal(t, tc.owner, v.Customer.OwnerAgent.UserID)
				require.Contains(t, v.Customer.OwnerAgent.Email, "*")
				require.NotContains(t, w.Body.String(), state.User(tc.owner).Email)
			} else {
				require.Nil(t, v.Customer.OwnerAgent)
			}
			if tc.user == 20 {
				require.Equal(t, "区域客户", v.Customer.User.Nickname)
				require.Equal(t, "渠道经理", v.Customer.OwnerAgent.Nickname)
			}
			require.Equal(t, []service.AffiliateTeamScope{{ViewerID: 1, MemberIDs: []int64{tc.user}}}, repo.scopes)
			require.Equal(t, []int64{tc.user}, repo.filterIDs)
			require.Equal(t, service.PartnerTeamSummary{Cost: 5, SubCost: 5, ActiveCustomers: 1, Customers: 1, Requests: 2, Tokens: 30}, v.Summary)
			require.Equal(t, []service.PartnerTeamTrend{{Date: "2026-02-01"}, {Date: "2026-02-02", Cost: 5}}, v.Trend)
			require.Equal(t, []service.AffiliateTeamModelShare{{Model: "customer-model", Cost: 5, Share: 1}}, v.Models)
			require.Equal(t, &service.AffiliateTeamFilters{Models: []string{"customer-model"}, Groups: []service.AffiliateTeamGroup{{ID: 4, Name: "客户分组"}}}, v.Filters)
			for _, rng := range repo.ranges {
				require.Equal(t, "2026-02-01", rng.StartDate)
				require.Equal(t, "2026-02-02", rng.EndDate)
			}
			require.NotContains(t, w.Body.String(), "earnings")
		})
	}
}

func TestPartnerTeamOverviewCustomerForbiddenAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		id     string
		status int
	}{
		{"30", 403}, {"2", 403}, {"99", 403},
		{"bad", 400}, {"0", 400}, {"-1", 400}, {"9223372036854775808", 400},
	} {
		t.Run(tc.id, func(t *testing.T) {
			router, repo, _ := newPartnerTeamOverviewRouter(t)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/user/aff/partner/team/overview?user_id="+tc.id, nil))
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == http.StatusForbidden {
				var body struct {
					Code   int
					Reason string
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, 403, body.Code)
				require.Equal(t, "PARTNER_FORBIDDEN", body.Reason)
			}
			require.Empty(t, repo.scopes)
			require.Empty(t, repo.filterIDs)
		})
	}
}

func TestPartnerTeamOverviewWithoutCustomerKeepsRegion(t *testing.T) {
	router, repo, _ := newPartnerTeamOverviewRouter(t)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/user/aff/partner/team/overview", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct{ Data service.PartnerTeamOverview }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Nil(t, body.Data.Customer)
	require.Nil(t, body.Data.Filters)
	require.EqualValues(t, 6, body.Data.Summary.Customers)
	require.Len(t, repo.scopes, 1)
	require.ElementsMatch(t, []int64{1, 10, 11, 20, 21, 22}, repo.scopes[0].MemberIDs)
	require.Empty(t, repo.filterIDs)
	require.NotContains(t, w.Body.String(), "earnings")
}

func TestPartnerTeamOverviewDeletedCustomerAndOwner(t *testing.T) {
	router, _, state := newPartnerTeamOverviewRouter(t)
	for _, id := range []int64{10, 20} {
		snapshot := state.Snapshots[id]
		snapshot.User.Deleted = true
		state.Snapshots[id] = snapshot
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/user/aff/partner/team/overview?user_id=20", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct{ Data service.PartnerTeamOverview }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data.Customer)
	require.Equal(t, service.AffiliateTeamUser{UserID: 20, Deleted: true}, body.Data.Customer.User)
	require.Equal(t, &service.AffiliateTeamUser{UserID: 10, Deleted: true}, body.Data.Customer.OwnerAgent)
}

func TestPartnerTeamOverviewReadErrors(t *testing.T) {
	for _, filters := range []bool{true, false} {
		t.Run(map[bool]string{true: "filters", false: "aggregates"}[filters], func(t *testing.T) {
			router, repo, _ := newPartnerTeamOverviewRouter(t)
			err := errors.New("private database error")
			if filters {
				repo.filterErr = err
			} else {
				repo.aggregateErr = err
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/user/aff/partner/team/overview?user_id=20", nil))
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), err.Error())
			if filters {
				require.Empty(t, repo.scopes)
			}
		})
	}
}
