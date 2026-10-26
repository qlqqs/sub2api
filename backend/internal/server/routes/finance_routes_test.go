package routes

// CUSTOM(fork): 所有财务路由必须处于管理员认证链，且不与账号动态路由冲突。
import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFinanceRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{Finance: &adminhandler.FinanceHandler{}, FinanceChannelCost: &adminhandler.FinanceChannelCostHandler{}, FinanceRecalculation: &adminhandler.FinanceRecalculationHandler{}, AccountModelCost: &adminhandler.AccountModelCostHandler{}}}
	auth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		code := http.StatusUnauthorized
		if c.GetHeader("Authorization") != "" {
			code = http.StatusForbidden
		}
		c.AbortWithStatus(code)
	})
	RegisterAdminRoutes(r.Group("/api/v1"), h, auth, servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	paths := map[string][]string{"GET": {"finance/channel-costs/1", "finance/recalculations", "finance/recalculations/1", "finance/recalculations/1/items", "finance/summary", "finance/recharge", "finance/breakdown", "finance/affiliate", "finance/reconcile", "finance/export", "finance/settings", "finance/expenses", "accounts/model-cost-multipliers", "accounts/1/model-cost-multipliers"}, "PUT": {"finance/channel-costs/1", "finance/settings", "finance/expenses/1", "accounts/1/model-cost-multipliers"}, "POST": {"finance/recalculations", "finance/recalculations/1/confirm", "finance/recalculations/1/retry-sync", "finance/expenses"}, "DELETE": {"finance/expenses/1"}}
	for method, list := range paths {
		for _, path := range list {
			for token, want := range map[string]int{"": 401, "user": 403} {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(method, "/api/v1/admin/"+path, nil)
				req.Header.Set("Authorization", token)
				r.ServeHTTP(w, req)
				require.Equal(t, want, w.Code, path)
			}
		}
	}
}
