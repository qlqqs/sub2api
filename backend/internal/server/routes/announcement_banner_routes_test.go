package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// CUSTOM: 公告轮播配置——管理端读写挂在管理员设置组，用户端只读挂在登录组。
func TestAnnouncementBannerRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminRouter := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{AnnouncementBanner: &adminhandler.AnnouncementBannerHandler{}}}
	adminAuth := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		middleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
	})
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(adminRouter.Group("/api/v1"), handlers, adminAuth, audit, stepUp, nil, nil)
	registered := map[string]string{}
	for _, route := range adminRouter.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}
	for _, key := range []string{"GET /api/v1/admin/settings/announcement-banner", "PUT /api/v1/admin/settings/announcement-banner"} {
		require.Contains(t, registered, key)
		require.True(t, strings.Contains(registered[key], "AnnouncementBannerHandler)"), "%s -> %s", key, registered[key])
		method, path, _ := strings.Cut(key, " ")
		rec := httptest.NewRecorder()
		adminRouter.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		require.Equal(t, http.StatusForbidden, rec.Code, key)
	}

	// 用户端：在 JWT 位置截断，只验证路由存在；只装配用户端 handler 时注册也不能 panic
	userRouter := gin.New()
	var matchedPath string
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		matchedPath = c.FullPath()
		c.AbortWithStatus(http.StatusNoContent)
	})
	require.NotPanics(t, func() {
		RegisterUserRoutes(userRouter.Group("/api/v1"), &handler.Handlers{}, jwt, audit, nil, middleware.NewPanelRateLimiter(nil, nil))
	})
	rec := httptest.NewRecorder()
	userRouter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/announcement-banner", nil))
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "/api/v1/announcement-banner", matchedPath)
	rec = httptest.NewRecorder()
	userRouter.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/announcement-banner", nil))
	require.NotEqual(t, http.StatusNoContent, rec.Code, "user side must be read-only")
}
