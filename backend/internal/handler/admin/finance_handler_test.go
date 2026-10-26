//go:build unit

package admin

// CUSTOM(fork): 财务 HTTP 合约、CSV、错误脱敏与输入校验。
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type financeHandlerRepo struct {
	err  error
	data *service.FinanceData
}

func (r *financeHandlerRepo) Load(context.Context, service.FinanceRange) (*service.FinanceData, error) {
	if r.data != nil {
		return r.data, r.err
	}
	return &service.FinanceData{ComputedAt: time.Now()}, r.err
}
func (r *financeHandlerRepo) Settings(context.Context) (service.FinanceSettings, error) {
	return service.FinanceSettings{FixedCostAccountIDs: []int64{}, Accounts: []service.FinanceAccount{}}, r.err
}
func (r *financeHandlerRepo) SaveSettings(context.Context, []int64) error { return r.err }
func (r *financeHandlerRepo) ListExpenses(context.Context, service.FinanceRange, string, int, int) ([]service.FinanceExpense, int64, error) {
	return []service.FinanceExpense{}, 0, r.err
}
func (r *financeHandlerRepo) SaveExpense(context.Context, int64, int64, service.FinanceExpenseInput) (*service.FinanceExpense, error) {
	return nil, r.err
}
func (r *financeHandlerRepo) DeleteExpense(context.Context, int64) error {
	return service.ErrFinanceExpenseNotFound
}
func TestFinanceHandlerContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &financeHandlerRepo{}
	h := NewFinanceHandler(service.NewFinanceService(repo))
	r := gin.New()
	r.GET("/summary", h.Summary)
	r.GET("/export", h.Export)
	r.GET("/expenses", h.Expenses)
	r.GET("/breakdown", h.Breakdown)
	r.GET("/reconcile", h.Reconcile)
	r.PUT("/settings", h.SaveSettings)
	r.DELETE("/expenses/:id", h.DeleteExpense)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	w := do("GET", "/summary?start=2026-01-01&end=2026-01-02", "")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"revenue":"0"`)
	require.Contains(t, w.Body.String(), `"profit_margin":null`)
	require.Contains(t, w.Body.String(), `"warnings":[`)
	w = do("GET", "/export?start=2026-01-01&end=2026-01-02", "")
	require.Equal(t, 200, w.Code)
	require.True(t, strings.HasPrefix(w.Body.String(), "\xef\xbb\xbf期间,"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "finance-2026-01-01-2026-01-02.csv")
	require.Len(t, strings.Split(strings.TrimSpace(w.Body.String()), "\n"), 3)
	for _, path := range []string{"/summary?start=2000-01-01&end=2026-01-01", "/summary?granularity=year", "/summary?start=bad", "/expenses?page=-1", "/expenses?category=bad", "/breakdown?dim=sql", "/breakdown?limit=0", "/reconcile?only_mismatch=bad"} {
		require.Equal(t, 400, do("GET", path, "").Code, path)
	}
	require.Equal(t, 400, do("PUT", "/settings", `{}`).Code)
	require.Equal(t, 200, do("PUT", "/settings", `{"fixed_cost_account_ids":[]}`).Code)
	require.Equal(t, 404, do("DELETE", "/expenses/99", "").Code)
	repo.err = errors.New("secret postgres password")
	w = do(http.MethodGet, "/summary", "")
	require.Equal(t, 500, w.Code)
	require.NotContains(t, w.Body.String(), "postgres")
}

// CUSTOM(fork): 联调字段必须完整走到 HTTP JSON；ID 搜索也应保留数字邮箱匹配。
func TestFinanceHandlerRechargeTradeNumberAndReconcileSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	at := time.Date(2026, 10, 25, 12, 0, 0, 0, service.AffiliateTeamLocation)
	repo := &financeHandlerRepo{data: &service.FinanceData{
		Users:  []service.FinanceUser{{ID: 123, Email: "Alice@Example.COM"}, {ID: 456, Email: "client123@example.com"}, {ID: 1234, Email: "other@example.com"}},
		Orders: []service.FinanceOrder{{ID: 1, UserID: 123, OutTradeNo: "S2A-00123", OrderType: "balance", CompletedAt: at}, {ID: 2, UserID: 456, OrderType: "balance", CompletedAt: at}},
	}}
	h := NewFinanceHandler(service.NewFinanceService(repo))
	r := gin.New()
	r.GET("/recharge", h.Recharge)
	r.GET("/reconcile", h.Reconcile)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/recharge?start=2026-10-25&end=2026-10-25", nil))
	require.Equal(t, 200, w.Code)
	var recharge struct {
		Data struct {
			Orders []map[string]any `json:"orders"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &recharge))
	require.Len(t, recharge.Data.Orders, 2)
	tradeNumbers := map[float64]any{}
	for _, o := range recharge.Data.Orders {
		value, ok := o["out_trade_no"]
		require.True(t, ok)
		require.IsType(t, "", value)
		tradeNumbers[o["id"].(float64)] = value
	}
	require.Equal(t, "S2A-00123", tradeNumbers[1])
	require.Equal(t, "", tradeNumbers[2])
	for _, tc := range []struct {
		search string
		ids    []int64
	}{{"123", []int64{123, 456}}, {"000123", []int64{123}}, {"ALICE", []int64{123}}, {"1234", []int64{1234}}, {"%2B123", []int64{}}} {
		t.Run(tc.search, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/reconcile?search="+tc.search, nil))
			require.Equal(t, 200, w.Code)
			var result struct {
				Data struct {
					Items []struct {
						ID int64 `json:"user_id"`
					} `json:"items"`
					Total int `json:"total"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
			ids := []int64{}
			for _, item := range result.Data.Items {
				ids = append(ids, item.ID)
			}
			require.Equal(t, tc.ids, ids)
			require.Equal(t, len(tc.ids), result.Data.Total)
		})
	}
}

// CUSTOM(fork): 生图请求本身不等于按张成本；标签由配置决定，不能重算历史金额。
func TestFinanceBreakdownCostModeUsesConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, want                 string
		fixed, perImage, modelRate bool
		images                     int64
	}{
		{name: "只有生图用量仍按倍率", images: 100, want: "rate"},
		{name: "无图片用量但有按张配置", perImage: true, want: "per_image"},
		{name: "只有模型倍率", modelRate: true, images: 100, want: "model_rate"},
		{name: "按张优先于模型倍率", perImage: true, modelRate: true, want: "per_image"},
		{name: "固定优先于所有配置", fixed: true, perImage: true, modelRate: true, images: 100, want: "fixed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &service.FinanceData{Costs: []service.FinanceCost{{AccountID: 7, Requests: 1, Images: tc.images, HasPerImagePricing: tc.perImage, ModelRate: tc.modelRate, Cost: decimal.RequireFromString("0.12345678"), Consumption: decimal.RequireFromString("1")}}}
			if tc.fixed {
				data.FixedIDs = []int64{7}
			}
			h := NewFinanceHandler(service.NewFinanceService(&financeHandlerRepo{data: data}))
			r := gin.New()
			r.GET("/breakdown", h.Breakdown)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/breakdown?dim=account", nil))
			require.Equal(t, 200, w.Code)
			var result struct {
				Data struct {
					Rows []struct {
						Mode string `json:"cost_mode"`
						Cost string `json:"account_cost"`
					} `json:"rows"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
			require.Len(t, result.Data.Rows, 1)
			require.Equal(t, tc.want, result.Data.Rows[0].Mode)
			wantCost := "0.12345678"
			if tc.fixed {
				wantCost = "0"
			}
			require.Equal(t, wantCost, result.Data.Rows[0].Cost)
		})
	}
}
