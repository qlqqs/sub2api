package routes

// CUSTOM(fork): 合伙人路由与既有动态参数同时注册，用户接口均带重查询限流。
import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPartnerRoutesRegisterAndMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{Admin: &handler.AdminHandlers{Affiliate: adminhandler.NewAffiliateHandler(nil, nil)}, AffiliatePartner: handler.NewAffiliatePartnerHandler(nil, nil)}
	router := gin.New()
	var matched string
	router.Use(func(c *gin.Context) { matched = c.FullPath(); c.AbortWithStatus(204) })
	require.NotPanics(t, func() { registerAffiliateRoutes(router.Group("/api/v1/admin"), h) })
	noop := func(c *gin.Context) { c.Next() }
	require.NotPanics(t, func() {
		RegisterUserRoutes(router.Group("/api/v1"), h, middleware.JWTAuthMiddleware(noop), middleware.AuditLogMiddleware(noop), nil, nil)
	})
	tests := []string{
		"GET /admin/affiliates/partners", "GET /admin/affiliates/partners/preview", "POST /admin/affiliates/partners", "GET /admin/affiliates/partners/:partner_id", "PUT /admin/affiliates/partners/:partner_id", "GET /admin/affiliates/partners/:partner_id/agents/preview", "POST /admin/affiliates/partners/:partner_id/agents", "GET /admin/affiliates/partners/:partner_id/statements", "GET /admin/affiliates/partners/:partner_id/statements/:month", "POST /admin/affiliates/partners/:partner_id/statements/:month/confirm", "POST /admin/affiliates/partners/:partner_id/statements/:month/pay", "GET /admin/affiliates/partners/:partner_id/statements/:month/export", "POST /admin/affiliates/partners/:partner_id/adjustments", "DELETE /admin/affiliates/partners/:partner_id/adjustments/:id",
		"GET /user/aff/partner/overview", "GET /user/aff/partner/statements", "GET /user/aff/partner/statements/:month", "GET /user/aff/partner/team/overview", "GET /user/aff/partner/team/customers", "GET /user/aff/partner/team/lines", "GET /user/aff/partner/team/customers/:user_id/requests", "GET /user/aff/partner/team/agents/:agent_id/overview", "GET /user/aff/partner/team/agents/:agent_id/customers", "GET /user/aff/partner/team/agents/:agent_id/agents", "GET /user/aff/partner/team/agents/:agent_id/customers/:user_id/requests",
	}
	replace := strings.NewReplacer(":partner_id", "12", ":agent_id", "13", ":user_id", "14", ":month", "2026-02", ":id", "15")
	for _, tc := range tests {
		parts := strings.SplitN(tc, " ", 2)
		pattern := "/api/v1" + parts[1]
		w := httptest.NewRecorder()
		matched = ""
		router.ServeHTTP(w, httptest.NewRequest(parts[0], replace.Replace(pattern), nil))
		require.Equal(t, 204, w.Code, tc)
		require.Equal(t, pattern, matched, tc)
	}
}
