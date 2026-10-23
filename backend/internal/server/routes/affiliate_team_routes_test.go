package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAffiliateTeamRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var matchedPath, userID string
	// 在 JWT 位置截断请求：只验证路由注册与参数名，不进入 handler 与限流。
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		matchedPath = c.FullPath()
		userID = c.Param("user_id")
		c.AbortWithStatus(http.StatusNoContent)
	})
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	handlers := &handler.Handlers{AffiliateTeam: handler.NewAffiliateTeamHandler(nil)}

	require.NotPanics(t, func() {
		RegisterUserRoutes(router.Group("/api/v1"), handlers, jwt, audit, nil, middleware.NewPanelRateLimiter(nil, nil))
	})
	for _, path := range []string{
		"/api/v1/user/aff/team/overview",
		"/api/v1/user/aff/team/customers",
		"/api/v1/user/aff/team/agents",
		"/api/v1/user/aff/team/customers/:user_id/requests",
		"/api/v1/user/aff/invitees/:user_id/agent-level",
		"/api/v1/user/aff/invitees",
	} {
		t.Run(path, func(t *testing.T) {
			matchedPath, userID = "", ""
			method := http.MethodGet
			if path == "/api/v1/user/aff/invitees/:user_id/agent-level" {
				method = http.MethodPost
			}
			request := httptest.NewRequest(method, replaceUserIDParam(path, "42"), nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, path, matchedPath)
			if matchedPath != replaceUserIDParam(path, "42") {
				require.Equal(t, "42", userID)
			}
		})
	}
}

func replaceUserIDParam(path, id string) string {
	return strings.Replace(path, ":user_id", id, 1)
}
