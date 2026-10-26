//go:build unit

package admin

// CUSTOM(fork): 显式零倍率合法，缺失倍率不能默认为免费成本。
import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountCostHandlerRepo struct {
	rules []service.AccountModelCostRule
}

func (r *accountCostHandlerRepo) ListAll(context.Context) (map[int64][]service.AccountModelCostRule, error) {
	return map[int64][]service.AccountModelCostRule{1: r.rules}, nil
}
func (r *accountCostHandlerRepo) ListByAccount(context.Context, int64) ([]service.AccountModelCostRule, error) {
	return r.rules, nil
}
func (r *accountCostHandlerRepo) ReplaceForAccount(_ context.Context, _ int64, v []service.AccountModelCostRule) error {
	r.rules = v
	return nil
}
func (r *accountCostHandlerRepo) AccountExists(_ context.Context, id int64) (bool, error) {
	return id == 1, nil
}
func TestAccountModelCostHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAccountModelCostHandler(service.NewAccountModelCostService(&accountCostHandlerRepo{}))
	r := gin.New()
	r.PUT("/accounts/:id/model-cost-multipliers", h.Replace)
	r.GET("/accounts/:id/model-cost-multipliers", h.Get)
	r.GET("/accounts/model-cost-multipliers", h.ListAll)
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"rules":[{"model":"A","rate_multiplier":0}]}`, 200}, {`{"rules":[{"model":"a"}]}`, 400}, {`{"rules":[{"model":"a","rate_multiplier":-1}]}`, 400}, {`{"rules":[{"model":"a","rate_multiplier":1.23456}]}`, 400}, {`{"rules":[]}`, 200}, {`{}`, 400}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/accounts/1/model-cost-multipliers", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code)
		if tc.status == 400 {
			require.Contains(t, w.Body.String(), "ACCOUNT_MODEL_COST_INVALID")
		}
	}
	for _, path := range []string{"/accounts/model-cost-multipliers", "/accounts/1/model-cost-multipliers"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 200, w.Code)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/accounts/999/model-cost-multipliers", nil))
	require.Equal(t, 404, w.Code)
}
