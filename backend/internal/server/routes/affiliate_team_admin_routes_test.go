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

// 管理端团队消费接口挂在管理员路由组下：未登录 401、非管理员 403，
// 中间件链与其他邀请返利只读接口相同（不额外加重型限流）。
func TestAdminAffiliateTeamRoutesSitBehindAdminChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Admin:         &handler.AdminHandlers{Affiliate: adminhandler.NewAffiliateHandler(nil, nil)},
		AffiliateTeam: handler.NewAffiliateTeamHandler(nil),
	}
	chains := map[string][]string{}
	var matchedPath, viewerID, userID string
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		names := c.HandlerNames()
		chains[c.FullPath()] = names[:len(names)-1]
		matchedPath, viewerID, userID = c.FullPath(), c.Param("viewer_id"), c.Param("user_id")
		switch c.GetHeader("Authorization") {
		case "":
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
		case "Bearer admin":
			c.AbortWithStatus(http.StatusNoContent)
		default:
			servermiddleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
		}
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	require.NotPanics(t, func() {
		RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, stepUp, nil, nil)
	})

	serve := func(path, auth string) int {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if auth != "" {
			request.Header.Set("Authorization", auth)
		}
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}
	require.Equal(t, http.StatusNoContent, serve("/api/v1/admin/affiliates/invites", "Bearer admin"))
	reference := strings.Join(chains["/api/v1/admin/affiliates/invites"], "\n")
	require.NotEmpty(t, reference)

	const prefix = "/api/v1/admin/affiliates/team"
	for _, tc := range []struct{ path, route, viewer, user string }{
		{prefix + "/overview", prefix + "/overview", "", ""},
		{prefix + "/agents", prefix + "/agents", "", ""},
		{prefix + "/42/overview", prefix + "/:viewer_id/overview", "42", ""},
		{prefix + "/42/customers", prefix + "/:viewer_id/customers", "42", ""},
		{prefix + "/42/agents", prefix + "/:viewer_id/agents", "42", ""},
		{prefix + "/42/customers/7/requests", prefix + "/:viewer_id/customers/:user_id/requests", "42", "7"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			require.Equal(t, http.StatusUnauthorized, serve(tc.path, ""))
			require.Equal(t, http.StatusForbidden, serve(tc.path, "Bearer user"))
			matchedPath, viewerID, userID = "", "", ""
			require.Equal(t, http.StatusNoContent, serve(tc.path, "Bearer admin"))
			require.Equal(t, tc.route, matchedPath)
			require.Equal(t, tc.viewer, viewerID)
			require.Equal(t, tc.user, userID)
			require.Equal(t, reference, strings.Join(chains[tc.route], "\n"))
		})
	}
}
