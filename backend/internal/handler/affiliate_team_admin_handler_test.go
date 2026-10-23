//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// adminTeamRepoStub 只实现本测试用到的方法；其余方法调用会 panic，用来发现意外的查询。
type adminTeamRepoStub struct {
	service.AffiliateTeamRepository
	agentLevels map[int64]int
	stateFor    []int64
	ownerQuery  service.AffiliateTeamListQuery
}

func (s *adminTeamRepoStub) GetViewerState(_ context.Context, userID int64) (service.AffiliateTeamViewerState, error) {
	s.stateFor = append(s.stateFor, userID)
	level, ok := s.agentLevels[userID]
	if !ok {
		level = 3
	}
	return service.AffiliateTeamViewerState{AgentLevel: level}, nil
}

func (s *adminTeamRepoStub) CountTeam(context.Context, int64) (service.AffiliateTeamCounts, error) {
	return service.AffiliateTeamCounts{DirectCustomers: 2}, nil
}

func (s *adminTeamRepoStub) GetAggregates(context.Context, service.AffiliateTeamScope, service.AffiliateTeamRange) (service.AffiliateTeamAggregates, error) {
	return service.AffiliateTeamAggregates{Summary: service.AffiliateTeamSummary{Cost: 5}}, nil
}

func (s *adminTeamRepoStub) GetViewerSubject(_ context.Context, userID int64) (service.AffiliateTeamViewerSubject, error) {
	return service.AffiliateTeamViewerSubject{User: service.AffiliateTeamUser{UserID: userID, Nickname: "张三", Email: "zhang@example.com"}}, nil
}

func (s *adminTeamRepoStub) ListTeamOwners(_ context.Context, _ service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamOwnerEntry, int64, error) {
	s.ownerQuery = query
	return []service.AffiliateTeamOwnerEntry{{User: service.AffiliateTeamUser{UserID: 42, Email: "zhang@example.com"}, AgentLevel: 1}}, 21, nil
}

func newAdminTeamTestRouter(repo *adminTeamRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	settings := service.NewSettingService(&oauthCaptchaSettingRepo{values: map[string]string{}}, nil)
	h := NewAffiliateTeamHandler(service.NewAffiliateTeamService(repo, settings))
	router := gin.New()
	team := router.Group("/api/v1/admin/affiliates/team")
	team.GET("/overview", h.AdminOverview)
	team.GET("/agents", h.AdminListOwners)
	team.GET("/:viewer_id/overview", h.AdminViewerOverview)
	team.GET("/:viewer_id/customers", h.AdminViewerCustomers)
	team.GET("/:viewer_id/customers/:user_id/requests", h.AdminViewerRequests)
	user := router.Group("/api/v1/user/aff/team", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
	})
	user.GET("/overview", h.Overview)
	return router
}

func serveAdminTeam(router *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]any) {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

func TestAdminAffiliateTeamHandlerValidatesViewerAndRange(t *testing.T) {
	router := newAdminTeamTestRouter(&adminTeamRepoStub{agentLevels: map[int64]int{42: 1}})
	for _, path := range []string{
		"/api/v1/admin/affiliates/team/abc/overview",
		"/api/v1/admin/affiliates/team/0/customers",
		"/api/v1/admin/affiliates/team/-3/customers/7/requests",
		"/api/v1/admin/affiliates/team/42/customers/0/requests",
		"/api/v1/admin/affiliates/team/42/overview?agent_id=x",
	} {
		recorder, _ := serveAdminTeam(router, path)
		require.Equal(t, http.StatusBadRequest, recorder.Code, path)
	}

	recorder, body := serveAdminTeam(router, "/api/v1/admin/affiliates/team/agents?start_date=2026-01-01&end_date=2026-12-31")
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "AFFILIATE_TEAM_INVALID_RANGE", body["reason"])

	// 没有团队（含不存在的 ID）为 404，与代理端的 403 区分。
	recorder, body = serveAdminTeam(router, "/api/v1/admin/affiliates/team/99/overview")
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, "AFFILIATE_TEAM_NOT_FOUND", body["reason"])
}

func TestAdminAffiliateTeamHandlerPassesQueryAndSubject(t *testing.T) {
	repo := &adminTeamRepoStub{agentLevels: map[int64]int{42: 1}}
	router := newAdminTeamTestRouter(repo)

	recorder, body := serveAdminTeam(router, "/api/v1/admin/affiliates/team/agents?level=2&search=zh&sort_by=earnings&sort_order=asc&page=2&page_size=10")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.AffiliateTeamListQuery{
		Level: "2", Search: "zh", SortBy: service.AffiliateTeamSortEarnings, SortDesc: false, Page: 2, PageSize: 10, AdminSearch: true,
	}, repo.ownerQuery)
	data := body["data"].(map[string]any)
	require.EqualValues(t, 21, data["total"])
	require.Equal(t, "zhang@example.com", data["items"].([]any)[0].(map[string]any)["user"].(map[string]any)["email"])

	// 管理端代理视角以路径中的代理为根，并附 subject；代理端同一接口不含 subject。
	recorder, body = serveAdminTeam(router, "/api/v1/admin/affiliates/team/42/overview")
	require.Equal(t, http.StatusOK, recorder.Code)
	subject := body["data"].(map[string]any)["subject"].(map[string]any)
	require.Equal(t, "zhang@example.com", subject["user"].(map[string]any)["email"])
	require.Equal(t, []int64{42}, repo.stateFor)

	recorder, body = serveAdminTeam(router, "/api/v1/user/aff/team/overview")
	require.Equal(t, http.StatusForbidden, recorder.Code, "代理端仍受功能开关约束")
	require.Equal(t, "AFFILIATE_TEAM_FORBIDDEN", body["reason"])
}
