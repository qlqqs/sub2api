//go:build unit

package handler

// CUSTOM(fork): 合伙人金额输入、认证主体与 CSV 安全。
import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type partnerHandlerRepo struct {
	service.PartnerRepository
	calls int
	actor int64
	rate  decimal.Decimal
}

func (r *partnerHandlerRepo) Create(_ context.Context, _ int64, _ string, rate decimal.Decimal, actor int64) error {
	r.calls++
	r.actor = actor
	r.rate = rate
	return service.ErrPartnerExists
}
func TestPartnerHandlerDecimalValidationAndActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &partnerHandlerRepo{}
	h := NewAffiliatePartnerHandler(service.NewPartnerService(repo, nil, nil), nil)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7}) })
	router.POST("/partners", h.Create)
	for _, tc := range []struct {
		body  string
		code  int
		calls int
	}{
		{`{"user_id":1,"region_name":"区域"}`, 400, 0},
		{`{"user_id":1,"region_name":"区域","share_rate":null}`, 400, 0},
		{`{"user_id":1,"region_name":"区域","share_rate":"bad"}`, 400, 0},
		{`{"user_id":1,"region_name":"区域","share_rate":1.1}`, 400, 0},
		{`{"user_id":1,"region_name":"区域","share_rate":0.12345}`, 400, 0},
		{`{"user_id":1,"region_name":"区域","share_rate":0.4}`, 409, 1},
		{`{"user_id":1,"region_name":"区域","share_rate":"0.4000"}`, 409, 2},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/partners", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, tc.code, w.Code, w.Body.String())
		require.Equal(t, tc.calls, repo.calls)
	}
	require.EqualValues(t, 7, repo.actor)
	require.True(t, repo.rate.Equal(decimal.RequireFromString("0.4")))
}
func TestPartnerCSVHasBOMAndEscapesText(t *testing.T) {
	detail := service.PartnerStatementDetail{Totals: service.PartnerMonthTotals{Month: "2026-02", Profit: "-5"}, CostByGroup: []service.PartnerCostGroup{{GroupID: 1, GroupName: "=SUM(A1)", Consumption: "2", AccountCost: "1"}}, Adjustments: []service.PartnerAdjustment{{ID: 1, Amount: "-5.00", Note: "	=cmd()"}}}
	raw, err := partnerStatementCSV(detail)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(raw, []byte{239, 187, 191}))
	reader := csv.NewReader(bytes.NewReader(raw[3:]))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	require.NoError(t, err)
	require.Equal(t, "-5", rows[1][9])
	require.Contains(t, string(raw), "'=SUM(A1)")
	require.Contains(t, string(raw), "'	=cmd()")
	require.Contains(t, string(raw), "-5.00")
}
