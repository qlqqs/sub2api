package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type agentLevelSet struct {
	ids   []int64
	level int
}

type agentLevelLock struct {
	ids    []int64
	locked bool
}

type agentLevelHandlerRepo struct {
	service.AffiliateRepository
	manual []agentLevelSet
	locked []agentLevelLock
}

func (r *agentLevelHandlerRepo) SetAgentLevelManual(_ context.Context, ids []int64, level int) error {
	r.manual = append(r.manual, agentLevelSet{ids: ids, level: level})
	return nil
}

func (r *agentLevelHandlerRepo) SetAgentLevelLocked(_ context.Context, ids []int64, locked bool) error {
	r.locked = append(r.locked, agentLevelLock{ids: ids, locked: locked})
	return nil
}

func newAgentLevelRouter(repo *agentLevelHandlerRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewAffiliateHandler(service.NewAffiliateService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.PUT("/api/v1/admin/affiliates/users/:user_id", handler.UpdateUserSettings)
	router.POST("/api/v1/admin/affiliates/users/batch-agent-level", handler.BatchSetAgentLevel)
	return router
}

func doAgentLevelJSON(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestAffiliateHandlerUpdateUserSettingsSetsAgentLevelAndUnlocks(t *testing.T) {
	repo := &agentLevelHandlerRepo{}
	router := newAgentLevelRouter(repo)

	resp := doAgentLevelJSON(router, http.MethodPut, "/api/v1/admin/affiliates/users/11", `{"agent_level":2,"agent_level_locked":false}`)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Equal(t, []agentLevelSet{{ids: []int64{11}, level: 2}}, repo.manual)
	require.Equal(t, []agentLevelLock{{ids: []int64{11}, locked: false}}, repo.locked)

	resp = doAgentLevelJSON(router, http.MethodPut, "/api/v1/admin/affiliates/users/11", `{"agent_level":7}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Contains(t, resp.Body.String(), "INVALID_AGENT_LEVEL")
}

func TestAffiliateHandlerBatchSetAgentLevel(t *testing.T) {
	repo := &agentLevelHandlerRepo{}
	router := newAgentLevelRouter(repo)

	resp := doAgentLevelJSON(router, http.MethodPost, "/api/v1/admin/affiliates/users/batch-agent-level", `{"user_ids":[1,2,0],"agent_level":1}`)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.JSONEq(t, `{"code":0,"message":"success","data":{"affected":3}}`, resp.Body.String())
	require.Equal(t, []agentLevelSet{{ids: []int64{1, 2}, level: 1}}, repo.manual)

	resp = doAgentLevelJSON(router, http.MethodPost, "/api/v1/admin/affiliates/users/batch-agent-level", `{"user_ids":[],"agent_level":1}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	resp = doAgentLevelJSON(router, http.MethodPost, "/api/v1/admin/affiliates/users/batch-agent-level", `{"user_ids":[1]}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}
