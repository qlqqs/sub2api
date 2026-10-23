package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// CUSTOM: 分组模型倍率与汇率接口挂在管理员路由组：静态 /model-rates 与 /:id 同级共存，
// 非管理员 403、未登录 401。
func TestGroupModelRateAndFxRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Group:          &adminhandler.GroupHandler{},
		GroupModelRate: &adminhandler.GroupModelRateHandler{},
		FxRate:         &adminhandler.FxRateHandler{},
	}}
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		servermiddleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, stepUp, nil, nil)

	registered := map[string]string{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}
	for key, handlerName := range map[string]string{
		"GET /api/v1/admin/groups/model-rates":        "GroupModelRateHandler).ListAll",
		"GET /api/v1/admin/groups/:id/model-rates":    "GroupModelRateHandler).Get",
		"PUT /api/v1/admin/groups/:id/model-rates":    "GroupModelRateHandler).Replace",
		"GET /api/v1/admin/groups/:id":                "GroupHandler).GetByID",
		"GET /api/v1/admin/settings/fx-rate":          "FxRateHandler).Get",
		"PUT /api/v1/admin/settings/fx-rate":          "FxRateHandler).Update",
		"POST /api/v1/admin/settings/fx-rate/refresh": "FxRateHandler).Refresh",
	} {
		require.Contains(t, registered, key)
		require.True(t, strings.Contains(registered[key], handlerName), "%s -> %s", key, registered[key])
	}

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/admin/groups/model-rates"},
		{http.MethodPut, "/api/v1/admin/groups/3/model-rates"},
		{http.MethodPost, "/api/v1/admin/settings/fx-rate/refresh"},
	} {
		for auth, want := range map[string]int{"": http.StatusUnauthorized, "Bearer user-token": http.StatusForbidden} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			router.ServeHTTP(rec, req)
			require.Equal(t, want, rec.Code, "%s %s auth=%q", tc.method, tc.path, auth)
		}
	}
}
