package admin

// CUSTOM(fork): Protocol validation occurs before balance mutation/idempotency execution.
import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminBalanceRejectsInvalidKind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"balance":10,"operation":"add","kind":"unknown"}`, `{"balance":10,"operation":"set","kind":"offline"}`, `{"balance":10,"operation":"subtract","kind":"offline"}`} {
		router := gin.New()
		h := &UserHandler{}
		router.POST("/users/:id/balance", h.UpdateBalance)
		request := httptest.NewRequest(http.MethodPost, "/users/1/balance", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		router.ServeHTTP(out, request)
		require.Equal(t, http.StatusBadRequest, out.Code)
		require.Contains(t, out.Body.String(), "OFFLINE_INVALID_KIND")
	}
}

func TestAdminBalanceHistoryReturnsCashAndGiftTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &UserHandler{adminService: newStubAdminService()}
	router.GET("/users/:id/balance-history", h.GetBalanceHistory)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/users/1/balance-history?type=subscription&page=2&page_size=1", nil))
	require.Equal(t, http.StatusOK, out.Code)
	var body struct {
		Data struct {
			TotalRecharged float64 `json:"total_recharged"`
			TotalGifted    float64 `json:"total_gifted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Body.Bytes(), &body))
	require.Equal(t, 100.0, body.Data.TotalRecharged)
	require.Equal(t, 25.0, body.Data.TotalGifted)
}
