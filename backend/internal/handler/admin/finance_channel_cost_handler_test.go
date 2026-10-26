//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type financeCostHandlerRepo struct {
	input service.FinanceChannelCostUpdate
	calls int
	err   error
}

func (r *financeCostHandlerRepo) Get(context.Context, int64) (*service.FinanceChannelCostConfig, error) {
	return &service.FinanceChannelCostConfig{ChannelID: 7, Version: "v"}, r.err
}
func (r *financeCostHandlerRepo) CompareAndSwap(_ context.Context, id int64, input service.FinanceChannelCostUpdate) (*service.FinanceChannelCostConfig, error) {
	r.calls++
	r.input = input
	result := &service.FinanceChannelCostConfig{ChannelID: id, Version: "new"}
	if input.AccountStatsPricingRules != nil {
		result.AccountStatsPricingRules = *input.AccountStatsPricingRules
	}
	if input.ApplyPricingToAccountStats != nil {
		result.ApplyPricingToAccountStats = *input.ApplyPricingToAccountStats
	}
	return result, r.err
}
func financeCostRouter(repo *financeCostHandlerRepo, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if role != "" {
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
		c.Next()
	}, middleware.AdminOnly())
	handler := NewFinanceChannelCostHandler(service.NewFinanceChannelCostService(repo, nil))
	router.GET("/costs/:id", handler.Get)
	router.PUT("/costs/:id", handler.Update)
	return router
}
func TestFinanceChannelCostHTTPRoundTrip(t *testing.T) {
	repo := &financeCostHandlerRepo{}
	router := financeCostRouter(repo, service.RoleAdmin)
	body := `{"expected_version":"v","apply_pricing_to_account_stats":true,"account_stats_pricing_rules":[{"name":"cross-platform","group_ids":[3],"account_ids":[5],"pricing":[{"platform":"","models":["gpt-*"],"billing_mode":"image","input_price":0,"output_price":0.3,"cache_write_price":0.4,"cache_write_1h_price":0.5,"cache_read_price":0.6,"image_output_price":0.7,"per_request_price":0.8,"reasoning_effort_multipliers":{"high":2},"intervals":[{"min_tokens":0,"max_tokens":10,"tier_label":"2K","input_price":0,"output_price":0.2,"cache_write_price":0.3,"cache_write_1h_price":0.4,"cache_read_price":0.5,"per_request_price":0.6,"sort_order":9}]}]}]}`
	request := httptest.NewRequest(http.MethodPut, "/costs/7", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, repo.calls)
	var result struct {
		Data financeChannelCostResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, int64(7), result.Data.ChannelID)
	require.Equal(t, "new", result.Data.Version)
	p := result.Data.AccountStatsPricingRules[0].Pricing[0]
	require.Empty(t, p.Platform)
	require.Equal(t, 0.0, *p.InputPrice)
	require.Equal(t, 0.5, *p.CacheWrite1hPrice)
	require.Equal(t, 0.7, *p.ImageOutputPrice)
	require.Equal(t, 2.0, p.ReasoningEffortMultipliers["high"])
	require.Equal(t, 9, p.Intervals[0].SortOrder)
	require.Equal(t, 0.4, *p.Intervals[0].CacheWrite1hPrice)
}
func TestFinanceChannelCostHTTPErrors(t *testing.T) {
	for _, tc := range []struct {
		name, role, body, path string
		status                 int
		err                    error
	}{
		{name: "unauthenticated", status: 401},
		{name: "nonadmin", role: service.RoleUser, status: 403},
		{name: "invalid id", role: service.RoleAdmin, path: "0", body: `{"expected_version":"v","apply_pricing_to_account_stats":true}`, status: 400},
		{name: "missing version", role: service.RoleAdmin, body: `{"apply_pricing_to_account_stats":true}`, status: 400},
		{name: "stale", role: service.RoleAdmin, body: `{"expected_version":"old","apply_pricing_to_account_stats":true}`, status: 409, err: service.ErrFinanceCostConfigChanged},
		{name: "no patch", role: service.RoleAdmin, body: `{"expected_version":"v","model_mapping":{}}`, status: 400},
		{name: "unsupported fast not silently dropped", role: service.RoleAdmin, body: `{"expected_version":"v","account_stats_pricing_rules":[{"account_ids":[1],"pricing":[{"models":["gpt"],"fast_multiplier":2}]}]}`, status: 400},
		{name: "unsupported time", role: service.RoleAdmin, body: `{"expected_version":"v","account_stats_pricing_rules":[{"account_ids":[1],"pricing":[{"models":["gpt"],"time_pricing":{"periods":[{"start_time":"00:00","end_time":"01:00","multiplier":2}]}}]}]}`, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &financeCostHandlerRepo{err: tc.err}
			router := financeCostRouter(repo, tc.role)
			path := tc.path
			if path == "" {
				path = "7"
			}
			request := httptest.NewRequest(http.MethodPut, "/costs/"+path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, request)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.status == 409 {
				require.Contains(t, rec.Body.String(), "FINANCE_COST_CONFIG_CHANGED")
			} else {
				require.Zero(t, repo.calls)
			}
		})
	}
}
